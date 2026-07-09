package models

// ComposeContainerFailure describes a container that failed a compose action.
type ComposeContainerFailure struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Error string `json:"error"`
}

// ComposeActionResult summarizes a compose project action across its containers.
type ComposeActionResult struct {
	Project   string                    `json:"project"`
	Host      string                    `json:"host"`
	Total     int                       `json:"total"`
	Succeeded int                       `json:"succeeded"`
	Failed    []ComposeContainerFailure `json:"failed"`
}
