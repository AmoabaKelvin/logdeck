package api

import (
	"encoding/json"
	"net/http"

	"github.com/AmoabaKelvin/logdeck/internal/api/middleware"
	"github.com/AmoabaKelvin/logdeck/internal/auth"
	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/docker"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

type APIRouter struct {
	router      *chi.Mux
	docker      *docker.Client
	authService *auth.Service
	config      *config.Config
}

func NewRouter(docker *docker.Client, authService *auth.Service, config *config.Config) *chi.Mux {
	r := &APIRouter{
		router:      chi.NewRouter(),
		docker:      docker,
		authService: authService,
		config:      config,
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
	ar.router.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
	}))

	ar.router.Route("/api/v1", func(r chi.Router) {
		if ar.authService != nil {
			authHandlers := NewAuthHandlers(ar.authService)
			r.Post("/auth/login", authHandlers.Login)

			r.Group(func(protected chi.Router) {
				protected.Use(auth.Middleware(ar.authService))

				protected.Get("/auth/me", authHandlers.GetMe)
				ar.registerContainerRoutes(protected)
			})
			return
		}

		ar.registerContainerRoutes(r)
	})

	return ar.router
}

func (ar *APIRouter) registerContainerRoutes(r chi.Router) {
	r.Get("/containers", ar.GetContainers)
	r.Route("/containers/{id}", func(r chi.Router) {
		// Read-only routes (always available)
		r.Get("/", ar.GetContainer)
		r.Get("/logs/parsed", ar.GetContainerLogsParsed)
		r.Get("/env", ar.GetEnvVariables)

		// Mutating routes (blocked in read-only mode)
		r.Group(func(mutating chi.Router) {
			mutating.Use(middleware.ReadOnly(ar.config))
			mutating.Post("/start", ar.StartContainer)
			mutating.Post("/stop", ar.StopContainer)
			mutating.Post("/restart", ar.RestartContainer)
			mutating.Post("/remove", ar.RemoveContainer)
			mutating.Put("/env", ar.UpdateEnvVariables)
		})
	})
}
