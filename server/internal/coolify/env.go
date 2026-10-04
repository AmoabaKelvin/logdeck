package coolify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

// Value is nil when Coolify hides it or the token cannot read it. RealValue is
// deliberately not decoded: it resolves shared references and is not config.
type EnvVar struct {
	UUID        string  `json:"uuid"`
	Key         string  `json:"key"`
	Value       *string `json:"value"`
	IsPreview   bool    `json:"is_preview"`
	IsLiteral   bool    `json:"is_literal"`
	IsMultiline bool    `json:"is_multiline"`
	IsShownOnce bool    `json:"is_shown_once"`
	IsRuntime   *bool   `json:"is_runtime,omitempty"`
	IsBuildtime *bool   `json:"is_buildtime,omitempty"`
	IsShared    bool    `json:"is_shared"`
}

// Existing records are addressed by UUID; new records have only a key/scope.
// Settings on existing records are preserved from a fresh Coolify read.
type EnvChange struct {
	UUID          string  `json:"uuid,omitempty"`
	Key           string  `json:"key"`
	Value         *string `json:"value,omitempty"`
	ExpectedValue *string `json:"expected_value"`
	IsPreview     bool    `json:"is_preview"`
	Remove        bool    `json:"remove,omitempty"`
}

func (r *ResourceInfo) ValidateEnvSupport() error {
	if r == nil || r.UUID == "" {
		return fmt.Errorf("coolify: resource ownership could not be determined; edit its environment in Coolify")
	}
	if r.Type == ResourceTypeDatabase {
		return fmt.Errorf("coolify: database environment editing is not supported; edit its configuration in Coolify")
	}
	if r.Type != ResourceTypeApplication && r.Type != ResourceTypeService {
		return fmt.Errorf("coolify: environment editing is not supported for resource type %q", r.Type)
	}
	return nil
}

func (c *Client) envURL(resource *ResourceInfo) string {
	return fmt.Sprintf("%s/api/v1/%ss/%s/envs", c.apiURL, resource.Type, url.PathEscape(resource.UUID))
}

func (c *Client) ListEnvVars(ctx context.Context, resource *ResourceInfo) ([]EnvVar, error) {
	if err := resource.ValidateEnvSupport(); err != nil {
		return nil, err
	}
	body, err := c.doRequest(ctx, http.MethodGet, c.envURL(resource), nil)
	if err != nil {
		return nil, fmt.Errorf("could not read Coolify environment: %w", err)
	}
	vars := make([]EnvVar, 0)
	if err := json.Unmarshal(body, &vars); err != nil {
		return nil, err
	}
	for i := range vars {
		if vars[i].IsShownOnce {
			vars[i].Value = nil
		}
	}
	return vars, nil
}

var envKeyPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
var resourceEnvLocks sync.Map

// SaveEnvChanges returns the number of acknowledged writes. Coolify has no
// transaction across variables; a failed request may also have committed.
func (c *Client) SaveEnvChanges(ctx context.Context, resource *ResourceInfo, changes []EnvChange) (int, error) {
	if err := resource.ValidateEnvSupport(); err != nil {
		return 0, err
	}
	lock, _ := resourceEnvLocks.LoadOrStore(c.envURL(resource), &sync.Mutex{})
	mu := lock.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()
	existing, err := c.ListEnvVars(ctx, resource)
	if err != nil {
		return 0, err
	}
	byUUID := make(map[string]EnvVar, len(existing))
	byScope := make(map[string]int, len(existing))
	scope := func(key string, preview bool) string { return fmt.Sprintf("%s/%t", key, preview) }
	for _, v := range existing {
		byUUID[v.UUID] = v
		byScope[scope(v.Key, v.IsPreview)]++
	}
	seen := make(map[string]bool)
	// Validate the entire request before the first mutation.
	for _, change := range changes {
		identity := scope(change.Key, change.IsPreview)
		if !envKeyPattern.MatchString(change.Key) || IsCoolifyDefaultEnvVar(change.Key) {
			return 0, fmt.Errorf("invalid or reserved Coolify variable key %q", change.Key)
		}
		if seen[identity] {
			return 0, fmt.Errorf("duplicate change for %s", change.Key)
		}
		seen[identity] = true
		if resource.Type == ResourceTypeService && change.IsPreview {
			return 0, fmt.Errorf("service preview variables are not supported")
		}
		if change.UUID != "" {
			v, ok := byUUID[change.UUID]
			if !ok || v.Key != change.Key || v.IsPreview != change.IsPreview || byScope[identity] != 1 {
				return 0, fmt.Errorf("coolify: variable %s changed or is ambiguous; reload before saving", change.Key)
			}
			// Unknown values have no comparable baseline. Replacements are
			// explicit; readable values must still match the loaded original.
			if (v.Value == nil) != (change.ExpectedValue == nil) || (v.Value != nil && *v.Value != *change.ExpectedValue) {
				return 0, fmt.Errorf("coolify: variable %s changed since it was loaded or its original value was not supplied; reload before saving", change.Key)
			}
		} else if change.Remove || byScope[identity] != 0 {
			return 0, fmt.Errorf("coolify: variable %s already exists or cannot be removed without its UUID; reload before saving", change.Key)
		}
		if !change.Remove && change.Value == nil {
			return 0, fmt.Errorf("a replacement value is required for %s", change.Key)
		}
	}
	completed := 0
	for _, change := range changes {
		method, endpoint := http.MethodPatch, c.envURL(resource)
		var body []byte
		if change.Remove {
			method, endpoint = http.MethodDelete, endpoint+"/"+url.PathEscape(change.UUID)
		} else {
			v := byUUID[change.UUID]
			if change.UUID == "" {
				method = http.MethodPost
				v.IsMultiline = strings.Contains(*change.Value, "\n")
			}
			payload := map[string]any{"key": change.Key, "value": *change.Value,
				"is_literal": v.IsLiteral, "is_multiline": v.IsMultiline, "is_shown_once": v.IsShownOnce}
			if resource.Type == ResourceTypeApplication {
				payload["is_preview"] = change.IsPreview
				if v.IsRuntime != nil {
					payload["is_runtime"] = *v.IsRuntime
				}
				if v.IsBuildtime != nil {
					payload["is_buildtime"] = *v.IsBuildtime
				}
			}
			body, err = json.Marshal(payload)
			if err != nil {
				return completed, err
			}
		}
		if _, err := c.doRequest(ctx, method, endpoint, body); err != nil {
			return completed, fmt.Errorf("coolify: save failed for %s after %d acknowledged changes; some changes may have been saved. Reload before retrying: %w", change.Key, completed, err)
		}
		completed++
	}
	return completed, nil
}

// Deploy queues work in Coolify. An acknowledgement is not a completed deploy.
func (c *Client) Deploy(ctx context.Context, resource *ResourceInfo) error {
	if err := resource.ValidateEnvSupport(); err != nil {
		return err
	}
	endpoint := c.apiURL + "/api/v1/deploy?uuid=" + url.QueryEscape(resource.UUID)
	method := http.MethodGet
	if resource.Type == ResourceTypeService {
		method = http.MethodPost
		endpoint = c.apiURL + "/api/v1/services/" + url.PathEscape(resource.UUID) + "/restart"
	}
	body, err := c.doRequest(ctx, method, endpoint, nil)
	if err != nil {
		return err
	}
	if resource.Type == ResourceTypeApplication {
		var response struct {
			Deployments []struct {
				ResourceUUID   string `json:"resource_uuid"`
				DeploymentUUID string `json:"deployment_uuid"`
			} `json:"deployments"`
		}
		if err := json.Unmarshal(body, &response); err != nil {
			return fmt.Errorf("invalid Coolify deployment response: %w", err)
		}
		for _, deployment := range response.Deployments {
			if deployment.ResourceUUID == resource.UUID && deployment.DeploymentUUID != "" {
				return nil
			}
		}
		return fmt.Errorf("coolify: did not acknowledge an application deployment; check deployment permissions and status in Coolify")
	}
	return nil
}
