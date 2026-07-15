package services

import (
	"sync"
	"testing"

	"github.com/AmoabaKelvin/logdeck/internal/auth"
	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/coolify"
	"github.com/AmoabaKelvin/logdeck/internal/docker"
)

func TestNewRegistryGetters(t *testing.T) {
	dc := &docker.MultiHostClient{}
	cc := &coolify.MultiClient{}
	as := &auth.Service{}
	cfg := &config.Config{ReadOnly: true}

	r := NewRegistry(dc, cc, as, cfg)

	if r.Docker() != dc {
		t.Error("Docker() did not return the injected client")
	}
	if r.Coolify() != cc {
		t.Error("Coolify() did not return the injected client")
	}
	if r.Auth() != as {
		t.Error("Auth() did not return the injected service")
	}
	if r.Config() != cfg {
		t.Error("Config() did not return the injected config")
	}
}

func TestSwapDockerReturnsOld(t *testing.T) {
	old := &docker.MultiHostClient{}
	r := NewRegistry(old, nil, nil, nil)

	next := &docker.MultiHostClient{}
	returned := r.SwapDocker(next)

	if returned != old {
		t.Error("SwapDocker did not return the previous client")
	}
	if r.Docker() != next {
		t.Error("Docker() did not reflect the swapped-in client")
	}
}

func TestSwapCoolifyAndAuth(t *testing.T) {
	r := NewRegistry(nil, nil, nil, nil)

	cc := &coolify.MultiClient{}
	r.SwapCoolify(cc)
	if r.Coolify() != cc {
		t.Error("SwapCoolify did not take effect")
	}

	as := &auth.Service{}
	r.SwapAuth(as)
	if r.Auth() != as {
		t.Error("SwapAuth did not take effect")
	}
}

func TestUpdateConfig(t *testing.T) {
	r := NewRegistry(nil, nil, nil, &config.Config{ReadOnly: false})
	next := &config.Config{ReadOnly: true}
	r.UpdateConfig(next)
	if r.Config() != next {
		t.Error("UpdateConfig did not take effect")
	}
}

// TestRegistryConcurrentReadSwapNoRace runs concurrent readers against a stream
// of swaps. Under `go test -race` this fails if the RWMutex is dropped or a
// reader observes a torn pointer. Each Get must return one of the two valid,
// fully-constructed clients, never a partial state.
func TestRegistryConcurrentReadSwapNoRace(t *testing.T) {
	a := &docker.MultiHostClient{}
	b := &docker.MultiHostClient{}
	r := NewRegistry(a, nil, nil, nil)

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Writers flip the docker client back and forth.
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			toggle := a
			for {
				select {
				case <-stop:
					return
				default:
					if toggle == a {
						toggle = b
					} else {
						toggle = a
					}
					r.SwapDocker(toggle)
				}
			}
		}()
	}

	// Readers must always see one of the two valid pointers.
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					got := r.Docker()
					if got != a && got != b {
						t.Errorf("reader observed unexpected client %p", got)
						return
					}
				}
			}
		}()
	}

	// Let the goroutines race for a while.
	for i := 0; i < 5000; i++ {
		r.Coolify()
		r.Config()
	}
	close(stop)
	wg.Wait()
}
