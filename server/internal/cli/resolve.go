package cli

import (
	"context"
	"fmt"
	"strings"
)

// containerName returns the primary display name (leading / stripped).
func containerName(c containerInfo) string {
	if len(c.Names) == 0 {
		return shortID(c.ID)
	}
	return strings.TrimPrefix(c.Names[0], "/")
}

func nameMatches(c containerInfo, query string) bool {
	for _, name := range c.Names {
		if strings.TrimPrefix(name, "/") == query {
			return true
		}
	}
	return false
}

// resolveContainer matches query against a container list: exact name first
// (leading / stripped), then ID prefix. host filters candidates when set.
func resolveContainer(containers []containerInfo, query, host string) (containerInfo, error) {
	var candidates []containerInfo
	for _, c := range containers {
		if host != "" && c.Host != host {
			continue
		}
		if nameMatches(c, query) {
			candidates = append(candidates, c)
		}
	}
	if len(candidates) == 0 {
		for _, c := range containers {
			if host != "" && c.Host != host {
				continue
			}
			if strings.HasPrefix(c.ID, query) {
				candidates = append(candidates, c)
			}
		}
	}

	switch len(candidates) {
	case 1:
		return candidates[0], nil
	case 0:
		if host != "" {
			return containerInfo{}, fmt.Errorf("no container matches %q on host %q", query, host)
		}
		return containerInfo{}, fmt.Errorf("no container matches %q", query)
	}

	lines := make([]string, 0, len(candidates))
	for _, c := range candidates {
		lines = append(lines, fmt.Sprintf("  %s (host %s, id %s)", containerName(c), c.Host, shortID(c.ID)))
	}
	return containerInfo{}, fmt.Errorf("%q is ambiguous, candidates:\n%s\nuse --host to disambiguate", query, strings.Join(lines, "\n"))
}

// fetchContainers loads the full container list once; commands resolve names
// and derive the required ?host= parameter from it.
func (a *app) fetchContainers(ctx context.Context) (containersResponse, error) {
	var resp containersResponse
	err := a.client.get(ctx, "/containers", nil, &resp)
	return resp, err
}

// resolve fetches the container list and resolves query within it.
func (a *app) resolve(ctx context.Context, query, host string) (containerInfo, error) {
	resp, err := a.fetchContainers(ctx)
	if err != nil {
		return containerInfo{}, err
	}
	return resolveContainer(resp.Containers, query, host)
}
