package api

import (
	"encoding/json"
	"net/http"

	"github.com/AmoabaKelvin/logdeck/internal/docker"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

type APIRouter struct {
	router *chi.Mux
	docker *docker.Client
}

func NewRouter(docker *docker.Client) *chi.Mux {
	r := &APIRouter{
		router: chi.NewRouter(),
		docker: docker,
	}

	return r.Routes()
}

func WriteJsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (ar *APIRouter) Routes() *chi.Mux {
	ar.router.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
	}))

	ar.router.Mount("/api", ar.router)

	ar.router.Get("/api/v1/containers", ar.GetContainers)
	ar.router.Route("/api/v1/containers/{id}", func(r chi.Router) {
		r.Get("/", ar.GetContainer)
		r.Post("/start", ar.StartContainer)
		r.Post("/stop", ar.StopContainer)
		r.Post("/restart", ar.RestartContainer)
		r.Post("/remove", ar.RemoveContainer)
		r.Get("/logs/parsed", ar.GetContainerLogsParsed)
	})

	return ar.router
}
