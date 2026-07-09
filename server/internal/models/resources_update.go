package models

import "fmt"

// RestartPolicySpec describes a container restart policy.
type RestartPolicySpec struct {
	Name              string `json:"name"`
	MaximumRetryCount int    `json:"maximumRetryCount"`
}

// UpdateResourcesRequest carries resource-limit and restart-policy changes.
// Pointer fields: omitted means unchanged.
type UpdateResourcesRequest struct {
	MemoryBytes   *int64             `json:"memoryBytes,omitempty"`
	NanoCPUs      *int64             `json:"nanoCPUs,omitempty"`
	RestartPolicy *RestartPolicySpec `json:"restartPolicy,omitempty"`
}

// ContainerResources is the API response for a container's current limits.
type ContainerResources struct {
	MemoryBytes   int64             `json:"memoryBytes"`
	NanoCPUs      int64             `json:"nanoCPUs"`
	RestartPolicy RestartPolicySpec `json:"restartPolicy"`
}

var validRestartPolicyNames = map[string]bool{
	"no":             true,
	"always":         true,
	"unless-stopped": true,
	"on-failure":     true,
}

// Validate checks the request for values the Docker API would reject.
func (r UpdateResourcesRequest) Validate() error {
	if r.MemoryBytes != nil && *r.MemoryBytes < 0 {
		return fmt.Errorf("memoryBytes must be >= 0")
	}
	if r.NanoCPUs != nil && *r.NanoCPUs < 0 {
		return fmt.Errorf("nanoCPUs must be >= 0")
	}
	if rp := r.RestartPolicy; rp != nil {
		if !validRestartPolicyNames[rp.Name] {
			return fmt.Errorf("invalid restart policy: %q (must be one of no, always, unless-stopped, on-failure)", rp.Name)
		}
		if rp.MaximumRetryCount < 0 {
			return fmt.Errorf("maximumRetryCount must be >= 0")
		}
		if rp.MaximumRetryCount > 0 && rp.Name != "on-failure" {
			return fmt.Errorf("maximumRetryCount is only valid with the on-failure restart policy")
		}
	}
	return nil
}
