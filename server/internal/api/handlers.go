package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/go-chi/chi/v5"
)

func (ar *APIRouter) GetContainers(w http.ResponseWriter, r *http.Request) {
	containers, err := ar.docker.ListContainers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"containers": containers,
		"readOnly":   ar.config.ReadOnly,
	})
}

func (ar *APIRouter) GetContainer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	container, err := ar.docker.GetContainer(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"container": container,
	})
}

func (ar *APIRouter) StartContainer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	err := ar.docker.StartContainer(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"message": "Container started",
	})
}

func (ar *APIRouter) StopContainer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	err := ar.docker.StopContainer(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"message": "Container stopped",
	})
}

func (ar *APIRouter) RestartContainer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	err := ar.docker.RestartContainer(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"message": "Container restarted",
	})
}

func (ar *APIRouter) RemoveContainer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	err := ar.docker.RemoveContainer(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"message": "Container removed",
	})
}

func (ar *APIRouter) GetContainerLogsParsed(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Parse query parameters for log options
	options := parseLogOptions(r)

	if options.Follow {
		ar.streamParsedLogs(w, id, options)
		return
	}

	logs, err := ar.docker.GetContainerLogsParsed(id, options)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"logs":  logs,
		"count": len(logs),
	})
}

func (ar *APIRouter) streamParsedLogs(w http.ResponseWriter, id string, options models.LogOptions) {
	stream, err := ar.docker.StreamContainerLogsParsed(id, options)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stream.Close()

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	buffer := make([]byte, 32*1024)
	for {
		n, readErr := stream.Read(buffer)
		if n > 0 {
			if _, writeErr := w.Write(buffer[:n]); writeErr != nil {
				break
			}
			flusher.Flush()
		}
		if readErr != nil {
			break
		}
	}
}

func parseLogOptions(r *http.Request) models.LogOptions {
	query := r.URL.Query()

	options := models.DefaultLogOptions()

	if follow := query.Get("follow"); follow != "" {
		options.Follow, _ = strconv.ParseBool(follow)
	}

	if timestamps := query.Get("timestamps"); timestamps != "" {
		options.Timestamps, _ = strconv.ParseBool(timestamps)
	}

	if since := query.Get("since"); since != "" {
		options.Since = since
	}

	if until := query.Get("until"); until != "" {
		options.Until = until
	}

	if tail := query.Get("tail"); tail != "" {
		options.Tail = tail
	}

	if details := query.Get("details"); details != "" {
		options.Details, _ = strconv.ParseBool(details)
	}

	if stdout := query.Get("stdout"); stdout != "" {
		options.ShowStdout, _ = strconv.ParseBool(stdout)
	}

	if stderr := query.Get("stderr"); stderr != "" {
		options.ShowStderr, _ = strconv.ParseBool(stderr)
	}

	return options
}

func (ar *APIRouter) GetEnvVariables(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	envVariables, err := ar.docker.GetEnvVariables(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"env": envVariables,
	})
}

func (ar *APIRouter) UpdateEnvVariables(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var envVariables models.EnvVariables
	if err := json.NewDecoder(r.Body).Decode(&envVariables); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate environment variable keys
	envKeyRegex := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	for key := range envVariables.Env {
		if !envKeyRegex.MatchString(key) {
			http.Error(w, fmt.Sprintf("invalid environment variable key: %s", key), http.StatusBadRequest)
			return
		}
	}

	newContainerID, err := ar.docker.SetEnvVariables(id, envVariables.Env)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"message":          "Environment variables updated",
		"new_container_id": newContainerID,
	})
}
