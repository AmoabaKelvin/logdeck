package dokploy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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

func TestOnlyTheFirstRefreshIsWaitedFor(t *testing.T) {
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer upstream.Close()
	defer close(release)
	client := NewClient(config.DokployHostConfig{APIURL: upstream.URL})
	// A refresh after a failed one hangs, but the listing must not wait on it.
	client.names.fetched = time.Now().Add(-2 * appNamesTTL)
	returned := make(chan bool)
	go func() { returned <- client.OwnsAppName(context.Background(), "web") }()
	select {
	case owned := <-returned:
		if owned {
			t.Fatal("an unread inventory owns nothing")
		}
	case <-time.After(time.Second):
		t.Fatal("a later refresh was waited for")
	}
}

func TestInventoryReadsRecordsInParallel(t *testing.T) {
	var inFlight, peak atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/project.all" {
			apps := make([]string, 3*recordReads)
			for i := range apps {
				apps[i] = fmt.Sprintf(`{"applicationId":"app-%d"}`, i)
			}
			_, _ = w.Write([]byte(`[{"environments":[{"applications":[` + strings.Join(apps, ",") + `]}]}]`))
			return
		}
		n := inFlight.Add(1)
		defer inFlight.Add(-1)
		for p := peak.Load(); n > p && !peak.CompareAndSwap(p, n); p = peak.Load() {
		}
		time.Sleep(20 * time.Millisecond)
		id := r.URL.Query().Get("applicationId")
		_, _ = fmt.Fprintf(w, `{"applicationId":%q,"appName":%q,"serverId":null}`, id, id)
	}))
	defer upstream.Close()
	inventory, err := NewClient(config.DokployHostConfig{APIURL: upstream.URL}).Inventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if p := peak.Load(); p < 2 || p > recordReads {
		t.Fatalf("peak concurrent reads = %d, want 2..%d", p, recordReads)
	}
	for i, r := range inventory.Resources {
		if want := fmt.Sprintf("app-%d", i); r.AppName != want {
			t.Fatalf("resource %d = %q, want %q: order must follow project.all", i, r.AppName, want)
		}
	}
}
