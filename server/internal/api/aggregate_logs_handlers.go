package api

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/AmoabaKelvin/logdeck/internal/docker"
)

// maxAggregateTargets bounds how many container streams one aggregate
// request may open.
const maxAggregateTargets = 20

// GetAggregatedLogs serves merged logs for multiple containers. Targets are
// passed as repeated (or comma-separated) "host~id~name" triples in the
// "targets" query param; "~" and "," cannot appear in Docker container names
// or IDs. All other query params match the single-container logs endpoint.
func (ar *APIRouter) GetAggregatedLogs(w http.ResponseWriter, r *http.Request) {
	targets, err := parseAggregateTargets(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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
		stream, err := ar.registry.Docker().StreamAggregatedLogsParsed(r.Context(), targets, options)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer stream.Close()
		writeNDJSONStream(w, stream)
		return
	}

	logs, err := ar.registry.Docker().GetAggregatedLogsParsed(r.Context(), targets, options)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"logs":  logs,
		"count": len(logs),
	})
}

func parseAggregateTargets(r *http.Request) ([]docker.LogTarget, error) {
	var targets []docker.LogTarget

	for _, value := range r.URL.Query()["targets"] {
		for _, triple := range strings.Split(value, ",") {
			triple = strings.TrimSpace(triple)
			if triple == "" {
				continue
			}
			parts := strings.SplitN(triple, "~", 3)
			if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
				return nil, fmt.Errorf("invalid target %q: expected host~id~name", triple)
			}
			target := docker.LogTarget{Host: parts[0], ID: parts[1]}
			if len(parts) == 3 {
				target.Name = parts[2]
			}
			targets = append(targets, target)
		}
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("targets parameter is required")
	}
	if len(targets) > maxAggregateTargets {
		return nil, fmt.Errorf("too many targets: %d (max %d)", len(targets), maxAggregateTargets)
	}

	return targets, nil
}

// writeNDJSONStream copies an NDJSON stream to the client, flushing per read
// so follow mode delivers lines as they arrive. Mirrors streamParsedLogs.
func writeNDJSONStream(w http.ResponseWriter, stream io.Reader) {
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
