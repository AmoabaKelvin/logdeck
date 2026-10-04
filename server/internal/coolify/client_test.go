package coolify

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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
			name:   "unknown type remains unsupported",
			labels: map[string]string{LabelManaged: "true", "coolify.type": "mystery", "com.docker.compose.project": "uuid-x"},
			want:   &ResourceInfo{Type: ResourceType("mystery"), UUID: "uuid-x"},
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
