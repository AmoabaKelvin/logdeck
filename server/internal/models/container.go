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
	Labels  map[string]string `json:"labels,omitempty"`
}

// LogOptions represents options for fetching container logs
type LogOptions struct {
	Follow     bool   `json:"follow"`
	Timestamps bool   `json:"timestamps"`
	Since      string `json:"since"`
	Until      string `json:"until"`
	Tail       string `json:"tail"`
	Details    bool   `json:"details"`
	ShowStdout bool   `json:"show_stdout"`
	ShowStderr bool   `json:"show_stderr"`
}

// DefaultLogOptions returns sensible defaults for log fetching
func DefaultLogOptions() LogOptions {
	return LogOptions{
		Follow:     false,
		Timestamps: true,
		Tail:       "100",
		ShowStdout: true,
		ShowStderr: true,
		Details:    false,
	}
}
