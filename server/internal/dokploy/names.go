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
// inventory. A stale inventory is refreshed in the background. Only one never
// read is waited for, and only until ctx ends.
func (c *Client) OwnsAppName(ctx context.Context, name string) bool {
	a := &c.names
	a.mu.Lock()
	if a.refreshing == nil && time.Since(a.fetched) > appNamesTTL {
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
			// A failed refresh waits out the TTL too, so an unreachable
			// Dokploy does not slow every listing.
			if err != nil {
				a.fetched = time.Now()
			}
		}()
	}
	refreshing := a.refreshing
	cold := a.names == nil
	a.mu.Unlock()
	if cold && refreshing != nil {
		select {
		case <-refreshing:
		case <-ctx.Done():
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.names[name]
}
