package dokploy

import (
	"context"
	"errors"
	"time"
)

// Dokploy's deployment states. A deployment row appears only once its queued
// job starts, so a request still waiting in the queue has no row yet.
const (
	DeploymentRunning   = "running"
	DeploymentDone      = "done"
	DeploymentError     = "error"
	DeploymentCancelled = "cancelled"
)

// ErrMappingChanged means the confirmed deployment no longer matches the
// resource, so following it again needs a new confirmation.
var ErrMappingChanged = errors.New("the Dokploy connection or deployment name changed; choose the deployment again")

type deploymentRecord struct {
	Status       string  `json:"status"`
	CreatedAt    string  `json:"createdAt"`
	FinishedAt   *string `json:"finishedAt"`
	ErrorMessage *string `json:"errorMessage"`
}

// Deployment is one run of Dokploy's deployment job for a resource. Its times
// are Dokploy's clock.
type Deployment struct {
	Status       string
	StartedAt    time.Time
	FinishedAt   time.Time
	ErrorMessage string
}

// newest is the deployment that started last, or nil. Dokploy does not sort
// a record's deployments.
func newest(records []deploymentRecord, after time.Time) *Deployment {
	var latest *Deployment
	for _, d := range records {
		started, err := time.Parse(time.RFC3339Nano, d.CreatedAt)
		if err != nil || !started.After(after) || (latest != nil && !started.After(latest.StartedAt)) {
			continue
		}
		latest = &Deployment{Status: d.Status, StartedAt: started}
		if d.FinishedAt != nil {
			latest.FinishedAt, _ = time.Parse(time.RFC3339Nano, *d.FinishedAt)
		}
		if d.ErrorMessage != nil {
			latest.ErrorMessage = *d.ErrorMessage
		}
	}
	return latest
}

// LastDeploymentStart is when the resource's newest deployment started, by
// Dokploy's clock, or the zero time when it has none. Read before requesting
// a deployment, it separates later runs without comparing two machines' clocks.
func (c *Client) LastDeploymentStart(ctx context.Context, r Resource) (time.Time, error) {
	rec, err := c.readRecord(ctx, r)
	if err != nil {
		return time.Time{}, err
	}
	if latest := newest(rec.Deployments, time.Time{}); latest != nil {
		return latest.StartedAt, nil
	}
	return time.Time{}, nil
}

// LatestDeployment returns the resource's deployment name and the newest
// deployment that started after the given start, or nil while none has. Any
// such run applies the configuration saved before it, whoever requested it.
func (c *Client) LatestDeployment(ctx context.Context, r Resource, after time.Time) (string, *Deployment, error) {
	rec, err := c.readRecord(ctx, r)
	if err != nil {
		return "", nil, err
	}
	if c.mappingRevision(r, rec.AppName) != r.MappingRevision {
		return "", nil, ErrMappingChanged
	}
	return rec.AppName, newest(rec.Deployments, after), nil
}
