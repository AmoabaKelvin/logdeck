package models

// ContainerInfo represents the minimal container information exposed by the API
type ContainerInfo struct {
	ID      string            `json:"id"`
	Names   []string          `json:"names"`
	Image   string            `json:"image"`
	ImageID string            `json:"image_id"`
	Command string            `json:"command"`
	Created int64             `json:"created"`
	State   string            `json:"state"`
	Status  string            `json:"status"`
	Health  string            `json:"health,omitempty"`
	Labels  map[string]string `json:"labels,omitempty"`
	Host    string            `json:"host"`
	Ports   []ContainerPort   `json:"ports,omitempty"`
	// Platform names the deployment platform that owns the container when its
	// labels alone cannot say so. Only "dokploy" is set today.
	Platform string `json:"platform,omitempty"`
	// ImageUpdate is the last check of the container's image against its
	// registry, or nil before the first check finishes.
	ImageUpdate *ImageUpdate `json:"image_update,omitempty"`
}

// Image update statuses.
const (
	ImageUpdateAvailable = "available"
	ImageUpdateCurrent   = "current"
	// ImageUpdateUnknown means LogDeck cannot tell; Reason says why.
	ImageUpdateUnknown = "unknown"
	// ImageUpdateUnsupported means the engine cannot look up registry digests.
	ImageUpdateUnsupported = "unsupported"
)

// ImageUpdate says whether the registry has a newer image for the tag a
// container was created from.
type ImageUpdate struct {
	Status    string `json:"status"`
	Reason    string `json:"reason,omitempty"`
	CheckedAt int64  `json:"checked_at"`
}

// ContainerPort is a port published to the host. Exposed-but-unpublished ports
// carry no host port and are left out.
type ContainerPort struct {
	PublicPort  uint16 `json:"publicPort"`
	PrivatePort uint16 `json:"privatePort"`
	Type        string `json:"type"`
}

// ContainerEvent represents a container lifecycle event streamed to the frontend
type ContainerEvent struct {
	Host          string `json:"host"`
	ContainerID   string `json:"containerId"`
	ContainerName string `json:"containerName"`
	Action        string `json:"action"`
	Timestamp     int64  `json:"timestamp"`
}

type LogOptions struct {
	Follow     bool   `json:"follow"`
	Timestamps bool   `json:"timestamps"`
	Since      string `json:"since"`
	Until      string `json:"until"`
	Tail       string `json:"tail"`
	Details    bool   `json:"details"`
	ShowStdout bool   `json:"show_stdout"`
	ShowStderr bool   `json:"show_stderr"`
	Level      string `json:"level,omitempty"`
	Search     string `json:"search,omitempty"`
	// Unredacted skips log redaction. Only the alert engine sets it: its rules
	// match the original line, and it redacts the sample it keeps. Clients
	// cannot set it.
	Unredacted bool `json:"-"`
}

func DefaultLogOptions() LogOptions {
	return LogOptions{
		Timestamps: true,
		Tail:       "100",
		ShowStdout: true,
		ShowStderr: true,
	}
}

type EnvVariables struct {
	Env map[string]string `json:"env"`
}

type ContainerStats struct {
	ID            string  `json:"id"`
	Host          string  `json:"host"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
	MemoryUsed    uint64  `json:"memory_used"`
	MemoryLimit   uint64  `json:"memory_limit"`
}

// ContainerStatsResponse is the API response for container stats
type ContainerStatsResponse struct {
	Stats []ContainerStats `json:"stats"`
}
