package docker

import (
	"context"
	"errors"
	"testing"
)

func TestInComposeProject(t *testing.T) {
	tests := []struct {
		name    string
		labels  map[string]string
		project string
		want    bool
	}{
		{
			name:    "docker compose label",
			labels:  map[string]string{"com.docker.compose.project": "web"},
			project: "web",
			want:    true,
		},
		{
			name:    "podman compose label",
			labels:  map[string]string{"io.podman.compose.project": "web"},
			project: "web",
			want:    true,
		},
		{
			name:    "different project",
			labels:  map[string]string{"com.docker.compose.project": "other"},
			project: "web",
			want:    false,
		},
		{
			name:    "no labels",
			labels:  nil,
			project: "web",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inComposeProject(tt.labels, tt.project); got != tt.want {
				t.Errorf("inComposeProject(%v, %q) = %v, want %v", tt.labels, tt.project, got, tt.want)
			}
		})
	}
}

func TestApplyToTargets(t *testing.T) {
	targets := []composeTarget{
		{ID: "1", Name: "web-1"},
		{ID: "2", Name: "web-2"},
		{ID: "3", Name: "web-3"},
	}

	succeeded, failed := applyToTargets(context.Background(), targets, func(_ context.Context, id string) error {
		if id == "2" {
			return errors.New("boom")
		}
		return nil
	})

	if succeeded != 2 {
		t.Errorf("succeeded = %d, want 2", succeeded)
	}
	if len(failed) != 1 {
		t.Fatalf("len(failed) = %d, want 1", len(failed))
	}
	if failed[0].ID != "2" || failed[0].Name != "web-2" || failed[0].Error != "boom" {
		t.Errorf("unexpected failure: %+v", failed[0])
	}
}

func TestApplyToTargetsEmpty(t *testing.T) {
	succeeded, failed := applyToTargets(context.Background(), nil, func(_ context.Context, _ string) error {
		return nil
	})

	if succeeded != 0 {
		t.Errorf("succeeded = %d, want 0", succeeded)
	}
	if failed == nil || len(failed) != 0 {
		t.Errorf("failed = %v, want empty non-nil slice", failed)
	}
}
