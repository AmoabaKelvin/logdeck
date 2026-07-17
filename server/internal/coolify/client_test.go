package coolify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/AmoabaKelvin/logdeck/internal/config"
)

func TestIsCoolifyDefaultEnvVar(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"COOLIFY_URL", true},
		{"COOLIFY_", true},
		{"SOURCE_COMMIT", true},
		{"DATABASE_URL", false},
		{"MY_COOLIFY", false},
		{"source_commit", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsCoolifyDefaultEnvVar(tt.key); got != tt.want {
			t.Errorf("IsCoolifyDefaultEnvVar(%q) = %v, want %v", tt.key, got, tt.want)
		}
	}
}

func TestExtractResourceInfo(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   *ResourceInfo
	}{
		{
			name:   "not managed",
			labels: map[string]string{"com.docker.compose.project": "uuid-1"},
			want:   nil,
		},
		{
			name:   "managed but no uuid",
			labels: map[string]string{LabelManaged: "true"},
			want:   nil,
		},
		{
			name:   "application",
			labels: map[string]string{LabelManaged: "true", "coolify.type": "application", "com.docker.compose.project": "uuid-app"},
			want:   &ResourceInfo{Type: ResourceTypeApplication, UUID: "uuid-app"},
		},
		{
			name:   "service",
			labels: map[string]string{LabelManaged: "true", "coolify.type": "service", "com.docker.compose.project": "uuid-svc"},
			want:   &ResourceInfo{Type: ResourceTypeService, UUID: "uuid-svc"},
		},
		{
			name:   "database",
			labels: map[string]string{LabelManaged: "true", "coolify.type": "database", "com.docker.compose.project": "uuid-db"},
			want:   &ResourceInfo{Type: ResourceTypeDatabase, UUID: "uuid-db"},
		},
		{
			name:   "unknown type defaults to application",
			labels: map[string]string{LabelManaged: "true", "coolify.type": "mystery", "com.docker.compose.project": "uuid-x"},
			want:   &ResourceInfo{Type: ResourceTypeApplication, UUID: "uuid-x"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractResourceInfo(tt.labels)
			switch {
			case tt.want == nil && got != nil:
				t.Errorf("got %+v, want nil", got)
			case tt.want != nil && got == nil:
				t.Errorf("got nil, want %+v", tt.want)
			case tt.want != nil && (*got != *tt.want):
				t.Errorf("got %+v, want %+v", *got, *tt.want)
			}
		})
	}
}

func TestNewMultiClientEmptyReturnsNil(t *testing.T) {
	if mc := NewMultiClient(nil); mc != nil {
		t.Errorf("NewMultiClient(nil) = %v, want nil", mc)
	}
	if mc := NewMultiClient([]config.CoolifyHostConfig{}); mc != nil {
		t.Errorf("NewMultiClient(empty) = %v, want nil", mc)
	}
}

func TestMultiClientGetClient(t *testing.T) {
	mc := NewMultiClient([]config.CoolifyHostConfig{
		{HostName: "hostA", APIURL: "https://a.example", APIToken: "tok-a"},
	})
	if mc == nil {
		t.Fatal("NewMultiClient returned nil for non-empty config")
	}
	if c := mc.GetClient("hostA"); c == nil {
		t.Error("GetClient(hostA) = nil, want client")
	}
	if c := mc.GetClient("unknown"); c != nil {
		t.Error("GetClient(unknown) = client, want nil")
	}

	// nil receiver must not panic.
	var nilMC *MultiClient
	if c := nilMC.GetClient("hostA"); c != nil {
		t.Error("nil MultiClient GetClient = client, want nil")
	}
}

func TestDoRequestSetsAuthHeader(t *testing.T) {
	var gotAuth, gotContentType, gotMethod string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		gotMethod = r.Method
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := newClient(srv.URL, "secret-token")
	resp, err := c.doRequest(context.Background(), http.MethodPatch, srv.URL+"/x", []byte(`{"a":1}`))
	if err != nil {
		t.Fatalf("doRequest error: %v", err)
	}
	if gotAuth != "Bearer secret-token" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer secret-token")
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotMethod != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", gotMethod)
	}
	if string(gotBody) != `{"a":1}` {
		t.Errorf("body = %q, want %q", gotBody, `{"a":1}`)
	}
	if string(resp) != `{"ok":true}` {
		t.Errorf("resp = %q", resp)
	}
}

func TestDoRequestNoBodyOmitsContentType(t *testing.T) {
	var gotContentType = "sentinel"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newClient(srv.URL, "tok")
	if _, err := c.doRequest(context.Background(), http.MethodGet, srv.URL, nil); err != nil {
		t.Fatalf("doRequest error: %v", err)
	}
	if gotContentType != "" {
		t.Errorf("Content-Type = %q, want empty for bodyless request", gotContentType)
	}
}

func TestDoRequestNon2xxIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newClient(srv.URL, "tok")
	if _, err := c.doRequest(context.Background(), http.MethodGet, srv.URL, nil); err == nil {
		t.Error("expected error for 500 response, got nil")
	}
}

func TestConnectionCallsVersionEndpoint(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newClient(srv.URL, "tok")
	if err := c.TestConnection(context.Background()); err != nil {
		t.Fatalf("TestConnection error: %v", err)
	}
	if gotPath != "/api/v1/version" {
		t.Errorf("path = %q, want /api/v1/version", gotPath)
	}
}

func TestSyncEnvVarsDatabaseUnsupported(t *testing.T) {
	c := newClient("http://unused.invalid", "tok")
	resource := &ResourceInfo{Type: ResourceTypeDatabase, UUID: "uuid-db"}
	err := c.SyncEnvVars(context.Background(), resource, map[string]string{"A": "1"})
	if err == nil {
		t.Fatal("expected error syncing database resource, got nil")
	}
}

// TestSyncEnvVarsDeletionAndUpsert exercises the full sync flow against a fake
// Coolify API: it must list existing vars, delete removed non-default vars,
// preserve Coolify-injected defaults, and bulk-upsert the current set (also
// excluding defaults). It also verifies the pluralized URL construction.
func TestSyncEnvVarsDeletionAndUpsert(t *testing.T) {
	var deleted []string
	var bulkKeys []string
	var listPath, bulkPath string

	mux := http.NewServeMux()
	// Existing env vars in Coolify: KEEP (still present), REMOVED (should be
	// deleted), COOLIFY_URL (default, must NOT be deleted).
	mux.HandleFunc("/api/v1/applications/uuid-1/envs", func(w http.ResponseWriter, r *http.Request) {
		listPath = r.URL.Path
		existing := []coolifyEnvVar{
			{UUID: "u-keep", Key: "KEEP"},
			{UUID: "u-removed", Key: "REMOVED"},
			{UUID: "u-default", Key: "COOLIFY_URL"},
		}
		_ = json.NewEncoder(w).Encode(existing)
	})
	// Bulk upsert endpoint.
	mux.HandleFunc("/api/v1/applications/uuid-1/envs/bulk", func(w http.ResponseWriter, r *http.Request) {
		bulkPath = r.URL.Path
		var payload bulkEnvPayload
		_ = json.NewDecoder(r.Body).Decode(&payload)
		for _, e := range payload.Data {
			bulkKeys = append(bulkKeys, e.Key)
		}
		w.WriteHeader(http.StatusOK)
	})
	// Per-var delete endpoint: capture the UUID from the path.
	mux.HandleFunc("/api/v1/applications/uuid-1/envs/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			// path: /api/v1/applications/uuid-1/envs/<uuid>
			deleted = append(deleted, r.URL.Path[len("/api/v1/applications/uuid-1/envs/"):])
		}
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newClient(srv.URL, "tok")
	resource := &ResourceInfo{Type: ResourceTypeApplication, UUID: "uuid-1"}
	newVars := map[string]string{
		"KEEP":         "v1",
		"NEW":          "v2",
		"COOLIFY_BILT": "should-be-excluded", // default prefix, excluded from upsert
	}
	if err := c.SyncEnvVars(context.Background(), resource, newVars); err != nil {
		t.Fatalf("SyncEnvVars error: %v", err)
	}

	// URL pluralization: "application" -> "applications".
	if listPath != "/api/v1/applications/uuid-1/envs" {
		t.Errorf("list path = %q", listPath)
	}
	if bulkPath != "/api/v1/applications/uuid-1/envs/bulk" {
		t.Errorf("bulk path = %q", bulkPath)
	}

	// Only REMOVED should be deleted. COOLIFY_URL (default) must be preserved,
	// KEEP is still present.
	if len(deleted) != 1 || deleted[0] != "u-removed" {
		t.Errorf("deleted = %v, want [u-removed]", deleted)
	}

	// Bulk upsert must contain KEEP and NEW, but exclude the COOLIFY_ default.
	sort.Strings(bulkKeys)
	want := []string{"KEEP", "NEW"}
	if len(bulkKeys) != len(want) {
		t.Fatalf("bulk keys = %v, want %v", bulkKeys, want)
	}
	for i := range want {
		if bulkKeys[i] != want[i] {
			t.Errorf("bulk keys = %v, want %v", bulkKeys, want)
			break
		}
	}
}

func TestListEnvVarsParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"uuid":"u1","key":"A"},{"uuid":"u2","key":"B"}]`))
	}))
	defer srv.Close()

	c := newClient(srv.URL, "tok")
	got, err := c.listEnvVars(context.Background(), &ResourceInfo{Type: ResourceTypeService, UUID: "uuid-s"})
	if err != nil {
		t.Fatalf("listEnvVars error: %v", err)
	}
	if len(got) != 2 || got[0].Key != "A" || got[1].UUID != "u2" {
		t.Errorf("listEnvVars = %+v", got)
	}
}
