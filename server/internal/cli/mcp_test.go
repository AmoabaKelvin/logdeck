package cli

import (
	"sort"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// allTools is the complete surface. Nothing is gated client-side: the caller's
// token scope is what decides, and the server enforces it.
var allTools = []string{
	// reads
	"list_containers", "get_logs", "search_logs", "inspect_container",
	"list_events", "container_stats", "host_stats",
	"list_images", "list_volumes", "list_networks",
	"history_search", "history_status", "history_containers",
	// container actions
	"start_container", "stop_container", "restart_container",
	"remove_container", "run_command",
	// env
	"get_env", "set_env",
	// settings
	"get_settings", "set_read_only", "set_log_storage",
	"set_docker_hosts", "set_coolify_hosts", "set_auth",
	"list_api_tokens", "create_api_token", "delete_api_token",
}

func registeredTools(t *testing.T) []string {
	t.Helper()
	s := mcp.NewServer(&mcp.Implementation{Name: "logdeck", Version: "test"}, nil)
	names := registerMCPTools(s, &app{})
	sort.Strings(names)
	return names
}

func TestMCPRegistersEveryTool(t *testing.T) {
	want := append([]string{}, allTools...)
	sort.Strings(want)

	got := registeredTools(t)

	if len(got) != len(want) {
		t.Fatalf("registered %d tools, want %d\n got:  %v\n want: %v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tool set mismatch\n got:  %v\n want: %v", got, want)
		}
	}
}

// Tool names are a client-facing contract: a rename silently breaks anyone's
// saved prompts, so a change here should be deliberate.
func TestMCPToolNamesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, name := range registeredTools(t) {
		if seen[name] {
			t.Errorf("duplicate tool name registered: %s", name)
		}
		seen[name] = true
	}
}

func TestClampInt(t *testing.T) {
	tests := []struct {
		v, def, maximum, want int
	}{
		{0, 100, 500, 100},   // zero -> default
		{-5, 100, 500, 100},  // negative -> default
		{50, 100, 500, 50},   // in range passes through
		{500, 100, 500, 500}, // at max passes through
		{999, 100, 500, 500}, // over max -> clamped
	}
	for _, tt := range tests {
		if got := clampInt(tt.v, tt.def, tt.maximum); got != tt.want {
			t.Errorf("clampInt(%d, %d, %d) = %d, want %d", tt.v, tt.def, tt.maximum, got, tt.want)
		}
	}
}

func TestClampTail(t *testing.T) {
	// clampTail defaults to 100 and caps at mcpMaxTail (500).
	tests := []struct {
		in, want int
	}{
		{0, 100},
		{-1, 100},
		{250, 250},
		{500, 500},
		{5000, 500},
	}
	for _, tt := range tests {
		if got := clampTail(tt.in); got != tt.want {
			t.Errorf("clampTail(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

// The mcp command takes no flags: capability comes from the token, not from
// how the server was launched.
func TestMCPCommandHasNoCapabilityFlags(t *testing.T) {
	cmd := newMCPCmd(&app{})
	if f := cmd.Flags().Lookup("allow-all"); f != nil {
		t.Error("allow-all flag is back; the token scope is the boundary")
	}
	for _, name := range []string{"allow-destructive", "allow-exec", "allow-env", "allow-settings"} {
		if f := cmd.Flags().Lookup(name); f != nil {
			t.Errorf("%s flag is back; the token scope is the boundary", name)
		}
	}
}
