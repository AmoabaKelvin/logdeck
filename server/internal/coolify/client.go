package coolify

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/config"
)

type ResourceType string

const (
	ResourceTypeApplication ResourceType = "application"
	ResourceTypeService     ResourceType = "service"
	ResourceTypeDatabase    ResourceType = "database"

	// LabelManaged is the Docker label Coolify sets on containers it manages.
	LabelManaged = "coolify.managed"
)

type ResourceInfo struct {
	Type ResourceType
	UUID string
}

// IsCoolifyDefaultEnvVar reports whether key is a Coolify-injected environment
// variable that users should not see or modify. These are set automatically by
// Coolify at deploy time (see https://coolify.io/docs/knowledge-base/environment-variables).
func IsCoolifyDefaultEnvVar(key string) bool {
	return strings.HasPrefix(key, "COOLIFY_") || key == "SOURCE_COMMIT"
}

type Client struct {
	apiURL     string
	apiToken   string
	httpClient *http.Client
}

func newClient(apiURL, apiToken string) *Client {
	return &Client{
		apiURL:     apiURL,
		apiToken:   apiToken,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// MultiClient routes Coolify API calls to the correct per-host client.
type MultiClient struct {
	clients map[string]*Client
}

// NewMultiClient creates a MultiClient from per-host configs.
// Returns nil if no configs are provided.
func NewMultiClient(hostConfigs []config.CoolifyHostConfig) *MultiClient {
	if len(hostConfigs) == 0 {
		return nil
	}

	clients := make(map[string]*Client, len(hostConfigs))
	for _, hc := range hostConfigs {
		clients[hc.HostName] = newClient(hc.APIURL, hc.APIToken)
	}

	return &MultiClient{clients: clients}
}

// GetClient returns the Coolify client for the given Docker host name.
// Returns nil if no config exists for that host.
func (mc *MultiClient) GetClient(hostName string) *Client {
	if mc == nil {
		return nil
	}
	return mc.clients[hostName]
}

// TestConnection checks if the Coolify API is reachable by calling /api/v1/version.
func (c *Client) TestConnection(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/v1/version", c.apiURL)
	_, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("coolify API unreachable: %w", err)
	}
	return nil
}

// NewSingleClient creates a single Coolify client for testing connections.
func NewSingleClient(apiURL, apiToken string) *Client {
	return newClient(apiURL, apiToken)
}

// ExtractResourceInfo checks container labels for Coolify management info.
// Returns nil if the container is not managed by Coolify.
//
// Coolify labels its containers with:
//   - coolify.managed=true
//   - coolify.type={application,service,database}
//   - com.docker.compose.project={uuid}  (the API-compatible UUID)
func ExtractResourceInfo(labels map[string]string) *ResourceInfo {
	if labels[LabelManaged] != "true" {
		return nil
	}

	uuid := labels["com.docker.compose.project"]
	if uuid == "" {
		return nil
	}

	switch labels["coolify.type"] {
	case "application":
		return &ResourceInfo{Type: ResourceTypeApplication, UUID: uuid}
	case "service":
		return &ResourceInfo{Type: ResourceTypeService, UUID: uuid}
	case "database":
		return &ResourceInfo{Type: ResourceTypeDatabase, UUID: uuid}
	default:
		return &ResourceInfo{Type: ResourceType(labels["coolify.type"]), UUID: uuid}
	}
}

func (c *Client) doRequest(ctx context.Context, method, url string, body []byte) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	return respBody, nil
}
