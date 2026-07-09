package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/auth"
	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/docker/docker/api/types"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: checkWebSocketOrigin,
}

// checkWebSocketOrigin allows non-browser clients (no Origin header) and
// browser requests whose Origin host matches the request Host, preventing
// cross-site WebSocket hijacking.
func checkWebSocketOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

type ResizeMessage struct {
	Type string `json:"type"`
	Cols uint   `json:"cols"`
	Rows uint   `json:"rows"`
}

const terminalBufferSize = 32 * 1024

func (ar *APIRouter) HandleTerminal(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	host := r.URL.Query().Get("host")

	if host == "" {
		http.Error(w, "host parameter is required", http.StatusBadRequest)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %v", err)
		return
	}
	defer ws.Close()

	ctx := r.Context()

	execID, resp, err := ar.startExecSession(ctx, host, id)
	if err != nil {
		log.Printf("terminal session init failed: %v", err)
		if writeErr := ws.WriteMessage(websocket.TextMessage, []byte("Error creating terminal session: "+err.Error())); writeErr != nil {
			log.Printf("failed to send error message to websocket: %v", writeErr)
		}
		return
	}
	defer resp.Close()

	// Exec sessions grant shell access inside containers - audit them.
	user := execAuditUser(r)
	start := time.Now()
	slog.Info("exec session opened",
		"user", user, "host", host, "container", id, "exec_id", execID)
	defer func() {
		slog.Info("exec session closed",
			"user", user, "host", host, "container", id, "exec_id", execID,
			"duration", time.Since(start))
	}()

	outputDone := make(chan struct{})
	inputDone := make(chan struct{})

	go streamContainerOutput(resp.Reader, ws, outputDone)
	go func() {
		defer close(inputDone)
		// resp.Conn doubles as the closer: closing it on client disconnect
		// tears down the container stream and unblocks resp.Reader.
		ar.forwardClientInput(ctx, host, execID, resp.Conn, resp.Conn, ws)
	}()

	// Return when either direction finishes; the deferred ws.Close and
	// resp.Close tear down the other side.
	select {
	case <-outputDone:
	case <-inputDone:
	}
}

// execAuditUser returns the authenticated username for audit logs, or
// "anonymous" when auth is disabled.
func execAuditUser(r *http.Request) string {
	if user, ok := r.Context().Value(auth.UserContextKey).(models.User); ok {
		return user.Username
	}
	return "anonymous"
}

func (ar *APIRouter) startExecSession(ctx context.Context, host, containerID string) (string, *types.HijackedResponse, error) {
	execID, err := ar.registry.Docker().CreateExec(ctx, host, containerID)
	if err != nil {
		return "", nil, fmt.Errorf("create exec failed: %w", err)
	}

	resp, err := ar.registry.Docker().AttachExec(ctx, host, execID)
	if err != nil {
		return "", nil, fmt.Errorf("attach exec failed: %w", err)
	}

	return execID, resp, nil
}

func streamContainerOutput(reader io.Reader, ws *websocket.Conn, done chan<- struct{}) {
	defer close(done)

	buffer := make([]byte, terminalBufferSize)
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			if writeErr := ws.WriteMessage(websocket.BinaryMessage, buffer[:n]); writeErr != nil {
				log.Printf("error writing to websocket: %v", writeErr)
				return
			}
		}

		if err != nil {
			if err != io.EOF {
				log.Printf("error reading from container: %v", err)
			}
			return
		}
	}
}

func (ar *APIRouter) forwardClientInput(
	ctx context.Context,
	host,
	execID string,
	writer io.Writer,
	closer io.Closer,
	ws *websocket.Conn,
) {
	defer closer.Close()

	for {
		messageType, data, err := ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("websocket closed unexpectedly: %v", err)
			}
			return
		}

		if messageType == websocket.TextMessage {
			var msg ResizeMessage
			if err := json.Unmarshal(data, &msg); err == nil && msg.Type == "resize" {
				if err := ar.registry.Docker().ResizeExec(ctx, host, execID, msg.Rows, msg.Cols); err != nil {
					log.Printf("failed to resize terminal: %v", err)
				}
				continue
			}
		}

		if _, err := writer.Write(data); err != nil {
			log.Printf("failed to write to container: %v", err)
			return
		}
	}
}
