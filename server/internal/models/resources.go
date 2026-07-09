package models

// ImageInfo represents the minimal image information exposed by the API
type ImageInfo struct {
	ID       string   `json:"id"`
	RepoTags []string `json:"repo_tags"`
	Size     int64    `json:"size"`
	Created  int64    `json:"created"`
	Host     string   `json:"host"`
}

// VolumeInfo represents the minimal volume information exposed by the API
type VolumeInfo struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`
	Mountpoint string            `json:"mountpoint"`
	Created    string            `json:"created"`
	Labels     map[string]string `json:"labels,omitempty"`
	Host       string            `json:"host"`
}

// NetworkInfo represents the minimal network information exposed by the API
type NetworkInfo struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Driver  string   `json:"driver"`
	Scope   string   `json:"scope"`
	Subnets []string `json:"subnets,omitempty"`
	Host    string   `json:"host"`
}
