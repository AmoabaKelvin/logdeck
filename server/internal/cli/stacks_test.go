package cli

import (
	"reflect"
	"testing"
)

func TestGroupStacks(t *testing.T) {
	containers := []containerInfo{
		{ID: "1", Host: "prod", State: "running", Labels: map[string]string{"com.docker.compose.project": "web"}},
		{ID: "2", Host: "prod", State: "exited", Labels: map[string]string{"com.docker.compose.project": "web"}},
		{ID: "3", Host: "staging", State: "running", Labels: map[string]string{"io.podman.compose.project": "web"}},
		{ID: "4", Host: "prod", State: "running", Labels: map[string]string{"com.docker.compose.project": "api"}},
		{ID: "5", Host: "prod", State: "running"}, // no project label: excluded
	}

	stacks := groupStacks(containers)
	if len(stacks) != 2 {
		t.Fatalf("expected 2 stacks, got %d: %+v", len(stacks), stacks)
	}

	// Sorted by project name.
	api, web := stacks[0], stacks[1]
	if api.Project != "api" || api.Containers != 1 || api.Running != 1 {
		t.Errorf("unexpected api stack: %+v", api)
	}
	if web.Project != "web" || web.Containers != 3 || web.Running != 2 {
		t.Errorf("unexpected web stack: %+v", web)
	}
	if !reflect.DeepEqual(web.Hosts, []string{"prod", "staging"}) {
		t.Errorf("unexpected web hosts: %v", web.Hosts)
	}
}

func TestComposeProject(t *testing.T) {
	if got := composeProject(map[string]string{"com.docker.compose.project": "a"}); got != "a" {
		t.Errorf("docker label: got %q", got)
	}
	if got := composeProject(map[string]string{"io.podman.compose.project": "b"}); got != "b" {
		t.Errorf("podman label: got %q", got)
	}
	if got := composeProject(nil); got != "" {
		t.Errorf("nil labels: got %q", got)
	}
}

func TestBuildTargets(t *testing.T) {
	targets := buildTargets([]containerInfo{
		{ID: "abc", Names: []string{"/web"}, Host: "prod"},
	})
	if len(targets) != 1 || targets[0] != "prod~abc~web" {
		t.Errorf("unexpected targets: %v", targets)
	}
}
