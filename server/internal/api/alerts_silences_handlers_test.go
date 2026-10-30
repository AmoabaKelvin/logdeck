package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func listAlertSilences(t *testing.T, router http.Handler) []alertSilenceView {
	t.Helper()
	w := doAlertsRequest(t, router, "GET", "/api/v1/alerts/silences", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 listing silences, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Silences []alertSilenceView `json:"silences"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse list response: %v", err)
	}
	if resp.Silences == nil {
		t.Fatal("silences is null, want an array")
	}
	return resp.Silences
}

func TestAlertSilencesCreateListDelete(t *testing.T) {
	router, _ := newAlertsTestRouter(t, nil)
	ends := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)

	w := doAlertsRequest(t, router, "POST", "/api/v1/alerts/silences",
		`{"project":"shop","comment":"deploy","endsAt":"`+ends+`"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create = %d: %s", w.Code, w.Body.String())
	}
	w = doAlertsRequest(t, router, "POST", "/api/v1/alerts/silences",
		`{"host":"prod","schedule":{"days":["sun"],"start":"02:00","durationMinutes":120,"timezone":"Europe/Berlin"}}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create scheduled = %d: %s", w.Code, w.Body.String())
	}

	silences := listAlertSilences(t, router)
	if len(silences) != 2 {
		t.Fatalf("silences = %+v, want 2", silences)
	}
	muted := silences[0]
	if !muted.Active || muted.Until != ends || muted.Project != "shop" || muted.Comment != "deploy" {
		t.Fatalf("one-off silence = %+v, want active until %s", muted, ends)
	}
	if silences[1].Schedule == nil || silences[1].Schedule.Timezone != "Europe/Berlin" {
		t.Fatalf("scheduled silence = %+v", silences[1])
	}

	if w := doAlertsRequest(t, router, "DELETE", "/api/v1/alerts/silences/"+muted.ID, ""); w.Code != http.StatusOK {
		t.Fatalf("delete = %d: %s", w.Code, w.Body.String())
	}
	if w := doAlertsRequest(t, router, "DELETE", "/api/v1/alerts/silences/"+muted.ID, ""); w.Code != http.StatusNotFound {
		t.Fatalf("second delete = %d, want 404", w.Code)
	}
	if silences := listAlertSilences(t, router); len(silences) != 1 {
		t.Fatalf("silences after delete = %+v, want 1", silences)
	}
}

func TestAlertSilenceValidation(t *testing.T) {
	router, _ := newAlertsTestRouter(t, nil)
	past := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
	for name, body := range map[string]string{
		"ended":             `{"endsAt":"` + past + `"}`,
		"bad time":          `{"endsAt":"tomorrow"}`,
		"schedule and ends": `{"endsAt":"2099-01-01T00:00:00Z","schedule":{"start":"02:00","durationMinutes":60}}`,
		"bad day":           `{"schedule":{"days":["sunday"],"start":"02:00","durationMinutes":60}}`,
		"bad start":         `{"schedule":{"start":"2am","durationMinutes":60}}`,
		"too long":          `{"schedule":{"start":"02:00","durationMinutes":1441}}`,
		"bad zone":          `{"schedule":{"start":"02:00","durationMinutes":60,"timezone":"Mars/Olympus"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			if w := doAlertsRequest(t, router, "POST", "/api/v1/alerts/silences", body); w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d (%s), want 400", w.Code, w.Body.String())
			}
		})
	}
}
