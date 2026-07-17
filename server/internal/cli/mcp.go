package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

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
	env         bool // get_env, set_env
	settings    bool // settings reads and writes
}

const mcpLong = `Run a Model Context Protocol server over stdio so an AI assistant can
query and manage LogDeck through the same HTTP API the CLI uses.

Read and lifecycle (start/stop/restart) tools are always registered. Sensitive
tools are opt-in:
  --allow-destructive   register remove_container
  --allow-exec          register run_command (one-shot exec in a container)
  --allow-env           register get_env / set_env (env vars hold secrets, and
                        writing them recreates the container)
  --allow-settings      register settings reads and writes, including hosts,
                        authentication, and API tokens
  --allow-all           register all of the above

The token scope is the hard boundary and is enforced by the server: a read-only
API token is rejected on every mutation, and on env and settings entirely,
regardless of these flags. The flags only control which tools are advertised.`

func newMCPCmd(a *app) *cobra.Command {
	var allowDestructive, allowExec, allowEnv, allowSettings, allowAll bool

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run a Model Context Protocol server over stdio",
		Long:  mcpLong,
		Args:  cobra.NoArgs,
		RunE: a.run(func(cmd *cobra.Command, args []string) error {
			opts := mcpOptions{
				destructive: allowDestructive || allowAll,
				exec:        allowExec || allowAll,
				env:         allowEnv || allowAll,
				settings:    allowSettings || allowAll,
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
			if opts.env {
				tiers = append(tiers, "env")
			}
			if opts.settings {
				tiers = append(tiers, "settings")
			}
			fmt.Fprintf(os.Stderr, "MCP: %s enabled - server %s\n", strings.Join(tiers, " + "), a.conn.url)

			return server.Run(cmd.Context(), &mcp.StdioTransport{})
		}),
	}

	cmd.Flags().BoolVar(&allowDestructive, "allow-destructive", false, "register remove_container")
	cmd.Flags().BoolVar(&allowExec, "allow-exec", false, "register run_command (one-shot exec)")
	cmd.Flags().BoolVar(&allowEnv, "allow-env", false, "register get_env and set_env")
	cmd.Flags().BoolVar(&allowSettings, "allow-settings", false, "register settings reads and writes, including auth and API tokens")
	cmd.Flags().BoolVar(&allowAll, "allow-all", false, "register all action tools (destructive + exec + env + settings)")
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

	if opts.env {
		registerEnvTools(s, a, register)
	}

	if opts.settings {
		registerSettingsTools(s, a, register)
	}

	return names
}

// registerEnvTools registers container environment-variable access. The server
// denies /env to read-scoped tokens because the values are secrets, and a write
// recreates the container, so set_env is marked destructive.
func registerEnvTools(s *mcp.Server, a *app, register func(*mcp.Tool)) {
	tool := &mcp.Tool{Name: "get_env", Description: "Read a container's environment variables. These commonly hold secrets.", Annotations: readOnlyAnnot()}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in containerRef) (*mcp.CallToolResult, any, error) {
		container, err := a.resolve(ctx, in.Container, in.Host)
		if err != nil {
			return nil, nil, err
		}
		var resp struct {
			Env map[string]string `json:"env"`
		}
		if err := a.client.get(ctx, "/containers/"+container.ID+"/env", url.Values{"host": {container.Host}}, &resp); err != nil {
			return nil, nil, err
		}
		return mcpJSON(resp)
	})
	register(tool)

	type setEnvInput struct {
		Container string            `json:"container" jsonschema:"container name or ID"`
		Host      string            `json:"host,omitempty" jsonschema:"host name (disambiguates duplicate names)"`
		Env       map[string]string `json:"env" jsonschema:"the complete variable map to apply; it replaces the container's existing environment, so read it with get_env first"`
	}
	tool = &mcp.Tool{Name: "set_env", Description: "Replace a container's environment variables. The container is recreated to apply them, so it restarts and gets a new ID.", Annotations: destructiveAnnot()}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in setEnvInput) (*mcp.CallToolResult, any, error) {
		if len(in.Env) == 0 {
			return nil, nil, fmt.Errorf("env is required and must be the complete variable map")
		}
		container, err := a.resolve(ctx, in.Container, in.Host)
		if err != nil {
			return nil, nil, err
		}
		var resp map[string]any
		if err := a.client.put(ctx, "/containers/"+container.ID+"/env", url.Values{"host": {container.Host}}, map[string]any{"env": in.Env}, &resp); err != nil {
			return nil, nil, err
		}
		return mcpJSON(resp)
	})
	register(tool)
}

// registerSettingsTools registers the LogDeck settings surface. The server
// denies the whole /settings group to read-scoped tokens: it exposes host
// topology, the token inventory, and the auth configuration.
func registerSettingsTools(s *mcp.Server, a *app, register func(*mcp.Tool)) {
	tool := &mcp.Tool{Name: "get_settings", Description: "Read LogDeck settings: Docker and Coolify hosts, auth state, read-only mode, and log-storage retention, each with the source it came from.", Annotations: readOnlyAnnot()}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		return getJSON(ctx, a, "/settings", nil)
	})
	register(tool)

	type readOnlyInput struct {
		Value bool `json:"value" jsonschema:"true blocks every mutating request server-wide"`
	}
	tool = &mcp.Tool{Name: "set_read_only", Description: "Turn LogDeck's server-wide read-only mode on or off.", Annotations: destructiveAnnot()}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in readOnlyInput) (*mcp.CallToolResult, any, error) {
		return putJSON(ctx, a, "/settings/read-only", map[string]any{"value": in.Value})
	})
	register(tool)

	type logStorageInput struct {
		Enabled        *bool `json:"enabled,omitempty" jsonschema:"turn log persistence on or off"`
		PerContainerMB *int  `json:"perContainerMB,omitempty" jsonschema:"per-container retention cap in MB"`
		TotalMB        *int  `json:"totalMB,omitempty" jsonschema:"total retention cap in MB across all containers"`
	}
	tool = &mcp.Tool{Name: "set_log_storage", Description: "Update log persistence: enable or disable it, or change the retention caps. Omitted fields are left unchanged. Lowering a cap makes the next sweep evict stored logs.", Annotations: destructiveAnnot()}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in logStorageInput) (*mcp.CallToolResult, any, error) {
		body := map[string]any{}
		if in.Enabled != nil {
			body["enabled"] = *in.Enabled
		}
		if in.PerContainerMB != nil {
			body["perContainerMB"] = *in.PerContainerMB
		}
		if in.TotalMB != nil {
			body["totalMB"] = *in.TotalMB
		}
		if len(body) == 0 {
			return nil, nil, fmt.Errorf("set at least one of enabled, perContainerMB, or totalMB")
		}
		return putJSON(ctx, a, "/settings/log-storage", body)
	})
	register(tool)

	type dockerHost struct {
		Name string `json:"name" jsonschema:"host label shown in the UI"`
		Host string `json:"host" jsonschema:"engine address, e.g. unix:///var/run/docker.sock or ssh://user@box"`
	}
	type dockerHostsInput struct {
		Hosts []dockerHost `json:"hosts" jsonschema:"the complete host list; it replaces the configured hosts, so read get_settings first. Hosts pinned by environment variables are rejected"`
	}
	tool = &mcp.Tool{Name: "set_docker_hosts", Description: "Replace the configured Docker/Podman hosts. This is the whole list, not a merge.", Annotations: destructiveAnnot()}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in dockerHostsInput) (*mcp.CallToolResult, any, error) {
		return putJSON(ctx, a, "/settings/docker-hosts", map[string]any{"hosts": in.Hosts})
	})
	register(tool)

	type coolifyHost struct {
		HostName string `json:"hostName" jsonschema:"the Docker host name this Coolify config belongs to"`
		APIURL   string `json:"apiURL" jsonschema:"Coolify API base URL"`
		APIToken string `json:"apiToken" jsonschema:"Coolify API token"`
	}
	type coolifyHostsInput struct {
		Hosts []coolifyHost `json:"hosts" jsonschema:"the complete Coolify host list; it replaces the configured entries"`
	}
	tool = &mcp.Tool{Name: "set_coolify_hosts", Description: "Replace the Coolify host configuration. This is the whole list, not a merge.", Annotations: destructiveAnnot()}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in coolifyHostsInput) (*mcp.CallToolResult, any, error) {
		return putJSON(ctx, a, "/settings/coolify-hosts", map[string]any{"hosts": in.Hosts})
	})
	register(tool)

	type authInput struct {
		Enabled       bool   `json:"enabled" jsonschema:"false disables login for the whole server"`
		AdminUsername string `json:"adminUsername" jsonschema:"admin username"`
		NewPassword   string `json:"newPassword,omitempty" jsonschema:"set a new admin password; omit to keep the current one"`
	}
	tool = &mcp.Tool{Name: "set_auth", Description: "Change LogDeck's authentication: enable or disable login, set the admin username, or set a new password. Disabling it leaves the server open to anyone who can reach it.", Annotations: destructiveAnnot()}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in authInput) (*mcp.CallToolResult, any, error) {
		body := map[string]any{"enabled": in.Enabled, "adminUsername": in.AdminUsername}
		if in.NewPassword != "" {
			body["newPassword"] = in.NewPassword
		}
		return putJSON(ctx, a, "/settings/auth", body)
	})
	register(tool)

	tool = &mcp.Tool{Name: "list_api_tokens", Description: "List API tokens with their name, scope, and prefix. The secrets themselves are never stored and cannot be read back.", Annotations: readOnlyAnnot()}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		return getJSON(ctx, a, "/settings/api-tokens", nil)
	})
	register(tool)

	type createTokenInput struct {
		Name  string `json:"name" jsonschema:"a label for the token"`
		Scope string `json:"scope,omitempty" jsonschema:"admin (full access) or read (read-only); defaults to admin"`
	}
	tool = &mcp.Tool{Name: "create_api_token", Description: "Create an API token. The secret is returned once and cannot be retrieved again.", Annotations: destructiveAnnot()}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in createTokenInput) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(in.Name) == "" {
			return nil, nil, fmt.Errorf("name is required")
		}
		body := map[string]any{"name": in.Name}
		if in.Scope != "" {
			body["scope"] = in.Scope
		}
		var resp map[string]any
		if err := a.client.post(ctx, "/settings/api-tokens", nil, body, &resp); err != nil {
			return nil, nil, err
		}
		return mcpJSON(resp)
	})
	register(tool)

	type deleteTokenInput struct {
		Prefix string `json:"prefix" jsonschema:"the token's prefix, as shown by list_api_tokens"`
	}
	tool = &mcp.Tool{Name: "delete_api_token", Description: "Revoke an API token by its prefix. Revocation takes effect immediately.", Annotations: destructiveAnnot()}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in deleteTokenInput) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(in.Prefix) == "" {
			return nil, nil, fmt.Errorf("prefix is required")
		}
		if err := a.client.do(ctx, http.MethodDelete, "/settings/api-tokens/"+url.PathEscape(in.Prefix), nil, nil, nil); err != nil {
			return nil, nil, err
		}
		return mcpJSON(map[string]string{"message": "token revoked", "prefix": in.Prefix})
	})
	register(tool)
}

// getJSON and putJSON decode an endpoint's JSON body straight into a tool
// result, for the settings tools whose shapes the CLI does not model.
func getJSON(ctx context.Context, a *app, path string, query url.Values) (*mcp.CallToolResult, any, error) {
	var resp map[string]any
	if err := a.client.get(ctx, path, query, &resp); err != nil {
		return nil, nil, err
	}
	return mcpJSON(resp)
}

func putJSON(ctx context.Context, a *app, path string, body any) (*mcp.CallToolResult, any, error) {
	var resp map[string]any
	if err := a.client.put(ctx, path, nil, body, &resp); err != nil {
		return nil, nil, err
	}
	return mcpJSON(resp)
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

// execResult is the structured result of a run_command call.
type execResult struct {
	Command  string `json:"command"`
	ExitCode int    `json:"exitCode"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

func registerRunCommand(s *mcp.Server, a *app, register func(*mcp.Tool)) {
	type runCommandInput struct {
		Container string `json:"container" jsonschema:"container name or ID"`
		Host      string `json:"host,omitempty" jsonschema:"host name (disambiguates duplicate names)"`
		Command   string `json:"command" jsonschema:"shell command to run, executed with /bin/sh -c"`
	}
	tool := &mcp.Tool{Name: "run_command", Description: "Run a one-shot, non-interactive command in a container and return separate stdout, stderr, and the exit code.", Annotations: destructiveAnnot()}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in runCommandInput) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(in.Command) == "" {
			return nil, nil, fmt.Errorf("command is required")
		}
		container, err := a.resolve(ctx, in.Container, in.Host)
		if err != nil {
			return nil, nil, err
		}
		var resp struct {
			Stdout   string `json:"stdout"`
			Stderr   string `json:"stderr"`
			ExitCode int    `json:"exitCode"`
		}
		body := map[string]string{"command": in.Command}
		if err := a.client.post(ctx, "/containers/"+container.ID+"/exec/run", url.Values{"host": {container.Host}}, body, &resp); err != nil {
			return nil, nil, err
		}
		return mcpJSON(execResult{Command: in.Command, ExitCode: resp.ExitCode, Stdout: resp.Stdout, Stderr: resp.Stderr})
	})
	register(tool)
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

func clampInt(v, def, maximum int) int {
	if v <= 0 {
		return def
	}
	if v > maximum {
		return maximum
	}
	return v
}
