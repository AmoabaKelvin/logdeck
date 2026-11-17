package docker

import (
	"context"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
)

// ServiceUpdateStatus is the state of a Swarm service's latest rolling
// update, or nil when the service has never been updated.
func (c *MultiHostClient) ServiceUpdateStatus(ctx context.Context, hostName, service string) (*swarm.UpdateStatus, error) {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return nil, err
	}
	svc, _, err := apiClient.ServiceInspectWithRaw(ctx, service, swarm.ServiceInspectOptions{})
	if err != nil {
		return nil, err
	}
	return svc.UpdateStatus, nil
}

// NewestRunningContainer returns the ID of the most recently created running
// container carrying every label, or "" when none runs.
func (c *MultiHostClient) NewestRunningContainer(ctx context.Context, hostName string, labels map[string]string) (string, error) {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return "", err
	}
	args := filters.NewArgs(filters.Arg("status", "running"))
	for key, value := range labels {
		args.Add("label", key+"="+value)
	}
	containers, err := apiClient.ContainerList(ctx, container.ListOptions{Filters: args})
	if err != nil {
		return "", err
	}
	newest := ""
	var created int64
	for _, ctr := range containers {
		if newest == "" || ctr.Created > created {
			newest, created = ctr.ID, ctr.Created
		}
	}
	return newest, nil
}
