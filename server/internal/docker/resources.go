package docker

import (
	"context"
	"strings"

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
	byHost, hostErrors := eachHost(ctx, c, func(ctx context.Context, name string, cl *client.Client) ([]models.ImageInfo, error) {
		images, err := cl.ImageList(ctx, image.ListOptions{})
		if err != nil {
			return nil, err
		}
		infos := make([]models.ImageInfo, 0, len(images))
		for _, img := range images {
			infos = append(infos, models.ImageInfo{
				ID:       shortID(img.ID),
				RepoTags: img.RepoTags,
				Size:     img.Size,
				Created:  img.Created,
				Host:     name,
			})
		}
		return infos, nil
	})

	result := []models.ImageInfo{}
	for _, infos := range byHost {
		result = append(result, infos...)
	}
	return result, hostErrors
}

func (c *MultiHostClient) ListVolumesAllHosts(ctx context.Context) ([]models.VolumeInfo, []HostError) {
	byHost, hostErrors := eachHost(ctx, c, func(ctx context.Context, name string, cl *client.Client) ([]models.VolumeInfo, error) {
		volumes, err := cl.VolumeList(ctx, volume.ListOptions{})
		if err != nil {
			return nil, err
		}
		infos := make([]models.VolumeInfo, 0, len(volumes.Volumes))
		for _, vol := range volumes.Volumes {
			if vol == nil {
				continue
			}
			infos = append(infos, models.VolumeInfo{
				Name:       vol.Name,
				Driver:     vol.Driver,
				Mountpoint: vol.Mountpoint,
				Created:    vol.CreatedAt,
				Labels:     vol.Labels,
				Host:       name,
			})
		}
		return infos, nil
	})

	result := []models.VolumeInfo{}
	for _, infos := range byHost {
		result = append(result, infos...)
	}
	return result, hostErrors
}

func (c *MultiHostClient) ListNetworksAllHosts(ctx context.Context) ([]models.NetworkInfo, []HostError) {
	byHost, hostErrors := eachHost(ctx, c, func(ctx context.Context, name string, cl *client.Client) ([]models.NetworkInfo, error) {
		networks, err := cl.NetworkList(ctx, network.ListOptions{})
		if err != nil {
			return nil, err
		}
		infos := make([]models.NetworkInfo, 0, len(networks))
		for _, nw := range networks {
			var subnets []string
			for _, cfg := range nw.IPAM.Config {
				if cfg.Subnet != "" {
					subnets = append(subnets, cfg.Subnet)
				}
			}
			infos = append(infos, models.NetworkInfo{
				ID:      shortID(nw.ID),
				Name:    nw.Name,
				Driver:  nw.Driver,
				Scope:   nw.Scope,
				Subnets: subnets,
				Host:    name,
			})
		}
		return infos, nil
	})

	result := []models.NetworkInfo{}
	for _, infos := range byHost {
		result = append(result, infos...)
	}
	return result, hostErrors
}
