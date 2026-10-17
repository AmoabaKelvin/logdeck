package dokploy

import (
	"context"
	"sync"
	"time"
)

// appNamesTTL bounds how long a deployment Dokploy adds or removes goes
// unnoticed by the container list.
const appNamesTTL = 5 * time.Minute

// appNames caches the deployment names Dokploy uses on the configured server.
// Dokploy releases up to v0.30.8 leave nothing of their own on a Swarm task,
// so its service or stack name is the only sign Dokploy deployed it, and
// listing containers must not wait on a full inventory every time.
type appNames struct {
	mu      sync.Mutex
	names   map[string]bool
	fetched time.Time
	// refreshing is closed when the running refresh ends.
	refreshing chan struct{}
}

func (a *appNames) store(resources []Resource) {
	names := make(map[string]bool)
	for _, r := range resources {
		names[r.AppName] = true
		for _, preview := range r.PreviewNames {
			names[preview] = true
		}
	}
	delete(names, "")
	a.mu.Lock()
	defer a.mu.Unlock()
	a.names = names
	a.fetched = time.Now()
}

// OwnsAppName reports whether Dokploy deployed an application, Compose
// deployment or preview named name on the configured server, as of its last
// inventory. A stale inventory is refreshed in the background. Only the call
// that starts the first refresh waits for it, and only until ctx ends, so a
// slow or unreachable Dokploy delays one listing at most.
func (c *Client) OwnsAppName(ctx context.Context, name string) bool {
	a := &c.names
	a.mu.Lock()
	first := a.fetched.IsZero()
	started := false
	if a.refreshing == nil && time.Since(a.fetched) > appNamesTTL {
		started = true
		done := make(chan struct{})
		a.refreshing = done
		go func() {
			defer close(done)
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			_, err := c.Inventory(ctx)
			a.mu.Lock()
			defer a.mu.Unlock()
			a.refreshing = nil
			// A failed refresh waits out the TTL before the next try too.
			if err != nil {
				a.fetched = time.Now()
			}
		}()
	}
	refreshing := a.refreshing
	a.mu.Unlock()
	if started && first {
		select {
		case <-refreshing:
		case <-ctx.Done():
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.names[name]
}
