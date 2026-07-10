package docker

import "testing"

func TestHealthFromStatus(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   string
	}{
		{"healthy", "Up 3 hours (healthy)", "healthy"},
		{"unhealthy", "Up 2 minutes (unhealthy)", "unhealthy"},
		{"starting", "Up 5 seconds (health: starting)", "starting"},
		{"no healthcheck running", "Up 3 hours", ""},
		{"no healthcheck exited", "Exited (0) 2 hours ago", ""},
		{"empty status", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := healthFromStatus(tt.status); got != tt.want {
				t.Errorf("healthFromStatus(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}
