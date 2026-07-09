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
	calls      []string
	createErr  error
	startErrOn string // container ID whose start should fail
}

func (f *fakeRecreateAPI) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	f.calls = append(f.calls, "stop "+containerID)
	return nil
}

func (f *fakeRecreateAPI) ContainerRename(ctx context.Context, containerID, newContainerName string) error {
	f.calls = append(f.calls, "rename "+containerID+" "+newContainerName)
	return nil
}

func (f *fakeRecreateAPI) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
	f.calls = append(f.calls, "create "+containerName)
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
		})
	}
}
