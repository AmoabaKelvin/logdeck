package dokploy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
)

type ResourceType string

const (
	Application ResourceType = "application"
	Compose     ResourceType = "compose"
)

type Resource struct {
	MappingRevision string       `json:"mapping_revision"`
	Type            ResourceType `json:"type"`
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	AppName         string       `json:"appName"`
	ServerID        *string      `json:"serverId"`
	Project         string       `json:"project"`
	Environment     string       `json:"environment"`
	PreviewNames    []string     `json:"-"`
}

type Environment struct {
	Text          *string `json:"text"`
	Revision      string  `json:"revision"`
	CreateEnvFile *bool   `json:"createEnvFile"`
}

// EnvironmentStore is the saved-configuration contract used by the API.
// It deliberately separates persistence from deployment.
type EnvironmentStore interface {
	ReadEnvironment(context.Context, Resource) (Environment, error)
	SaveEnvironment(context.Context, Resource, string, string) (Environment, error)
	Deploy(context.Context, Resource) error
}

type record struct {
	ApplicationID      string          `json:"applicationId"`
	ComposeID          string          `json:"composeId"`
	Name               string          `json:"name"`
	AppName            string          `json:"appName"`
	ServerID           *string         `json:"serverId"`
	Env                json.RawMessage `json:"env"`
	BuildArgs          json.RawMessage `json:"buildArgs"`
	BuildSecrets       json.RawMessage `json:"buildSecrets"`
	CreateEnvFile      json.RawMessage `json:"createEnvFile"`
	PreviewDeployments []struct {
		AppName string `json:"appName"`
	} `json:"previewDeployments"`
}

func (c *Client) readRecord(ctx context.Context, resource Resource) (record, error) {
	var rec record
	if resource.ID == "" || (resource.Type != Application && resource.Type != Compose) {
		return rec, fmt.Errorf("unsupported Dokploy resource")
	}
	key := string(resource.Type) + "Id"
	err := c.request(ctx, http.MethodGet, string(resource.Type)+".one?"+key+"="+url.QueryEscape(resource.ID), nil, &rec)
	if err != nil {
		return rec, err
	}
	id := rec.ApplicationID
	if resource.Type == Compose {
		id = rec.ComposeID
	}
	server := ""
	if rec.ServerID != nil {
		server = *rec.ServerID
	}
	if id != resource.ID || server != c.serverID || rec.AppName == "" || (resource.AppName != "" && resource.AppName != rec.AppName) {
		return rec, fmt.Errorf("dokploy resource no longer belongs to the configured server; choose its deployment again")
	}
	return rec, nil
}

func (c *Client) Resources(ctx context.Context) ([]Resource, error) {
	var projects []struct {
		Name         string `json:"name"`
		Environments []struct {
			Name         string `json:"name"`
			Applications []struct {
				ID string `json:"applicationId"`
			} `json:"applications"`
			Compose []struct {
				ID string `json:"composeId"`
			} `json:"compose"`
		} `json:"environments"`
	}
	if err := c.request(ctx, http.MethodGet, "project.all", nil, &projects); err != nil {
		return nil, err
	}
	resources := make([]Resource, 0)
	// project.all intentionally exposes only summaries. Read the owning records
	// to obtain appName and serverId rather than assuming local deployments.
	for _, p := range projects {
		for _, e := range p.Environments {
			ids := make([]Resource, 0, len(e.Applications)+len(e.Compose))
			for _, a := range e.Applications {
				ids = append(ids, Resource{Type: Application, ID: a.ID})
			}
			for _, a := range e.Compose {
				ids = append(ids, Resource{Type: Compose, ID: a.ID})
			}
			for _, r := range ids {
				var rec record
				if r.ID == "" {
					return nil, fmt.Errorf("dokploy returned an incomplete resource inventory")
				}
				if err := c.request(ctx, http.MethodGet, string(r.Type)+".one?"+string(r.Type)+"Id="+url.QueryEscape(r.ID), nil, &rec); err != nil {
					return nil, err
				}
				server := ""
				if rec.ServerID != nil {
					server = *rec.ServerID
				}
				if server != c.serverID {
					continue
				}
				r.MappingRevision = c.mappingRevision(r, rec.AppName)
				r.Name = rec.Name
				r.AppName = rec.AppName
				r.ServerID = rec.ServerID
				r.Project = p.Name
				r.Environment = e.Name
				for _, preview := range rec.PreviewDeployments {
					r.PreviewNames = append(r.PreviewNames, preview.AppName)
				}
				resources = append(resources, r)
			}
		}
	}
	return resources, nil
}

func revision(rec record) string {
	// Include every setting that this API overwrites, including null values.
	b, _ := json.Marshal([]json.RawMessage{rec.Env, rec.BuildArgs, rec.BuildSecrets, rec.CreateEnvFile})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func environment(rec record, kind ResourceType) (Environment, error) {
	var result Environment
	if len(rec.Env) == 0 || len(rec.CreateEnvFile) == 0 {
		return result, fmt.Errorf("dokploy omitted environment settings; editing is unavailable")
	}
	if kind == Application && (len(rec.BuildArgs) == 0 || len(rec.BuildSecrets) == 0) {
		return result, fmt.Errorf("dokploy omitted build settings; editing is unavailable")
	}
	if err := json.Unmarshal(rec.Env, &result.Text); err != nil {
		return result, fmt.Errorf("unsupported Dokploy environment format")
	}
	if err := json.Unmarshal(rec.CreateEnvFile, &result.CreateEnvFile); err != nil {
		return result, fmt.Errorf("unsupported Dokploy environment-file setting")
	}
	if result.CreateEnvFile == nil {
		return result, fmt.Errorf("unsupported Dokploy environment-file setting")
	}
	result.Revision = revision(rec)
	return result, nil
}
func (c *Client) ReadEnvironment(ctx context.Context, r Resource) (Environment, error) {
	rec, err := c.readRecord(ctx, r)
	if err != nil {
		return Environment{}, err
	}
	return environment(rec, r.Type)
}

var ErrStaleEnvironment = errors.New("dokploy configuration changed; reload before saving")
var saveLocks sync.Map

func (c *Client) SaveEnvironment(ctx context.Context, r Resource, text, expected string) (Environment, error) {
	// Compose services, replicas and duplicate host connections share one record.
	key := c.apiURL + "/" + string(r.Type) + "/" + r.ID
	value, _ := saveLocks.LoadOrStore(key, &sync.Mutex{})
	lock := value.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	rec, err := c.readRecord(ctx, r)
	if err != nil {
		return Environment{}, err
	}
	current, err := environment(rec, r.Type)
	if err != nil {
		return Environment{}, err
	}
	if expected == "" || expected != current.Revision {
		return Environment{}, ErrStaleEnvironment
	}
	payload := map[string]any{string(r.Type) + "Id": r.ID, "env": text, "createEnvFile": rec.CreateEnvFile}
	if r.Type == Application {
		payload["buildArgs"] = rec.BuildArgs
		payload["buildSecrets"] = rec.BuildSecrets
	}
	var acknowledged bool
	if err := c.request(ctx, http.MethodPost, string(r.Type)+".saveEnvironment", payload, &acknowledged); err != nil {
		return Environment{}, err
	}
	if !acknowledged {
		return Environment{}, fmt.Errorf("dokploy did not acknowledge the save; reload before retrying")
	}
	// A read-back can fail after a successful write. Do not claim it was rolled back.
	rec.Env, _ = json.Marshal(text)
	return environment(rec, r.Type)
}
func (c *Client) Deploy(ctx context.Context, r Resource) error {
	if _, err := c.readRecord(ctx, r); err != nil {
		return err
	}
	// freshVolumes is deliberately omitted: applying environment must retain volumes.
	return c.request(ctx, http.MethodPost, string(r.Type)+".deploy", map[string]string{string(r.Type) + "Id": r.ID, "title": "Apply environment saved from LogDeck"}, nil)
}

func (c *Client) mappingRevision(r Resource, appName string) string {
	value := c.apiURL + "\x00" + c.serverID + "\x00" + string(r.Type) + "\x00" + r.ID + "\x00" + appName
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
