package dokploy

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/AmoabaKelvin/logdeck/internal/models"
)

// Dokploy stores an application's Swarm limits as decimal strings, memory in
// bytes and CPU in nano-CPUs, and treats an empty value as unlimited.
func limitValue(value *string) (int64, error) {
	if value == nil || *value == "" {
		return 0, nil
	}
	return strconv.ParseInt(*value, 10, 64)
}

func limitText(value int64) string {
	if value == 0 {
		return ""
	}
	return strconv.FormatInt(value, 10)
}

func (c *Client) readApplication(ctx context.Context, r Resource) (record, error) {
	if r.Type != Application {
		return record{}, fmt.Errorf("only Dokploy application limits can be saved here; Compose deployments set them in their Compose file")
	}
	return c.readRecord(ctx, r)
}

// ReadLimits returns the limits Dokploy applies on the application's next deployment.
func (c *Client) ReadLimits(ctx context.Context, r Resource) (models.ResourceLimits, error) {
	rec, err := c.readApplication(ctx, r)
	if err != nil {
		return models.ResourceLimits{}, err
	}
	memory, errMemory := limitValue(rec.MemoryLimit)
	cpus, errCPUs := limitValue(rec.CPULimit)
	if errMemory != nil || errCPUs != nil || memory < 0 || cpus < 0 {
		return models.ResourceLimits{}, fmt.Errorf("dokploy has limits LogDeck cannot read; change them in Dokploy")
	}
	return models.ResourceLimits{MemoryBytes: memory, NanoCPUs: cpus}, nil
}

// SaveLimits changes the saved limits only, keeping any the request omits.
// Running tasks keep their limits until the application is deployed.
func (c *Client) SaveLimits(ctx context.Context, r Resource, req models.UpdateResourcesRequest) error {
	saved, err := c.ReadLimits(ctx, r)
	if err != nil {
		return err
	}
	limits := saved.With(req)
	payload := map[string]string{"applicationId": r.ID, "memoryLimit": limitText(limits.MemoryBytes), "cpuLimit": limitText(limits.NanoCPUs)}
	var acknowledged bool
	if err := c.request(ctx, http.MethodPost, "application.update", payload, &acknowledged); err != nil {
		return err
	}
	if !acknowledged {
		return fmt.Errorf("dokploy did not acknowledge the save; reload before retrying")
	}
	return nil
}
