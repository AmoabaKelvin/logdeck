package system

import (
	"encoding/json"
	"os"
	"testing"
)

// TestStatsJSONContract locks the wire field names the frontend depends on.
func TestStatsJSONContract(t *testing.T) {
	s := SystemStats{
		HostInfo: HostInfo{
			Hostname:        "box",
			Platform:        "linux",
			PlatformVersion: "12",
			KernelVersion:   "6.1",
			Arch:            "arm64",
			Uptime:          3600,
		},
		Usage: Usage{
			CPUPercent:    12.5,
			MemoryPercent: 40.0,
			MemoryTotal:   1000,
			MemoryUsed:    400,
		},
	}

	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var got map[string]json.RawMessage
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if _, ok := got["hostInfo"]; !ok {
		t.Error("missing hostInfo key")
	}
	if _, ok := got["usage"]; !ok {
		t.Error("missing usage key")
	}

	var usage map[string]json.RawMessage
	if err := json.Unmarshal(got["usage"], &usage); err != nil {
		t.Fatalf("unmarshal usage error: %v", err)
	}
	for _, key := range []string{"cpuPercent", "memoryPercent", "memoryTotal", "memoryUsed"} {
		if _, ok := usage[key]; !ok {
			t.Errorf("usage missing %q key", key)
		}
	}
}

// TestInitNoHostProc verifies Init leaves HOST_PROC untouched when /host/proc
// is not mounted (the normal case outside a container). Skipped in the unlikely
// event the test host actually has /host/proc.
func TestInitNoHostProc(t *testing.T) {
	if _, err := os.Stat("/host/proc"); err == nil {
		t.Skip("/host/proc exists on this host; positive path not covered here")
	}

	orig, had := os.LookupEnv("HOST_PROC")
	os.Unsetenv("HOST_PROC")
	t.Cleanup(func() {
		if had {
			os.Setenv("HOST_PROC", orig)
		} else {
			os.Unsetenv("HOST_PROC")
		}
	})

	Init()

	if v, ok := os.LookupEnv("HOST_PROC"); ok {
		t.Errorf("Init set HOST_PROC=%q with no /host/proc present", v)
	}
}
