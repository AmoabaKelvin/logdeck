package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/AmoabaKelvin/logdeck/internal/api/middleware"
	"github.com/AmoabaKelvin/logdeck/internal/auth"
	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/services"
	"github.com/AmoabaKelvin/logdeck/internal/static"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

type APIRouter struct {
	router   *chi.Mux
	registry *services.Registry
	manager  *config.Manager
	version  string
}

func NewRouter(registry *services.Registry, manager *config.Manager, version string) *chi.Mux {
	r := &APIRouter{
		router:   chi.NewRouter(),
		registry: registry,
		manager:  manager,
		version:  version,
	}

	return r.Routes()
}

func WriteJsonResponse(w http.ResponseWriter, status int, data interface{}) {
	payload, err := json.Marshal(data)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_, _ = w.Write(payload)
}

func (ar *APIRouter) Routes() *chi.Mux {
	ar.router.Use(chimiddleware.RequestID)
	ar.router.Use(chimiddleware.Recoverer)
	ar.router.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
	}))

	// API routes
	ar.router.Route("/api/v1", func(r chi.Router) {
		// Access logging only for API routes (static SPA routes stay quiet)
		r.Use(middleware.AccessLog)

		// Health and version - publicly available
		r.Get("/healthz", ar.handleHealthz)
		r.Get("/version", ar.handleVersion)

		// System stats - publicly available
		r.Get("/system/stats", ar.GetSystemStats)

		// Auth endpoints (always registered, dynamic behavior)
		r.Post("/auth/login", ar.handleLogin)

		// Settings endpoints (follow same auth pattern as other routes)
		ar.registerSettingsRoutes(r)

		// All other routes go through dynamic auth middleware
		r.Group(func(protected chi.Router) {
			protected.Use(auth.DynamicMiddleware(ar.registry.Auth))

			protected.Get("/auth/me", ar.handleGetMe)
			ar.registerContainerRoutes(protected)
		})
	})

	// Serve embedded frontend static files
	staticFS, err := static.GetFileSystem()
	if err != nil {
		log.Printf("Warning: Could not load embedded frontend files: %v", err)
		log.Println("The frontend will not be available. API routes will still work.")
	} else {
		spaHandler := static.NewSPAHandler(staticFS)
		ar.router.Handle("/*", spaHandler)
	}

	return ar.router
}

func (ar *APIRouter) registerContainerRoutes(r chi.Router) {
	r.Get("/containers", ar.GetContainers)
	r.Get("/containers/stats", ar.GetContainerStats)
	r.Route("/containers/{id}", func(r chi.Router) {
		// Read-only routes (always available)
		r.Get("/", ar.GetContainer)
		r.Get("/logs/parsed", ar.GetContainerLogsParsed)
		r.Get("/env", ar.GetEnvVariables)

		// Mutating routes (blocked in read-only mode)
		r.Group(func(mutating chi.Router) {
			mutating.Use(middleware.ReadOnly(func() bool {
				return ar.registry.Config().ReadOnly
			}))
			mutating.Post("/start", ar.StartContainer)
			mutating.Post("/stop", ar.StopContainer)
			mutating.Post("/restart", ar.RestartContainer)
			mutating.Post("/remove", ar.RemoveContainer)
			mutating.Put("/env", ar.UpdateEnvVariables)
			mutating.Get("/exec", ar.HandleTerminal)
		})
	})
}

func (ar *APIRouter) registerSettingsRoutes(r chi.Router) {
	r.Route("/settings", func(r chi.Router) {
		// Settings follow the same auth pattern — protected when auth is enabled
		r.Use(auth.DynamicMiddleware(ar.registry.Auth))

		r.Get("/", ar.GetSettings)
		r.Put("/docker-hosts", ar.UpdateDockerHosts)
		r.Put("/coolify-hosts", ar.UpdateCoolifyHosts)
		r.Put("/read-only", ar.UpdateReadOnly)
		r.Put("/auth", ar.UpdateAuth)
		r.Post("/test/docker-host", ar.TestDockerHost)
		r.Post("/test/coolify-host", ar.TestCoolifyHost)
	})
}

// handleLogin delegates to the dynamic auth service.
func (ar *APIRouter) handleLogin(w http.ResponseWriter, r *http.Request) {
	svc := ar.registry.Auth()
	if svc == nil {
		http.Error(w, "Authentication is not enabled", http.StatusNotFound)
		return
	}

	var loginReq struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&loginReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if loginReq.Username == "" || loginReq.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	if err := svc.ValidateCredentials(loginReq.Username, loginReq.Password); err != nil {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	token, err := svc.GenerateToken(loginReq.Username)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"token": token,
		"user": map[string]string{
			"username": loginReq.Username,
			"role":     "admin",
		},
	})
}

// handleGetMe returns the current authenticated user's information.
func (ar *APIRouter) handleGetMe(w http.ResponseWriter, r *http.Request) {
	userValue := r.Context().Value(auth.UserContextKey)
	if userValue == nil {
		http.Error(w, "User not found in context", http.StatusUnauthorized)
		return
	}

	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"user": userValue,
	})
}
