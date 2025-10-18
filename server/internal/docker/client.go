package docker

import (
	"context"

	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type Client struct {
	apiClient *client.Client
}

func NewClient() (*Client, error) {
	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}
	return &Client{apiClient: apiClient}, nil
}

func (c *Client) ListContainers() ([]models.ContainerInfo, error) {
	containers, err := c.apiClient.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	result := make([]models.ContainerInfo, 0, len(containers))
	for _, ctr := range containers {
		result = append(result, models.ContainerInfo{
			ID:      ctr.ID,
			Names:   ctr.Names,
			Image:   ctr.Image,
			ImageID: ctr.ImageID,
			Command: ctr.Command,
			Created: ctr.Created,
			State:   ctr.State,
			Status:  ctr.Status,
			Labels:  ctr.Labels,
		})
	}

	return result, nil
}

func (c *Client) GetContainer(id string) (container.InspectResponse, error) {
	result, err := c.apiClient.ContainerInspect(context.Background(), id)
	if err != nil {
		return container.InspectResponse{}, err
	}
	return result, nil
}

// container life cycle methods, start, stop, restart, remove

func (c *Client) StartContainer(id string) error {
	err := c.apiClient.ContainerStart(context.Background(), id, container.StartOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) StopContainer(id string) error {
	err := c.apiClient.ContainerStop(context.Background(), id, container.StopOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) RestartContainer(id string) error {
	err := c.apiClient.ContainerRestart(context.Background(), id, container.StopOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) RemoveContainer(id string) error {
	err := c.apiClient.ContainerRemove(context.Background(), id, container.RemoveOptions{})
	if err != nil {
		return err
	}
	return nil
}
