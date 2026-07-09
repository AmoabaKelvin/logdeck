package api

import (
	"net/http/httptest"
	"testing"
)

func TestCheckWebSocketOrigin(t *testing.T) {
	tests := []struct {
		name   string
		host   string
		origin string
		want   bool
	}{
		{"no origin (non-browser client)", "localhost:8080", "", true},
		{"matching origin", "localhost:8080", "http://localhost:8080", true},
		{"matching origin case-insensitive", "localhost:8080", "http://LOCALHOST:8080", true},
		{"different host", "localhost:8080", "http://evil.example.com", false},
		{"different port", "localhost:8080", "http://localhost:9999", false},
		{"origin without host", "localhost:8080", "null", false},
		{"unparseable origin", "localhost:8080", "http://[::1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/api/v1/containers/abc/exec", nil)
			r.Host = tt.host
			if tt.origin != "" {
				r.Header.Set("Origin", tt.origin)
			}
			if got := checkWebSocketOrigin(r); got != tt.want {
				t.Errorf("checkWebSocketOrigin(host=%q, origin=%q) = %v, want %v", tt.host, tt.origin, got, tt.want)
			}
		})
	}
}
