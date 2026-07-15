package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

// mcpAPITokenPrefix mirrors auth.APITokenPrefix: scoped API tokens carry it,
// admin session JWTs do not.
const mcpAPITokenPrefix = "ldk_"

// mcpMaxTail caps how many log lines a single tool call returns. The server
// allows 10000, which is too large for a model's context window.
const mcpMaxTail = 500

// mcpOptions selects which action tiers are advertised. The read and lifecycle
// tiers are always registered; these gate the sensitive ones.
type mcpOptions struct {
	destructive bool // remove_container
	exec        bool // run_command
}

const mcpLong = `Run a Model Context Protocol server over stdio so an AI assistant can
query and manage LogDeck through the same HTTP API the CLI uses.

Read and lifecycle (start/stop/restart) tools are always registered. Sensitive
tools are opt-in:
  --allow-destructive   register remove_container
  --allow-exec          register run_command (one-shot exec in a container)
  --allow-all           register all of the above

The token scope is the hard boundary and is enforced by the server: a read-only
API token is rejected on every mutation regardless of these flags, which only
control which tools are advertised.

Environment-variable and settings writes are intentionally not exposed in this
version.`

func newMCPCmd(a *app) *cobra.Command {
	var allowDestructive, allowExec, allowAll bool

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run a Model Context Protocol server over stdio",
		Long:  mcpLong,
		Args:  cobra.NoArgs,
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			opts := mcpOptions{
				destructive: allowDestructive || allowAll,
				exec:        allowExec || allowAll,
			}

			if a.conn.token != "" && !strings.HasPrefix(a.conn.token, mcpAPITokenPrefix) {
				fmt.Fprintln(os.Stderr, "MCP: warning: token is not an ldk_ API token; it looks like an admin session token. Prefer a scoped API token from LogDeck Settings.")
			}

			server := mcp.NewServer(&mcp.Implementation{Name: "logdeck", Version: cmd.Root().Version}, nil)
			registerMCPTools(server, a, opts)

			tiers := []string{"read", "lifecycle"}
			if opts.destructive {
				tiers = append(tiers, "destructive")
			}
			if opts.exec {
				tiers = append(tiers, "exec")
			}
			fmt.Fprintf(os.Stderr, "MCP: %s enabled - server %s\n", strings.Join(tiers, " + "), a.conn.url)

			return server.Run(cmd.Context(), &mcp.StdioTransport{})
		}),
	}

	cmd.Flags().BoolVar(&allowDestructive, "allow-destructive", false, "register remove_container")
	cmd.Flags().BoolVar(&allowExec, "allow-exec", false, "register run_command (one-shot exec)")
	cmd.Flags().BoolVar(&allowAll, "allow-all", false, "register all action tools (destructive + exec)")
	return cmd
}

// registerMCPTools registers the tool set for the given tiers and returns the
// names it registered, so flag gating can be unit-tested without a live server.
func registerMCPTools(s *mcp.Server, a *app, opts mcpOptions) []string {
	var names []string
	register := func(t *mcp.Tool) { names = append(names, t.Name) }

	// --- Read tier (always registered) ---

	type listContainersInput struct {
		State string `json:"state,omitempty" jsonschema:"filter by state: running, exited, paused, restarting, dead"`
		Host  string `json:"host,omitempty" jsonschema:"filter by host name"`
	}
	tool := &mcp.Tool{Name: "list_containers", Description: "List containers across all hosts, including stopped ones and their health state.", Annotations: readOnlyAnnot()}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in listContainersInput) (*mcp.CallToolResult, any, error) {
		var resp struct {
			Containers []map[string]any `json:"containers"`
			HostErrors []hostError      `json:"hostErrors"`
			ReadOnly   bool             `json:"readOnly"`
		}
		if err := a.client.get(ctx, "/containers", nil, &resp); err != nil {
			return nil, nil, err
		}
		filtered := make([]map[string]any, 0, len(resp.Containers))
		for _, c := range resp.Containers {
			if in.State != "" && asString(c["state"]) != in.State {
				continue
			}
			if in.Host != "" && asString(c["host"]) != in.Host {
				continue
			}
			filtered = append(filtered, c)
		}
		return mcpJSON(map[string]any{"containers": filtered, "hostErrors": resp.HostErrors, "readOnly": resp.ReadOnly})
	})
	register(tool)

	type getLogsInput struct {
		Container string `json:"container" jsonschema:"container name or ID"`
		Host      string `json:"host,omitempty" jsonschema:"host name (disambiguates duplicate names)"`
		Tail      int    `json:"tail,omitempty" jsonschema:"lines from the end of the logs (default 100, max 500)"`
		Level     string `json:"level,omitempty" jsonschema:"filter by level: TRACE, DEBUG, INFO, WARN, ERROR, FATAL, PANIC"`
		Search    string `json:"search,omitempty" jsonschema:"filter by regular expression"`
		Since     string `json:"since,omitempty" jsonschema:"only logs after this time (RFC3339 or relative: 30s, 15m, 2h, 1d)"`
		Until     string `json:"until,omitempty" jsonschema:"only logs before this time (RFC3339 or relative)"`
	}
	tool = &mcp.Tool{Name: "get_logs", Description: "Read recent parsed logs for one container. Never follows.", Annotations: readOnlyAnnot()}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in getLogsInput) (*mcp.CallToolResult, any, error) {
		container, err := a.resolve(ctx, in.Container, in.Host)
		if err != nil {
			return nil, nil, err
		}
		f := logFlags{tail: clampTail(in.Tail), level: in.Level, search: in.Search, since: in.Since, until: in.Until}
		query, err := f.query(time.Now())
		if err != nil {
			return nil, nil, err
		}
		query.Set("host", container.Host)
		var resp struct {
			Logs  []logEntry `json:"logs"`
			Count int        `json:"count"`
		}
		if err := a.client.get(ctx, "/containers/"+container.ID+"/logs/parsed", query, &resp); err != nil {
			return nil, nil, err
		}
		return mcpJSON(resp)
	})
	register(tool)

	type searchLogsInput struct {
		Search string `json:"search" jsonschema:"regular expression to search for across containers"`
		Host   string `json:"host,omitempty" jsonschema:"only search containers on this host"`
		Level  string `json:"level,omitempty" jsonschema:"filter by level: TRACE, DEBUG, INFO, WARN, ERROR, FATAL, PANIC"`
		Since  string `json:"since,omitempty" jsonschema:"only logs after this time (RFC3339 or relative; default 15m)"`
		Tail   int    `json:"tail,omitempty" jsonschema:"lines scanned per container (default 100, max 500)"`
	}
	tool = &mcp.Tool{Name: "search_logs", Description: "Search recent logs of every running container, merged by timestamp.", Annotations: readOnlyAnnot()}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in searchLogsInput) (*mcp.CallToolResult, any, error) {
		if _, err := regexp.Compile(in.Search); err != nil {
			return nil, nil, fmt.Errorf("invalid regex %q: %v", in.Search, err)
		}
		since := in.Since
		if since == "" {
			since = "15m"
		}
		sinceValue, err := parseTimeArg(since, time.Now())
		if err != nil {
			return nil, nil, err
		}
		resp, err := a.fetchContainers(ctx)
		if err != nil {
			return nil, nil, err
		}
		var running []containerInfo
		for _, c := range resp.Containers {
			if c.State != "running" {
				continue
			}
			if in.Host != "" && c.Host != in.Host {
				continue
			}
			running = append(running, c)
		}
		if len(running) == 0 {
			return mcpJSON(map[string]any{"logs": []logEntry{}, "count": 0})
		}
		query := url.Values{}
		query.Set("search", in.Search)
		query.Set("tail", strconv.Itoa(clampTail(in.Tail)))
		if sinceValue != "" {
			query.Set("since", sinceValue)
		}
		if in.Level != "" {
			query.Set("level", strings.ToUpper(in.Level))
		}
		logs, err := a.aggregatedLogs(ctx, buildTargets(running), query)
		if err != nil {
			return nil, nil, err
		}
		if logs == nil {
			logs = []logEntry{}
		}
		return mcpJSON(map[string]any{"logs": logs, "count": len(logs)})
	})
	register(tool)

	type inspectInput struct {
		Container string `json:"container" jsonschema:"container name or ID"`
		Host      string `json:"host,omitempty" jsonschema:"host name (disambiguates duplicate names)"`
	}
	tool = &mcp.Tool{Name: "inspect_container", Description: "Return the full inspect document for one container.", Annotations: readOnlyAnnot()}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in inspectInput) (*mcp.CallToolResult, any, error) {
		container, err := a.resolve(ctx, in.Container, in.Host)
		if err != nil {
			return nil, nil, err
		}
		var resp struct {
			Container json.RawMessage `json:"container"`
		}
		if err := a.client.get(ctx, "/containers/"+container.ID+"/", url.Values{"host": {container.Host}}, &resp); err != nil {
			return nil, nil, err
		}
		var inspect any
		if err := json.Unmarshal(resp.Container, &inspect); err != nil {
			return nil, nil, err
		}
		return mcpJSON(inspect)
	})
	register(tool)

	type listEventsInput struct {
		Wait int `json:"wait,omitempty" jsonschema:"seconds to collect lifecycle events before returning (default 3, max 15)"`
	}
	tool = &mcp.Tool{Name: "list_events", Description: "Collect container lifecycle events from all hosts for a short window, then return.", Annotations: readOnlyAnnot()}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in listEventsInput) (*mcp.CallToolResult, any, error) {
		ctx, cancel := context.WithTimeout(ctx, time.Duration(clampInt(in.Wait, 3, 15))*time.Second)
		defer cancel()
		body, err := a.client.stream(ctx, "/events", nil)
		if err != nil {
			return nil, nil, err
		}
		defer body.Close()
		events := []containerEvent{}
		_ = a.scanNDJSON(ctx, body, func(line []byte) error {
			var event containerEvent
			if err := json.Unmarshal(line, &event); err == nil {
				events = append(events, event)
			}
			return nil
		})
		return mcpJSON(map[string]any{"events": events, "count": len(events)})
	})
	register(tool)

	type containerStatsInput struct {
		Container string `json:"container,omitempty" jsonschema:"limit to one container (name or ID); omit for all"`
		Host      string `json:"host,omitempty" jsonschema:"host name (disambiguates duplicate names)"`
	}
	tool = &mcp.Tool{Name: "container_stats", Description: "Show CPU and memory usage for running containers.", Annotations: readOnlyAnnot()}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in containerStatsInput) (*mcp.CallToolResult, any, error) {
		var resp struct {
			Stats []containerStats `json:"stats"`
		}
		if err := a.client.get(ctx, "/containers/stats", nil, &resp); err != nil {
			return nil, nil, err
		}
		stats := resp.Stats
		if in.Container != "" {
			container, err := a.resolve(ctx, in.Container, in.Host)
			if err != nil {
				return nil, nil, err
			}
			stats = nil
			for _, st := range resp.Stats {
				if st.ID == container.ID && st.Host == container.Host {
					stats = append(stats, st)
				}
			}
		}
		if stats == nil {
			stats = []containerStats{}
		}
		return mcpJSON(map[string]any{"stats": stats})
	})
	register(tool)

	tool = &mcp.Tool{Name: "host_stats", Description: "Show engine-level info for every configured Docker host.", Annotations: readOnlyAnnot()}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		var resp struct {
			Hosts []hostInfo `json:"hosts"`
		}
		if err := a.client.get(ctx, "/hosts/stats", nil, &resp); err != nil {
			return nil, nil, err
		}
		return mcpJSON(resp)
	})
	register(tool)

	registerListTool(s, a, register, "list_images", "List images across all hosts.", "/images", "images")
	registerListTool(s, a, register, "list_volumes", "List volumes across all hosts.", "/volumes", "volumes")
	registerListTool(s, a, register, "list_networks", "List networks across all hosts.", "/networks", "networks")

	type historySearchInput struct {
		Container string `json:"container" jsonschema:"logical container name"`
		Host      string `json:"host,omitempty" jsonschema:"host name"`
		Search    string `json:"search,omitempty" jsonschema:"text or regex to match"`
		Regex     bool   `json:"regex,omitempty" jsonschema:"treat search as a regular expression"`
		Levels    string `json:"levels,omitempty" jsonschema:"comma-separated levels (e.g. ERROR,WARN)"`
		Since     string `json:"since,omitempty" jsonschema:"only logs after this time (RFC3339 or relative)"`
		Until     string `json:"until,omitempty" jsonschema:"only logs before this time (RFC3339 or relative)"`
		Limit     int    `json:"limit,omitempty" jsonschema:"max lines per page (default 100, max 500)"`
		Cursor    string `json:"cursor,omitempty" jsonschema:"nextCursor from a previous page to fetch older lines"`
	}
	tool = &mcp.Tool{Name: "history_search", Description: "Search persisted (stored) logs for a container. Pages backwards; follow nextCursor for older lines.", Annotations: readOnlyAnnot()}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in historySearchInput) (*mcp.CallToolResult, any, error) {
		query := url.Values{}
		query.Set("container", in.Container)
		if in.Host != "" {
			query.Set("host", in.Host)
		}
		if in.Search != "" {
			query.Set("search", in.Search)
		}
		if in.Regex {
			query.Set("regex", "true")
		}
		if in.Levels != "" {
			query.Set("levels", strings.ToUpper(in.Levels))
		}
		since, err := parseTimeArg(in.Since, time.Now())
		if err != nil {
			return nil, nil, err
		}
		if since != "" {
			query.Set("since", since)
		}
		until, err := parseTimeArg(in.Until, time.Now())
		if err != nil {
			return nil, nil, err
		}
		if until != "" {
			query.Set("until", until)
		}
		query.Set("limit", strconv.Itoa(clampTail(in.Limit)))
		if in.Cursor != "" {
			query.Set("cursor", in.Cursor)
		}
		var resp map[string]any
		if err := a.client.get(ctx, "/history/logs", query, &resp); err != nil {
			return nil, nil, err
		}
		return mcpJSON(resp)
	})
	register(tool)

	tool = &mcp.Tool{Name: "history_status", Description: "Report whether log persistence is enabled and how much disk stored logs use.", Annotations: readOnlyAnnot()}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		var resp map[string]any
		if err := a.client.get(ctx, "/history/status", nil, &resp); err != nil {
			return nil, nil, err
		}
		return mcpJSON(resp)
	})
	register(tool)

	tool = &mcp.Tool{Name: "history_containers", Description: "List every logical container that has stored logs.", Annotations: readOnlyAnnot()}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		var resp map[string]any
		if err := a.client.get(ctx, "/history/containers", nil, &resp); err != nil {
			return nil, nil, err
		}
		return mcpJSON(resp)
	})
	register(tool)

	// --- Lifecycle tier (always registered) ---

	registerAction(s, a, register, "start_container", "start", "started", "Start a stopped container.", lifecycleAnnot())
	registerAction(s, a, register, "stop_container", "stop", "stopped", "Stop a running container.", lifecycleAnnot())
	registerAction(s, a, register, "restart_container", "restart", "restarted", "Restart a container.", lifecycleAnnot())

	// --- Destructive tier ---

	if opts.destructive {
		registerAction(s, a, register, "remove_container", "remove", "removed", "Remove a container. This is irreversible.", destructiveAnnot())
	}

	// --- Exec tier ---

	if opts.exec {
		registerRunCommand(s, a, register)
	}

	return names
}

// containerRef is the shared input for tools that target one container.
type containerRef struct {
	Container string `json:"container" jsonschema:"container name or ID"`
	Host      string `json:"host,omitempty" jsonschema:"host name (disambiguates duplicate names)"`
}

// registerAction registers a lifecycle/removal tool that resolves a container
// and POSTs the matching endpoint, mirroring newActionCmd.
func registerAction(s *mcp.Server, a *app, register func(*mcp.Tool), name, endpoint, past, description string, annot *mcp.ToolAnnotations) {
	tool := &mcp.Tool{Name: name, Description: description, Annotations: annot}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in containerRef) (*mcp.CallToolResult, any, error) {
		container, err := a.resolve(ctx, in.Container, in.Host)
		if err != nil {
			return nil, nil, err
		}
		if err := a.client.post(ctx, "/containers/"+container.ID+"/"+endpoint, url.Values{"host": {container.Host}}, nil, nil); err != nil {
			return nil, nil, err
		}
		return mcpJSON(map[string]string{
			"message": "container " + past,
			"name":    containerName(container),
			"id":      container.ID,
			"host":    container.Host,
		})
	})
	register(tool)
}

// registerListTool registers a host-filterable list tool for a resource
// endpoint that returns {"<key>": [...], "hostErrors": [...]}.
func registerListTool(s *mcp.Server, a *app, register func(*mcp.Tool), name, description, path, key string) {
	type hostFilter struct {
		Host string `json:"host,omitempty" jsonschema:"filter by host name"`
	}
	tool := &mcp.Tool{Name: name, Description: description, Annotations: readOnlyAnnot()}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in hostFilter) (*mcp.CallToolResult, any, error) {
		var resp struct {
			Items      []map[string]any `json:"items"`
			HostErrors []hostError      `json:"hostErrors"`
		}
		raw := map[string]json.RawMessage{}
		if err := a.client.get(ctx, path, nil, &raw); err != nil {
			return nil, nil, err
		}
		_ = json.Unmarshal(raw[key], &resp.Items)
		_ = json.Unmarshal(raw["hostErrors"], &resp.HostErrors)
		filtered := make([]map[string]any, 0, len(resp.Items))
		for _, item := range resp.Items {
			if in.Host != "" && asString(item["host"]) != in.Host {
				continue
			}
			filtered = append(filtered, item)
		}
		return mcpJSON(map[string]any{key: filtered, "hostErrors": resp.HostErrors})
	})
	register(tool)
}

// execResult is the structured result of a run_command call. Output merges
// stdout and stderr because the exec runs on a pseudo-terminal.
type execResult struct {
	Command  string `json:"command"`
	ExitCode int    `json:"exitCode"`
	Output   string `json:"output"`
	TimedOut bool   `json:"timedOut,omitempty"`
}

func registerRunCommand(s *mcp.Server, a *app, register func(*mcp.Tool)) {
	type runCommandInput struct {
		Container      string `json:"container" jsonschema:"container name or ID"`
		Host           string `json:"host,omitempty" jsonschema:"host name (disambiguates duplicate names)"`
		Command        string `json:"command" jsonschema:"shell command to run, executed with the container's default shell"`
		TimeoutSeconds int    `json:"timeout_seconds,omitempty" jsonschema:"max seconds to wait (default 15, max 60)"`
	}
	tool := &mcp.Tool{Name: "run_command", Description: "Run a one-shot, non-interactive command in a container and return combined output and exit code. Output merges stdout and stderr (the exec uses a pseudo-terminal).", Annotations: destructiveAnnot()}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in runCommandInput) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(in.Command) == "" {
			return nil, nil, fmt.Errorf("command is required")
		}
		container, err := a.resolve(ctx, in.Container, in.Host)
		if err != nil {
			return nil, nil, err
		}
		result, err := a.runExec(ctx, container, in.Command, time.Duration(clampInt(in.TimeoutSeconds, 15, 60))*time.Second)
		if err != nil {
			return nil, nil, err
		}
		return mcpJSON(result)
	})
	register(tool)
}

// runExec runs command in container over the existing exec WebSocket endpoint.
//
// The server only exposes an interactive PTY terminal, not a one-shot exec, so
// this drives that terminal: it wraps the command between unique markers and a
// "printf $?" trailer, reads until the end marker appears, and slices the pure
// command output from between them. Because the session is a PTY, stdout and
// stderr are merged and the very first echoed input line is discarded via the
// marker boundary.
func (a *app) runExec(ctx context.Context, container containerInfo, command string, timeout time.Duration) (execResult, error) {
	endpoint, err := execWebSocketURL(a.conn.url, container.ID, container.Host)
	if err != nil {
		return execResult{}, err
	}

	header := http.Header{}
	if a.conn.token != "" {
		header.Set("Authorization", "Bearer "+a.conn.token)
	}

	dialCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	conn, resp, err := websocket.DefaultDialer.DialContext(dialCtx, endpoint, header)
	if err != nil {
		if resp != nil {
			return execResult{}, responseError(resp)
		}
		return execResult{}, fmt.Errorf("cannot open exec session on %s: %v", a.conn.url, err)
	}
	defer conn.Close()

	nonce := execNonce()
	begin := "__LDKB_" + nonce + "__"
	endRe := regexp.MustCompile("__LDKE_" + nonce + "__([0-9]+)__")

	// The markers reference $? so the real end marker carries the exit code;
	// the echoed input line carries a literal %d and never matches endRe.
	line := fmt.Sprintf("printf '\\n%s\\n'; { %s ; }; printf '\\n__LDKE_%s__%%d__\\n' \"$?\"\n", begin, command, nonce)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(line)); err != nil {
		return execResult{}, err
	}

	deadline := time.Now().Add(timeout)
	_ = conn.SetReadDeadline(deadline)

	var buf strings.Builder
	result := execResult{Command: command, ExitCode: -1}
	for {
		_, data, readErr := conn.ReadMessage()
		if len(data) > 0 {
			buf.WriteString(string(data))
			if m := endRe.FindStringSubmatch(buf.String()); m != nil {
				result.ExitCode, _ = strconv.Atoi(m[1])
				break
			}
		}
		if readErr != nil {
			if ne, ok := readErr.(net.Error); ok && ne.Timeout() {
				result.TimedOut = true
			}
			break
		}
	}

	result.Output = sliceExecOutput(buf.String(), begin, endRe)
	return result, nil
}

// sliceExecOutput extracts command output from the raw PTY stream: everything
// after the last real begin marker and before the end marker, with carriage
// returns stripped.
func sliceExecOutput(raw, begin string, endRe *regexp.Regexp) string {
	region := raw
	if loc := endRe.FindStringIndex(region); loc != nil {
		region = region[:loc[0]]
	}
	if i := strings.LastIndex(region, begin); i >= 0 {
		region = region[i+len(begin):]
	}
	region = strings.ReplaceAll(region, "\r", "")
	return strings.Trim(region, "\n")
}

// execWebSocketURL builds the exec terminal WebSocket URL from the API base.
func execWebSocketURL(base, id, host string) (string, error) {
	u, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	default:
		u.Scheme = "ws"
	}
	u.Path += "/api/v1/containers/" + id + "/exec"
	u.RawQuery = url.Values{"host": {host}}.Encode()
	return u.String(), nil
}

func execNonce() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b[:])
}

// mcpJSON packs a value as pretty-printed JSON text content.
func mcpJSON(v any) (*mcp.CallToolResult, any, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(data)}}}, nil, nil
}

func readOnlyAnnot() *mcp.ToolAnnotations    { return &mcp.ToolAnnotations{ReadOnlyHint: true} }
func lifecycleAnnot() *mcp.ToolAnnotations   { return &mcp.ToolAnnotations{DestructiveHint: boolPtr(false)} }
func destructiveAnnot() *mcp.ToolAnnotations { return &mcp.ToolAnnotations{DestructiveHint: boolPtr(true)} }

func boolPtr(b bool) *bool { return &b }

func asString(v any) string {
	s, _ := v.(string)
	return s
}

// clampTail defaults a tail/limit to 100 and caps it at mcpMaxTail.
func clampTail(v int) int { return clampInt(v, 100, mcpMaxTail) }

func clampInt(v, def, max int) int {
	if v <= 0 {
		return def
	}
	if v > max {
		return max
	}
	return v
}
