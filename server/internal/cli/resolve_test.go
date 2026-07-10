package cli

import (
	"strings"
	"testing"
)

func testContainers() []containerInfo {
	return []containerInfo{
		{ID: "aaa111bbb222ccc", Names: []string{"/web"}, Host: "prod", State: "running"},
		{ID: "ddd333eee444fff", Names: []string{"/web"}, Host: "staging", State: "running"},
		{ID: "abc999888777666", Names: []string{"/db"}, Host: "prod", State: "running"},
	}
}

func TestResolveContainerExactName(t *testing.T) {
	c, err := resolveContainer(testContainers(), "db", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ID != "abc999888777666" || c.Host != "prod" {
		t.Errorf("resolved wrong container: %+v", c)
	}
}

func TestResolveContainerIDPrefix(t *testing.T) {
	c, err := resolveContainer(testContainers(), "ddd333", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Host != "staging" {
		t.Errorf("resolved wrong container: %+v", c)
	}
}

func TestResolveContainerNameBeatsIDPrefix(t *testing.T) {
	// "web" only matches names, but a name match must win over checking IDs.
	containers := []containerInfo{
		{ID: "web123", Names: []string{"/other"}, Host: "prod"},
		{ID: "zzz", Names: []string{"/web"}, Host: "prod"},
	}
	c, err := resolveContainer(containers, "web", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ID != "zzz" {
		t.Errorf("expected exact name match to win, got %+v", c)
	}
}

func TestResolveContainerAmbiguous(t *testing.T) {
	_, err := resolveContainer(testContainers(), "web", "")
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	message := err.Error()
	if !strings.Contains(message, "prod") || !strings.Contains(message, "staging") {
		t.Errorf("ambiguity error should list candidate hosts, got: %s", message)
	}
	if !strings.Contains(message, "--host") {
		t.Errorf("ambiguity error should suggest --host, got: %s", message)
	}
}

func TestResolveContainerHostDisambiguates(t *testing.T) {
	c, err := resolveContainer(testContainers(), "web", "staging")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Host != "staging" {
		t.Errorf("resolved wrong container: %+v", c)
	}
}

func TestResolveContainerNotFound(t *testing.T) {
	if _, err := resolveContainer(testContainers(), "nope", ""); err == nil {
		t.Error("expected not-found error")
	}
	if _, err := resolveContainer(testContainers(), "db", "staging"); err == nil {
		t.Error("expected not-found error when host filter excludes the match")
	}
}

func TestContainerName(t *testing.T) {
	if got := containerName(containerInfo{Names: []string{"/web"}}); got != "web" {
		t.Errorf("containerName = %q, want web", got)
	}
	if got := containerName(containerInfo{ID: "abcdef123456789"}); got != "abcdef123456" {
		t.Errorf("containerName fallback = %q, want short id", got)
	}
}
