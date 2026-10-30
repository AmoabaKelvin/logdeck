package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAlertSilenceAddPayloads(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var bodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		bodies = append(bodies, body)
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":"s1"}`)
	}))
	defer server.Close()

	before := time.Now()
	for _, args := range [][]string{
		{"--project", "shop", "--for", "2h", "--comment", "deploy"},
		{"--host", "prod", "--every", "sat,sun", "--at", "02:00", "--duration", "90m", "--tz", "Europe/Berlin"},
	} {
		captureStdout(t, func() {
			if code := execute(context.Background(), "test", append([]string{"alerts", "silences", "add", "--url", server.URL}, args...)); code != 0 {
				t.Fatalf("%v: exit code = %d, want 0", args, code)
			}
		})
	}

	oneOff := bodies[0]
	ends, err := time.Parse(time.RFC3339, fmt.Sprint(oneOff["endsAt"]))
	if err != nil || oneOff["project"] != "shop" || oneOff["comment"] != "deploy" ||
		ends.Before(before.Add(2*time.Hour-time.Second)) || ends.After(time.Now().Add(2*time.Hour)) {
		t.Errorf("one-off body = %v", oneOff)
	}
	schedule, _ := bodies[1]["schedule"].(map[string]any)
	if fmt.Sprint(schedule["days"]) != "[sat sun]" || schedule["start"] != "02:00" ||
		schedule["durationMinutes"] != float64(90) || schedule["timezone"] != "Europe/Berlin" || bodies[1]["endsAt"] != nil {
		t.Errorf("scheduled body = %v", bodies[1])
	}
}

func TestAlertSilenceAddUsageErrorsBeforeHTTP(t *testing.T) {
	for name, args := range map[string][]string{
		"for and until":       {"--for", "1h", "--until", "2099-01-01T00:00:00Z"},
		"bad for":             {"--for", "soon"},
		"bad until":           {"--until", "tomorrow"},
		"window without at":   {"--every", "sun", "--duration", "1h"},
		"window and for":      {"--every", "sun", "--at", "02:00", "--duration", "1h", "--for", "1h"},
		"tz without a window": {"--for", "1h", "--tz", "UTC"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			requested := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requested = true }))
			defer server.Close()

			var code int
			captureStderr(t, func() {
				code = execute(context.Background(), "test", append([]string{"alerts", "silences", "add", "--url", server.URL}, args...))
			})
			if code != 2 || requested {
				t.Errorf("exit code = %d, requested = %v; want a usage error before any request", code, requested)
			}
		})
	}
}
