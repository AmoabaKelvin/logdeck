// Package alerts contains the alerting engine: it watches container events
// and log streams, matches them against the configured rules, and delivers
// webhook notifications. This file defines the exported surface; rule
// evaluation, windowing, and delivery land in the alerting territories.
package alerts

import (
	"context"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/docker"
	"github.com/AmoabaKelvin/logdeck/internal/logstream"
	"github.com/AmoabaKelvin/logdeck/internal/models"
)

// DockerProvider yields the current Docker client set; reading through it on
// every use keeps the engine correct across hot-swapped config updates.
type DockerProvider interface {
	Docker() *docker.MultiHostClient
}

// Engine evaluates alert rules against engine events and log records and
// records fired alerts in an in-memory history.
type Engine struct {
	provider DockerProvider
	manager  *config.Manager
	hub      *logstream.Hub
}

// NewEngine creates an alerting engine that reads rules through manager,
// watches events through the provider's Docker clients, and subscribes to
// log records through hub.
func NewEngine(provider DockerProvider, manager *config.Manager, hub *logstream.Hub) *Engine {
	return &Engine{
		provider: provider,
		manager:  manager,
		hub:      hub,
	}
}

// Start launches the engine's background loops (event watching and rule
// evaluation) tied to ctx. It returns immediately; use Wait to block until
// shutdown completes.
func (e *Engine) Start(ctx context.Context) {}

// Wait blocks until the engine's background loops have stopped after the
// Start context was cancelled.
func (e *Engine) Wait() {}

// Reconcile re-reads the rule set and the current Docker client set and
// adjusts event watches and log subscriptions to match. Called on rule
// changes and after the Docker clients are hot-swapped.
func (e *Engine) Reconcile() {}

// TestWebhook sends a test notification to the configured webhook URL and
// reports the delivery outcome.
func (e *Engine) TestWebhook(ctx context.Context) models.DeliveryResult {
	return models.DeliveryResult{}
}

// History returns the most recent fired alerts, newest first, capped at
// limit (or all entries when limit <= 0). Always returns a non-nil slice.
func (e *Engine) History(limit int) []models.Alert {
	return []models.Alert{}
}

// ClearHistory removes all recorded alert history entries.
func (e *Engine) ClearHistory() {}
