package docker

import (
	"testing"

	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/docker/docker/api/types/container"
)

// buildLogsOptions must map every LogOptions field onto the SDK options, and
// take Follow/Timestamps from its arguments (not from the struct) so the same
// options can drive both historical and follow requests.
func TestBuildLogsOptions(t *testing.T) {
	opts := models.LogOptions{
		Details:    true,
		Since:      "1700000000",
		Until:      "1700009999",
		Tail:       "100",
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true, // must be ignored; the follow argument wins
	}

	got := buildLogsOptions(opts, false, true)
	want := container.LogsOptions{
		Follow:     false,
		Timestamps: true,
		Details:    true,
		Since:      "1700000000",
		Until:      "1700009999",
		Tail:       "100",
		ShowStdout: true,
		ShowStderr: true,
	}
	if got != want {
		t.Fatalf("buildLogsOptions() = %+v, want %+v", got, want)
	}

	// The follow and timestamps flags come strictly from the arguments.
	if follow := buildLogsOptions(opts, true, false); !follow.Follow || follow.Timestamps {
		t.Fatalf("expected follow=true timestamps=false from args, got %+v", follow)
	}
}
