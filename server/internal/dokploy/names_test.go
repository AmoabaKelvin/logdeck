package dokploy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/AmoabaKelvin/logdeck/internal/config"
)

func TestOwnsAppNameReadsTheInventoryOnce(t *testing.T) {
	var inventories atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/project.all":
			inventories.Add(1)
			_, _ = w.Write([]byte(`[{"environments":[{"applications":[{"applicationId":"app"}],"compose":[{"composeId":"stack"}],"redis":[{"redisId":"cache","name":"cache"}]}]}]`))
		case "/api/application.one":
			_, _ = w.Write([]byte(`{"applicationId":"app","appName":"web","serverId":null,"previewDeployments":[{"appName":"preview-web-abc123"}]}`))
		case "/api/compose.one":
			_, _ = w.Write([]byte(`{"composeId":"stack","appName":"stack","serverId":"elsewhere"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	client := NewClient(config.DokployHostConfig{APIURL: upstream.URL})
	for name, want := range map[string]bool{"web": true, "preview-web-abc123": true, "stack": false, "cache": false, "": false} {
		if got := client.OwnsAppName(context.Background(), name); got != want {
			t.Fatalf("OwnsAppName(%q) = %v, want %v", name, got, want)
		}
	}
	if n := inventories.Load(); n != 1 {
		t.Fatalf("inventory read %d times, want once", n)
	}
	inventory, err := client.Inventory(context.Background())
	if err != nil || !inventory.Databases["cache"] || len(inventory.Resources) != 1 {
		t.Fatalf("inventory=%#v err=%v", inventory, err)
	}
}

func TestOwnsAppNameDoesNotWaitOnAnUnreachableDokploy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer upstream.Close()
	client := NewClient(config.DokployHostConfig{APIURL: upstream.URL})
	if client.OwnsAppName(context.Background(), "web") {
		t.Fatal("an unreadable inventory owns nothing")
	}
	client.names.mu.Lock()
	refreshing, fetched := client.names.refreshing, client.names.fetched
	client.names.mu.Unlock()
	if refreshing != nil || fetched.IsZero() {
		t.Fatal("a failed refresh must wait out the TTL before retrying")
	}
}
