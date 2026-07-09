package docker

import (
	"context"
	"strings"
	"sync"

	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

// shortID trims the sha256: prefix and truncates to the familiar 12 chars.
func shortID(id string) string {
	id = strings.TrimPrefix(id, "sha256:")
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func (c *MultiHostClient) ListImagesAllHosts(ctx context.Context) ([]models.ImageInfo, []HostError) {
	result := []models.ImageInfo{}
	var hostErrors []HostError
	var mu sync.Mutex
	var wg sync.WaitGroup

	for hostName, apiClient := range c.clients {
		wg.Add(1)
		go func(name string, cl *client.Client) {
			defer wg.Done()

			images, err := cl.ImageList(ctx, image.ListOptions{})
			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				hostErrors = append(hostErrors, HostError{HostName: name, Err: err})
				return
			}

			for _, img := range images {
				result = append(result, models.ImageInfo{
					ID:       shortID(img.ID),
					RepoTags: img.RepoTags,
					Size:     img.Size,
					Created:  img.Created,
					Host:     name,
				})
			}
		}(hostName, apiClient)
	}

	wg.Wait()
	return result, hostErrors
}

func (c *MultiHostClient) ListVolumesAllHosts(ctx context.Context) ([]models.VolumeInfo, []HostError) {
	result := []models.VolumeInfo{}
	var hostErrors []HostError
	var mu sync.Mutex
	var wg sync.WaitGroup

	for hostName, apiClient := range c.clients {
		wg.Add(1)
		go func(name string, cl *client.Client) {
			defer wg.Done()

			volumes, err := cl.VolumeList(ctx, volume.ListOptions{})
			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				hostErrors = append(hostErrors, HostError{HostName: name, Err: err})
				return
			}

			for _, vol := range volumes.Volumes {
				if vol == nil {
					continue
				}
				result = append(result, models.VolumeInfo{
					Name:       vol.Name,
					Driver:     vol.Driver,
					Mountpoint: vol.Mountpoint,
					Created:    vol.CreatedAt,
					Labels:     vol.Labels,
					Host:       name,
				})
			}
		}(hostName, apiClient)
	}

	wg.Wait()
	return result, hostErrors
}

func (c *MultiHostClient) ListNetworksAllHosts(ctx context.Context) ([]models.NetworkInfo, []HostError) {
	result := []models.NetworkInfo{}
	var hostErrors []HostError
	var mu sync.Mutex
	var wg sync.WaitGroup

	for hostName, apiClient := range c.clients {
		wg.Add(1)
		go func(name string, cl *client.Client) {
			defer wg.Done()

			networks, err := cl.NetworkList(ctx, network.ListOptions{})
			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				hostErrors = append(hostErrors, HostError{HostName: name, Err: err})
				return
			}

			for _, nw := range networks {
				var subnets []string
				for _, cfg := range nw.IPAM.Config {
					if cfg.Subnet != "" {
						subnets = append(subnets, cfg.Subnet)
					}
				}
				result = append(result, models.NetworkInfo{
					ID:      shortID(nw.ID),
					Name:    nw.Name,
					Driver:  nw.Driver,
					Scope:   nw.Scope,
					Subnets: subnets,
					Host:    name,
				})
			}
		}(hostName, apiClient)
	}

	wg.Wait()
	return result, hostErrors
}
