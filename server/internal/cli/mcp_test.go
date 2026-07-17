package cli

import (
	"sort"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// readAndLifecycleTools are registered regardless of the action flags. A read
// token still cannot mutate — the server enforces that — but the lifecycle
// tools are always advertised.
var readAndLifecycleTools = []string{
	"list_containers", "get_logs", "search_logs", "inspect_container",
	"list_events", "container_stats", "host_stats",
	"list_images", "list_volumes", "list_networks",
	"history_search", "history_status", "history_containers",
	"start_container", "stop_container", "restart_container",
}

func registeredTools(t *testing.T, opts mcpOptions) []string {
	t.Helper()
	s := mcp.NewServer(&mcp.Implementation{Name: "logdeck", Version: "test"}, nil)
	names := registerMCPTools(s, &app{}, opts)
	sort.Strings(names)
	return names
}

func TestMCPToolGating(t *testing.T) {
	tests := []struct {
		name  string
		opts  mcpOptions
		extra []string
	}{
		{name: "default", opts: mcpOptions{}},
		{name: "destructive", opts: mcpOptions{destructive: true}, extra: []string{"remove_container"}},
		{name: "exec", opts: mcpOptions{exec: true}, extra: []string{"run_command"}},
		{name: "all", opts: mcpOptions{destructive: true, exec: true}, extra: []string{"remove_container", "run_command"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			want := append(append([]string{}, readAndLifecycleTools...), tc.extra...)
			sort.Strings(want)

			got := registeredTools(t, tc.opts)

			if len(got) != len(want) {
				t.Fatalf("registered %d tools, want %d\n got:  %v\n want: %v", len(got), len(want), got, want)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("tool set mismatch\n got:  %v\n want: %v", got, want)
				}
			}
		})
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

// TestMCPActionToolsGatedOff confirms the sensitive tools are absent unless
// their flag is set — the registration surface is the advertised surface.
func TestMCPActionToolsGatedOff(t *testing.T) {
	got := registeredTools(t, mcpOptions{})
	for _, name := range []string{"remove_container", "run_command"} {
		for _, have := range got {
			if have == name {
				t.Errorf("%s must not be registered without its flag", name)
			}
		}
	}
}
