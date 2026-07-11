// Package logstream provides a shared hub that multiplexes container log
// tailing: the hub maintains one tail per (host, container) covering the
// union of live subscriber specs, and fans parsed entries out to every
// subscriber whose spec matches. Types and method contracts are defined
// here; the implementation lands in the logstream territory.
package logstream

import (
	"context"

	"github.com/AmoabaKelvin/logdeck/internal/docker"
	"github.com/AmoabaKelvin/logdeck/internal/models"
)

// DockerProvider yields the current Docker client set; reading through it on
// every use keeps the hub correct across hot-swapped config updates.
type DockerProvider interface {
	Docker() *docker.MultiHostClient
}

// Record is one parsed log entry tagged with its origin container.
type Record struct {
	Host          string
	ContainerID   string
	ContainerName string
	Labels        map[string]string
	Entry         models.LogEntry
}

// ContainerSpec selects containers to tail. All dimensions are optional and
// ANDed together; an empty slice matches everything in that dimension.
type ContainerSpec struct {
	Hosts      []string
	Containers []string // exact container names
	Projects   []string // compose projects
}

// Hub owns the container log tails shared by all subscribers. The desired
// tail set is the union of live subscriber specs; each subscriber keeps its
// own LogOptions and delivery policy, applied when records are fanned out.
type Hub struct {
	provider DockerProvider
}

// New creates a hub that tails containers through the provider's current
// Docker client set.
func New(provider DockerProvider) *Hub {
	return &Hub{provider: provider}
}

// Subscribe registers a sink for records from containers matching spec,
// tailed with the subscriber's opts. The returned function removes the
// subscription; after it returns, sink is never called again. Subscribing
// widens the hub's desired tail set, unsubscribing narrows it.
func (h *Hub) Subscribe(spec ContainerSpec, opts models.LogOptions, sink func(Record)) (unsubscribe func()) {
	return func() {}
}

// Reconcile re-computes the desired tail set (union of live subscriber
// specs) against the currently running containers and starts/stops tails to
// match. Called on subscription changes, container lifecycle events, and
// config reloads.
func (h *Hub) Reconcile() {}

// Run drives the hub's tail supervision until ctx is cancelled. It is
// typically invoked in its own goroutine; use Wait to block until every
// tail has fully stopped.
func (h *Hub) Run(ctx context.Context) {}

// Wait blocks until the hub has shut down: Run returned and all container
// tails have stopped.
func (h *Hub) Wait() {}
