package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/coolify"
	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/AmoabaKelvin/logdeck/internal/system"
	"github.com/go-chi/chi/v5"
)

// containerParams returns the required "host" query param and the "{id}" path
// param shared by all per-container routes. When host is missing it writes a
// 400 response and returns ok=false.
func containerParams(w http.ResponseWriter, r *http.Request) (host, id string, ok bool) {
	host = r.URL.Query().Get("host")
	if host == "" {
		http.Error(w, "host parameter is required", http.StatusBadRequest)
		return "", "", false
	}
	return host, chi.URLParam(r, "id"), true
}

func (ar *APIRouter) GetSystemStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stats, err := system.GetStats(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	WriteJsonResponse(w, http.StatusOK, stats)
}

func (ar *APIRouter) GetContainerStats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	stats, err := ar.registry.Docker().GetAllRunningContainerStats(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	WriteJsonResponse(w, http.StatusOK, models.ContainerStatsResponse{
		Stats: stats,
	})
}

func (ar *APIRouter) GetContainers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	containersMap, hostErrors, err := ar.registry.Docker().ListContainersAllHosts(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	allContainers := []models.ContainerInfo{}
	for _, containers := range containersMap {
		allContainers = append(allContainers, containers...)
	}

	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"containers":        allContainers,
		"hosts":             ar.registry.Docker().GetHosts(),
		"readOnly":          ar.registry.Config().ReadOnly,
		"hostErrors":        hostErrorMessages(hostErrors),
		"coolifyConfigured": ar.registry.Coolify() != nil,
	})
}

func (ar *APIRouter) GetContainer(w http.ResponseWriter, r *http.Request) {
	host, id, ok := containerParams(w, r)
	if !ok {
		return
	}

	container, err := ar.registry.Docker().GetContainer(r.Context(), host, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"container": container,
	})
}

func (ar *APIRouter) StartContainer(w http.ResponseWriter, r *http.Request) {
	host, id, ok := containerParams(w, r)
	if !ok {
		return
	}

	err := ar.registry.Docker().StartContainer(r.Context(), host, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"message": "Container started",
	})
}

func (ar *APIRouter) StopContainer(w http.ResponseWriter, r *http.Request) {
	host, id, ok := containerParams(w, r)
	if !ok {
		return
	}

	err := ar.registry.Docker().StopContainer(r.Context(), host, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"message": "Container stopped",
	})
}

func (ar *APIRouter) RestartContainer(w http.ResponseWriter, r *http.Request) {
	host, id, ok := containerParams(w, r)
	if !ok {
		return
	}

	err := ar.registry.Docker().RestartContainer(r.Context(), host, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"message": "Container restarted",
	})
}

func (ar *APIRouter) RemoveContainer(w http.ResponseWriter, r *http.Request) {
	host, id, ok := containerParams(w, r)
	if !ok {
		return
	}

	err := ar.registry.Docker().RemoveContainer(r.Context(), host, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"message": "Container removed",
	})
}

// execRunTimeout bounds a single RunCommand invocation.
const execRunTimeout = 60 * time.Second

type runCommandRequest struct {
	Command string `json:"command"`
}

// RunCommand runs one non-interactive command in a container through the shell
// and returns its separated stdout, stderr, and exit code. This is the
// programmatic counterpart to the interactive terminal (HandleTerminal): no
// TTY, so streams stay distinct and the exit code is authoritative.
func (ar *APIRouter) RunCommand(w http.ResponseWriter, r *http.Request) {
	host, id, ok := containerParams(w, r)
	if !ok {
		return
	}

	var req runCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Command) == "" {
		http.Error(w, "command is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), execRunTimeout)
	defer cancel()

	stdout, stderr, exitCode, err := ar.registry.Docker().RunExec(ctx, host, id, []string{"/bin/sh", "-c", req.Command})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"stdout":   stdout,
		"stderr":   stderr,
		"exitCode": exitCode,
	})
}

func (ar *APIRouter) GetContainerLogsParsed(w http.ResponseWriter, r *http.Request) {
	host, id, ok := containerParams(w, r)
	if !ok {
		return
	}

	options := parseLogOptions(r)

	if options.Search != "" {
		if _, err := regexp.Compile(options.Search); err != nil {
			http.Error(w, fmt.Sprintf("invalid search pattern: %v", err), http.StatusBadRequest)
			return
		}
	}

	if options.Follow {
		ar.streamParsedLogs(w, r, host, id, options)
		return
	}

	logs, err := ar.registry.Docker().GetContainerLogsParsed(r.Context(), host, id, options)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"logs":  logs,
		"count": len(logs),
	})
}

func (ar *APIRouter) streamParsedLogs(w http.ResponseWriter, r *http.Request, host, id string, options models.LogOptions) {
	stream, err := ar.registry.Docker().StreamContainerLogsParsed(r.Context(), host, id, options)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stream.Close()

	writeNDJSONStream(w, stream)
}

func (ar *APIRouter) GetContainerEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx := r.Context()
	events := ar.registry.Docker().StreamContainerEvents(ctx)

	encoder := json.NewEncoder(w)
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			if err := encoder.Encode(event); err != nil {
				return
			}
			flusher.Flush()
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
		options.Tail = clampTail(tail)
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

	if search := query.Get("search"); search != "" {
		options.Search = search
	}

	if level := query.Get("level"); level != "" {
		options.Level = level
	}

	return options
}

// maxTailLines bounds how many log lines a single request can pull into
// memory. "all" (and any other non-numeric value) is treated as the max.
const maxTailLines = 10000

func clampTail(tail string) string {
	n, err := strconv.Atoi(tail)
	if err != nil || n < 0 || n > maxTailLines {
		return strconv.Itoa(maxTailLines)
	}
	return tail
}

func (ar *APIRouter) GetEnvVariables(w http.ResponseWriter, r *http.Request) {
	host, id, ok := containerParams(w, r)
	if !ok {
		return
	}

	envVariables, err := ar.registry.Docker().GetEnvVariables(r.Context(), host, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"env": envVariables,
	})
}

var envKeyRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_\-\.]*$`)

func (ar *APIRouter) UpdateEnvVariables(w http.ResponseWriter, r *http.Request) {
	host, id, ok := containerParams(w, r)
	if !ok {
		return
	}

	var envVariables models.EnvVariables
	if err := json.NewDecoder(r.Body).Decode(&envVariables); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for key := range envVariables.Env {
		if !envKeyRegex.MatchString(key) {
			http.Error(w, fmt.Sprintf("invalid environment variable key: %s", key), http.StatusBadRequest)
			return
		}
	}

	newContainerID, labels, err := ar.registry.Docker().SetEnvVariables(r.Context(), host, id, envVariables.Env)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]any{
		"message":          "Environment variables updated",
		"new_container_id": newContainerID,
	}

	// Best-effort sync to Coolify API
	coolifyClient := ar.registry.Coolify().GetClient(host)
	coolifyResource := coolify.ExtractResourceInfo(labels)
	if coolifyClient != nil && coolifyResource != nil {
		syncCtx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		if syncErr := coolifyClient.SyncEnvVars(syncCtx, coolifyResource, envVariables.Env); syncErr != nil {
			log.Printf("Warning: failed to sync env vars to Coolify for host %s: %v", host, syncErr)
			response["coolify_synced"] = false
			response["coolify_error"] = syncErr.Error()
		} else {
			response["coolify_synced"] = true
		}
	}

	WriteJsonResponse(w, http.StatusOK, response)
}
