package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/alerts"
	"github.com/AmoabaKelvin/logdeck/internal/api"
	"github.com/AmoabaKelvin/logdeck/internal/auth"
	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/coolify"
	"github.com/AmoabaKelvin/logdeck/internal/docker"
	"github.com/AmoabaKelvin/logdeck/internal/logstore"
	"github.com/AmoabaKelvin/logdeck/internal/logstream"
	"github.com/AmoabaKelvin/logdeck/internal/services"
	"github.com/AmoabaKelvin/logdeck/internal/system"
)

// version is injected at build time via
// -ldflags "-X main.version=<version>". Defaults to "dev".
var version = "dev"

func main() {
	system.Init()

	manager := config.NewManager()
	cfg := manager.Config()

	multiHostClient, err := docker.NewMultiHostClient(cfg.DockerHosts)
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}

	// Auth: env-based first, then file-based fallback.
	authService, err := auth.NewService()
	if err != nil {
		log.Fatalf("Failed to initialize auth service: %v\nPlease ensure ALL auth environment variables are set: JWT_SECRET, ADMIN_USERNAME, and ADMIN_PASSWORD.", err)
	}
	if authService == nil {
		fc := manager.FileConfigSnapshot()
		if fc.Auth != nil && fc.Auth.Enabled {
			authService = auth.NewServiceFromFileConfig(fc.Auth)
		}
	}

	if authService == nil {
		log.Println("Authentication is DISABLED")
	} else {
		log.Println("Authentication is ENABLED")
	}

	if cfg.ReadOnly {
		log.Println("READ-ONLY MODE is ENABLED")
	}

	coolifyClient := coolify.NewMultiClient(cfg.CoolifyHosts)
	if coolifyClient != nil {
		log.Printf("Coolify integration is ENABLED (%d host configs)", len(cfg.CoolifyHosts))
	} else {
		log.Println("Coolify integration is DISABLED")
	}

	registry := services.NewRegistry(multiHostClient, coolifyClient, authService, cfg)

	logHub := logstream.New(registry)
	alertEngine := alerts.NewEngine(registry, manager, logHub)
	// logStore is nil when persistence is disabled or its database is
	// unusable; every consumer must treat that as "no stored logs".
	logStore := logstore.OpenFromConfig(manager)

	// Register hot-reload callback.
	manager.OnChange(func(newCfg *config.Config) {
		registry.UpdateConfig(newCfg)

		// Recreate Docker clients.
		newDocker, err := docker.NewMultiHostClient(newCfg.DockerHosts)
		if err != nil {
			log.Printf("Warning: failed to recreate Docker clients after config change: %v", err)
		} else {
			old := registry.SwapDocker(newDocker)
			// Close the old client after a grace period to let in-flight
			// requests (streaming logs, terminal sessions) drain.
			go func() {
				time.Sleep(30 * time.Second)
				old.Close()
			}()
			// The client set changed; re-evaluate alert watch targets and
			// converge the shared log tails onto the new client.
			alertEngine.Reconcile()
			logHub.Reconcile()
		}

		// Recreate Coolify clients.
		registry.SwapCoolify(coolify.NewMultiClient(newCfg.CoolifyHosts))

		// Recreate auth service from file config (env-based auth is immutable).
		fc := manager.FileConfigSnapshot()
		if manager.Sources().Auth == config.SourceFile && fc.Auth != nil {
			registry.SwapAuth(auth.NewServiceFromFileConfig(fc.Auth))
		}

		log.Println("Configuration reloaded successfully")
	})

	apiRouter := api.NewRouter(registry, manager, alertEngine, version)

	// No WriteTimeout/IdleTimeout: log streaming and terminal WebSockets are
	// long-lived connections and would be killed by them. ReadTimeout only
	// bounds reading the request (headers + body), so hijacked WebSockets
	// (unaffected after upgrade) and streaming responses (write-side) are safe.
	server := &http.Server{
		Addr:              ":8080",
		Handler:           apiRouter,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	// Once the first signal starts the graceful shutdown, restore default
	// signal handling so a second Ctrl-C terminates the process immediately.
	go func() {
		<-ctx.Done()
		stop()
	}()

	go logHub.Run(ctx)
	alertEngine.Start(ctx)
	if logStore != nil {
		logStore.Start(ctx, logHub, registry)
	}

	go func() {
		log.Println("Server starting on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Graceful shutdown failed: %v", err)
	}

	// Drain the alerting engine and the shared log tails after the server has
	// stopped accepting requests. The log store drains last: it feeds off the
	// hub, so its final batch can only be complete once the hub has stopped.
	alertEngine.Wait()
	logHub.Wait()
	if logStore != nil {
		logStore.Wait()
		if err := logStore.Close(); err != nil {
			log.Printf("Closing the log store failed: %v", err)
		}
	}
}
