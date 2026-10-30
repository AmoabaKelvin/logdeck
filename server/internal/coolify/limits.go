package coolify

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"

	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/docker/go-units"
)

// application holds the saved settings Coolify turns into the container's
// mem_limit, memswap_limit and cpus on each deployment. Coolify stores them
// as Docker strings such as "512m" and "1.5", with "0" for unlimited.
type application struct {
	BuildPack  string `json:"build_pack"`
	Memory     string `json:"limits_memory"`
	MemorySwap string `json:"limits_memory_swap"`
	CPUs       string `json:"limits_cpus"`
	limits     models.ResourceLimits
	swap       int64
}

func (c *Client) applicationURL(resource *ResourceInfo) string {
	return fmt.Sprintf("%s/api/v1/applications/%s", c.apiURL, url.PathEscape(resource.UUID))
}

func (c *Client) readApplication(ctx context.Context, resource *ResourceInfo) (application, error) {
	var app application
	if resource == nil || resource.Type != ResourceTypeApplication || resource.UUID == "" {
		return app, fmt.Errorf("only Coolify application limits can be saved here; change this resource's limits in Coolify")
	}
	body, err := c.doRequest(ctx, http.MethodGet, c.applicationURL(resource), nil)
	if err != nil {
		return app, fmt.Errorf("could not read Coolify application: %w", err)
	}
	if err := json.Unmarshal(body, &app); err != nil {
		return app, fmt.Errorf("coolify returned an unreadable application: %w", err)
	}
	// Compose applications take limits from their Compose file, not these fields.
	if app.BuildPack == "dockercompose" {
		return app, fmt.Errorf("this Coolify application is deployed from a Compose file, which sets its limits; change them in Coolify")
	}
	memory, errMemory := dockerBytes(app.Memory)
	swap, errSwap := dockerBytes(app.MemorySwap)
	cpus, errCPUs := strconv.ParseFloat(orZero(app.CPUs), 64)
	if errMemory != nil || errSwap != nil || errCPUs != nil || memory < 0 || cpus < 0 {
		return app, fmt.Errorf("coolify has limits LogDeck cannot read (memory %q, swap %q, CPUs %q); change them in Coolify", app.Memory, app.MemorySwap, app.CPUs)
	}
	app.limits = models.ResourceLimits{MemoryBytes: memory, NanoCPUs: int64(math.Round(cpus * 1e9))}
	app.swap = swap
	return app, nil
}

// ReadLimits returns the limits Coolify applies on the application's next deployment.
func (c *Client) ReadLimits(ctx context.Context, resource *ResourceInfo) (models.ResourceLimits, error) {
	app, err := c.readApplication(ctx, resource)
	return app.limits, err
}

// SaveLimits changes the saved limits only, keeping any the request omits.
// The running container keeps its limits until the application is deployed.
func (c *Client) SaveLimits(ctx context.Context, resource *ResourceInfo, req models.UpdateResourcesRequest) error {
	app, err := c.readApplication(ctx, resource)
	if err != nil {
		return err
	}
	limits := app.limits.With(req)
	// Docker refuses a swap limit without a memory limit or below it, which
	// would only surface as a failed deployment.
	if app.swap > 0 && (limits.MemoryBytes == 0 || limits.MemoryBytes > app.swap) {
		return fmt.Errorf("coolify's swap limit (%s) must not be below the memory limit; change both in Coolify", app.MemorySwap)
	}
	body, err := json.Marshal(map[string]string{
		"limits_memory": formatBytes(limits.MemoryBytes),
		"limits_cpus":   strconv.FormatFloat(float64(limits.NanoCPUs)/1e9, 'f', -1, 64),
	})
	if err != nil {
		return err
	}
	if _, err := c.doRequest(ctx, http.MethodPatch, c.applicationURL(resource), body); err != nil {
		return fmt.Errorf("coolify save could not be confirmed; reload to check the saved limits: %w", err)
	}
	return nil
}

func orZero(value string) string {
	if value == "" {
		return "0"
	}
	return value
}

// dockerBytes reads a Coolify memory setting. Swap also allows -1, unlimited.
func dockerBytes(value string) (int64, error) {
	if value == "-1" {
		return -1, nil
	}
	return units.RAMInBytes(orZero(value))
}

// formatBytes writes bytes in the largest exact unit, the way Coolify's own
// form expects them, e.g. "512m".
func formatBytes(bytes int64) string {
	for _, unit := range []struct {
		suffix string
		size   int64
	}{{"g", 1 << 30}, {"m", 1 << 20}, {"k", 1 << 10}} {
		if bytes > 0 && bytes%unit.size == 0 {
			return strconv.FormatInt(bytes/unit.size, 10) + unit.suffix
		}
	}
	return strconv.FormatInt(bytes, 10)
}
