package docker

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type fakeRecreateAPI struct {
	calls                []string
	stopErr              error
	renameErr            error
	createErr            error
	startErrOn           string                // container ID whose start should fail
	lastCreateEnv        []string              // env passed to the last ContainerCreate call
	lastCreateHostConfig *container.HostConfig // host config passed to the last ContainerCreate call
}

func (f *fakeRecreateAPI) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	f.calls = append(f.calls, "stop "+containerID)
	return f.stopErr
}

func (f *fakeRecreateAPI) ContainerRename(ctx context.Context, containerID, newContainerName string) error {
	f.calls = append(f.calls, "rename "+containerID+" "+newContainerName)
	return f.renameErr
}

func (f *fakeRecreateAPI) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
	f.calls = append(f.calls, "create "+containerName)
	f.lastCreateEnv = config.Env
	f.lastCreateHostConfig = hostConfig
	if f.createErr != nil {
		return container.CreateResponse{}, f.createErr
	}
	return container.CreateResponse{ID: "new-container-id"}, nil
}

func (f *fakeRecreateAPI) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	f.calls = append(f.calls, "start "+containerID)
	if containerID == f.startErrOn {
		return errors.New("start failed")
	}
	return nil
}

func (f *fakeRecreateAPI) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	f.calls = append(f.calls, fmt.Sprintf("remove %s force=%t", containerID, options.Force))
	return nil
}

func testInspectResponse(running bool) container.InspectResponse {
	return container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{
			ID:         "abcdef1234567890",
			Name:       "/web",
			State:      &container.State{Running: running},
			HostConfig: &container.HostConfig{},
		},
		Config: &container.Config{Env: []string{"A=1"}},
	}
}

func TestRecreateContainerWithEnv(t *testing.T) {
	const tempName = "web-logdeck-old-abcdef123456"

	tests := []struct {
		name      string
		api       *fakeRecreateAPI
		running   bool
		wantErr   bool
		wantID    string
		wantCalls []string
	}{
		{
			name:    "success removes renamed original only after new container starts",
			api:     &fakeRecreateAPI{},
			running: true,
			wantID:  "new-container-id",
			wantCalls: []string{
				"stop abcdef1234567890",
				"rename abcdef1234567890 " + tempName,
				"create web",
				"start new-container-id",
				"remove abcdef1234567890 force=false",
			},
		},
		{
			name:    "success on stopped container leaves replacement stopped",
			api:     &fakeRecreateAPI{},
			running: false,
			wantID:  "new-container-id",
			wantCalls: []string{
				"stop abcdef1234567890",
				"rename abcdef1234567890 " + tempName,
				"create web",
				"remove abcdef1234567890 force=false",
			},
		},
		{
			name:    "stop failure returns immediately with no rollback",
			api:     &fakeRecreateAPI{stopErr: errors.New("stop failed")},
			running: true,
			wantErr: true,
			wantCalls: []string{
				"stop abcdef1234567890",
			},
		},
		{
			name:    "rename failure restarts the running original",
			api:     &fakeRecreateAPI{renameErr: errors.New("rename failed")},
			running: true,
			wantErr: true,
			wantCalls: []string{
				"stop abcdef1234567890",
				"rename abcdef1234567890 " + tempName,
				"start abcdef1234567890",
			},
		},
		{
			name:    "create failure renames original back and restarts it",
			api:     &fakeRecreateAPI{createErr: errors.New("create failed")},
			running: true,
			wantErr: true,
			wantCalls: []string{
				"stop abcdef1234567890",
				"rename abcdef1234567890 " + tempName,
				"create web",
				"rename abcdef1234567890 web",
				"start abcdef1234567890",
			},
		},
		{
			name:    "start failure removes new container and restores original",
			api:     &fakeRecreateAPI{startErrOn: "new-container-id"},
			running: true,
			wantErr: true,
			wantCalls: []string{
				"stop abcdef1234567890",
				"rename abcdef1234567890 " + tempName,
				"create web",
				"start new-container-id",
				"remove new-container-id force=true",
				"rename abcdef1234567890 web",
				"start abcdef1234567890",
			},
		},
		{
			name:    "create failure on stopped container does not restart it",
			api:     &fakeRecreateAPI{createErr: errors.New("create failed")},
			running: false,
			wantErr: true,
			wantCalls: []string{
				"stop abcdef1234567890",
				"rename abcdef1234567890 " + tempName,
				"create web",
				"rename abcdef1234567890 web",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := recreateContainerWithEnv(context.Background(), tt.api, testInspectResponse(tt.running), []string{"A=2"})
			if (err != nil) != tt.wantErr {
				t.Fatalf("recreateContainerWithEnv() error = %v, wantErr %t", err, tt.wantErr)
			}
			if id != tt.wantID {
				t.Fatalf("recreateContainerWithEnv() id = %q, want %q", id, tt.wantID)
			}
			if !reflect.DeepEqual(tt.api.calls, tt.wantCalls) {
				t.Fatalf("call sequence mismatch:\ngot:  %v\nwant: %v", tt.api.calls, tt.wantCalls)
			}
			if tt.wantID != "" && !reflect.DeepEqual(tt.api.lastCreateEnv, []string{"A=2"}) {
				t.Fatalf("expected replacement created with env [A=2], got %v", tt.api.lastCreateEnv)
			}
		})
	}
}

// Podman reports a CPU limit as both NanoCpus and CpuQuota/CpuPeriod, but its
// create rejects a host config carrying both ("NanoCpus conflicts with
// CpuPeriod and CpuQuota"), which broke every env edit on a container whose CPU
// limit had been set. The replacement must carry the quota/period pair alone.
func TestRecreateContainerWithEnvDropsConflictingNanoCpus(t *testing.T) {
	inspect := testInspectResponse(true)
	inspect.HostConfig = &container.HostConfig{
		Resources: container.Resources{
			NanoCPUs:  500000000, // 0.5 CPU, the same limit the pair below expresses
			CPUQuota:  50000,
			CPUPeriod: 100000,
			Memory:    256 * 1024 * 1024,
		},
	}

	api := &fakeRecreateAPI{}
	if _, err := recreateContainerWithEnv(context.Background(), api, inspect, []string{"A=2"}); err != nil {
		t.Fatalf("recreateContainerWithEnv() error = %v", err)
	}

	got := api.lastCreateHostConfig
	if got == nil {
		t.Fatal("no host config passed to ContainerCreate")
	}
	if got.NanoCPUs != 0 {
		t.Errorf("NanoCpus = %d, want 0 (it conflicts with the quota/period pair)", got.NanoCPUs)
	}
	if got.CPUQuota != 50000 || got.CPUPeriod != 100000 {
		t.Errorf("quota/period = %d/%d, want 50000/100000 (the limit must survive)", got.CPUQuota, got.CPUPeriod)
	}
	if got.Memory != 256*1024*1024 {
		t.Errorf("Memory = %d, want it carried over unchanged", got.Memory)
	}
	// The caller's inspect must not be mutated.
	if inspect.HostConfig.NanoCPUs != 500000000 {
		t.Error("recreate mutated the caller's HostConfig")
	}
}

// A Docker container sets only NanoCpus, with no quota/period; that limit must
// be left alone.
func TestRecreateContainerWithEnvKeepsNanoCpusWhenUnambiguous(t *testing.T) {
	inspect := testInspectResponse(true)
	inspect.HostConfig = &container.HostConfig{
		Resources: container.Resources{NanoCPUs: 1000000000},
	}

	api := &fakeRecreateAPI{}
	if _, err := recreateContainerWithEnv(context.Background(), api, inspect, []string{"A=2"}); err != nil {
		t.Fatalf("recreateContainerWithEnv() error = %v", err)
	}
	if api.lastCreateHostConfig.NanoCPUs != 1000000000 {
		t.Errorf("NanoCpus = %d, want 1000000000 preserved", api.lastCreateHostConfig.NanoCPUs)
	}
}
