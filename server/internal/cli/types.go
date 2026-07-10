package cli

import "time"

// API response shapes. Field names mirror the server's JSON exactly
// (see server/internal/models and server/internal/api).

type containerInfo struct {
	ID      string            `json:"id"`
	Names   []string          `json:"names"`
	Image   string            `json:"image"`
	Command string            `json:"command"`
	Created int64             `json:"created"`
	State   string            `json:"state"`
	Status  string            `json:"status"`
	Labels  map[string]string `json:"labels,omitempty"`
	Host    string            `json:"host"`
}

type dockerHost struct {
	Name string `json:"name"`
	Host string `json:"host"`
}

type hostError struct {
	Host    string `json:"host"`
	Message string `json:"message"`
}

type containersResponse struct {
	Containers []containerInfo `json:"containers"`
	Hosts      []dockerHost    `json:"hosts"`
	HostErrors []hostError     `json:"hostErrors"`
	ReadOnly   bool            `json:"readOnly"`
}

type containerStats struct {
	ID            string  `json:"id"`
	Host          string  `json:"host"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
	MemoryUsed    uint64  `json:"memory_used"`
	MemoryLimit   uint64  `json:"memory_limit"`
}

type hostInfo struct {
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

type logEntry struct {
	Timestamp         time.Time         `json:"timestamp"`
	Level             string            `json:"level"`
	Message           string            `json:"message"`
	Stream            string            `json:"stream"`
	Raw               string            `json:"raw"`
	Fields            map[string]string `json:"fields,omitempty"`
	ContinuationCount int               `json:"continuationCount,omitempty"`
	ContainerID       string            `json:"containerId,omitempty"`
	ContainerName     string            `json:"containerName,omitempty"`
}

type containerEvent struct {
	Host          string `json:"host"`
	ContainerID   string `json:"containerId"`
	ContainerName string `json:"containerName"`
	Action        string `json:"action"`
	Timestamp     int64  `json:"timestamp"`
}

type imageInfo struct {
	ID       string   `json:"id"`
	RepoTags []string `json:"repo_tags"`
	Size     int64    `json:"size"`
	Created  int64    `json:"created"`
	Host     string   `json:"host"`
}

type volumeInfo struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`
	Mountpoint string            `json:"mountpoint"`
	Created    string            `json:"created"`
	Labels     map[string]string `json:"labels,omitempty"`
	Host       string            `json:"host"`
}

type networkInfo struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Driver  string   `json:"driver"`
	Scope   string   `json:"scope"`
	Subnets []string `json:"subnets,omitempty"`
	Host    string   `json:"host"`
}

type restartPolicySpec struct {
	Name              string `json:"name"`
	MaximumRetryCount int    `json:"maximumRetryCount"`
}

type containerResources struct {
	MemoryBytes   int64             `json:"memoryBytes"`
	NanoCPUs      int64             `json:"nanoCPUs"`
	RestartPolicy restartPolicySpec `json:"restartPolicy"`
}

type updateResourcesRequest struct {
	MemoryBytes   *int64             `json:"memoryBytes,omitempty"`
	NanoCPUs      *int64             `json:"nanoCPUs,omitempty"`
	RestartPolicy *restartPolicySpec `json:"restartPolicy,omitempty"`
}

type composeFailure struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Error string `json:"error"`
}

type composeResult struct {
	Project   string           `json:"project"`
	Host      string           `json:"host"`
	Total     int              `json:"total"`
	Succeeded int              `json:"succeeded"`
	Failed    []composeFailure `json:"failed"`
}
