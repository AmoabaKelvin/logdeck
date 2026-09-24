package docker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

func podMember(id string, labels map[string]string) container.Summary {
	ctr := container.Summary{ID: id, Labels: labels}
	ctr.HostConfig.Annotations = map[string]string{podSandboxAnnotation: "infra"}
	return ctr
}

func TestLabelPodStacks(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/v4.0.0/libpod/containers/json" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`[{"Id":"api","PodName":"shop"},{"Id":"worker","PodName":"shop"},{"Id":"db","PodName":""}]`))
	}))
	defer srv.Close()

	cl, err := client.NewClientWithOpts(client.WithHost("tcp://" + strings.TrimPrefix(srv.URL, "http://")))
	if err != nil {
		t.Fatal(err)
	}

	containers := []container.Summary{
		// Podman before 6.0 lists no HostConfig; the unit label still triggers the lookup.
		{ID: "api", Labels: map[string]string{"PODMAN_SYSTEMD_UNIT": "api.service"}},
		podMember("worker", map[string]string{StackLabel: "custom"}),
		{ID: "db", Labels: map[string]string{"PODMAN_SYSTEMD_UNIT": "db.service"}},
	}
	labelPodStacks(context.Background(), cl, containers)

	if got := containers[0].Labels[StackLabel]; got != "shop" {
		t.Errorf("pod member stack = %q, want shop", got)
	}
	if got := containers[1].Labels[StackLabel]; got != "custom" {
		t.Errorf("explicit stack label overwritten: %q", got)
	}
	if _, ok := containers[2].Labels[StackLabel]; ok {
		t.Errorf("container outside a pod got a stack label")
	}

	labelPodStacks(context.Background(), cl, []container.Summary{{ID: "web"}})
	if requests != 1 {
		t.Errorf("libpod requests = %d, want 1 (hosts without pods skip the call)", requests)
	}
}
