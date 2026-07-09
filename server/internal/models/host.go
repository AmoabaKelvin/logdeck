package models

// HostInfo represents engine-level capacity and inventory info for a
// configured Docker host, as reported by the engine's Info API.
type HostInfo struct {
	Host              string `json:"host"`
	Available         bool   `json:"available"`
	Error             string `json:"error,omitempty"`
	Name              string `json:"name,omitempty"`
	OperatingSystem   string `json:"operating_system,omitempty"`
	Architecture      string `json:"architecture,omitempty"`
	ServerVersion     string `json:"server_version,omitempty"`
	NCPU              int    `json:"ncpu"`
	MemTotal          int64  `json:"mem_total"`
	ContainersRunning int    `json:"containers_running"`
	ContainersPaused  int    `json:"containers_paused"`
	ContainersStopped int    `json:"containers_stopped"`
	Images            int    `json:"images"`
}
