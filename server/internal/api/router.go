package api

import (
	"encoding/json"
	"net/http"

	"github.com/AmoabaKelvin/logdeck/internal/auth"
	"github.com/AmoabaKelvin/logdeck/internal/docker"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

type APIRouter struct {
	router      *chi.Mux
	docker      *docker.Client
	authService *auth.Service
}

func NewRouter(docker *docker.Client, authService *auth.Service) *chi.Mux {
	r := &APIRouter{
		router:      chi.NewRouter(),
		docker:      docker,
		authService: authService,
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
		r.Get("/", ar.GetContainer)
		r.Post("/start", ar.StartContainer)
		r.Post("/stop", ar.StopContainer)
		r.Post("/restart", ar.RestartContainer)
		r.Post("/remove", ar.RemoveContainer)
		r.Get("/logs/parsed", ar.GetContainerLogsParsed)
	})
}
