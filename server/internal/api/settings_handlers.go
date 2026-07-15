package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/auth"
	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/coolify"
	"github.com/AmoabaKelvin/logdeck/internal/docker"
)

const secretMask = "••••••••"

var hostNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// GetSettings returns the current configuration with source tracking and masked secrets.
func (ar *APIRouter) GetSettings(w http.ResponseWriter, r *http.Request) {
	sources := ar.manager.Sources()
	cfg := ar.registry.Config()
	fc := ar.manager.FileConfigSnapshot()

	envDockerNames := ar.manager.EnvDockerHostNames()
	dockerHosts := make([]map[string]any, 0, len(cfg.DockerHosts))
	for _, h := range cfg.DockerHosts {
		source := config.SourceFile
		if envDockerNames[h.Name] {
			source = config.SourceEnv
		}
		dockerHosts = append(dockerHosts, map[string]any{
			"name":   h.Name,
			"host":   h.Host,
			"source": source,
		})
	}

	envCoolifyNames := ar.manager.EnvCoolifyHostNames()
	coolifyHosts := make([]map[string]any, 0, len(cfg.CoolifyHosts))
	for _, ch := range cfg.CoolifyHosts {
		source := config.SourceFile
		if envCoolifyNames[ch.HostName] {
			source = config.SourceEnv
		}
		coolifyHosts = append(coolifyHosts, map[string]any{
			"hostName": ch.HostName,
			"apiURL":   ch.APIURL,
			"apiToken": secretMask,
			"source":   source,
		})
	}

	authResp := map[string]any{
		"source":  sources.Auth,
		"enabled": false,
	}
	if sources.Auth == config.SourceEnv {
		svc := ar.registry.Auth()
		authResp["enabled"] = svc != nil
	} else if fc.Auth != nil {
		authResp["enabled"] = fc.Auth.Enabled
		authResp["adminUsername"] = fc.Auth.AdminUsername
	}

	// Log storage: each cap is overridden independently, so each carries its
	// own source rather than one for the section.
	logStore := ar.manager.LogStore()
	logStoreSources := ar.manager.LogStoreSources()

	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"dockerHosts": map[string]any{
			"source": sources.DockerHosts,
			"hosts":  dockerHosts,
		},
		"logStore": map[string]any{
			"enabled":              logStore.Enabled,
			"enabledSource":        logStoreSources.Enabled,
			"perContainerMB":       logStore.PerContainerMB,
			"perContainerMBSource": logStoreSources.PerContainerMB,
			"totalMB":              logStore.TotalMB,
			"totalMBSource":        logStoreSources.TotalMB,
		},
		"coolifyHosts": map[string]any{
			"source": sources.CoolifyHosts,
			"hosts":  coolifyHosts,
		},
		"readOnly": map[string]any{
			"source": sources.ReadOnly,
			"value":  cfg.ReadOnly,
		},
		"auth": authResp,
	})
}

// UpdateDockerHosts handles PUT /api/v1/settings/docker-hosts.
func (ar *APIRouter) UpdateDockerHosts(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Hosts []config.DockerHost `json:"hosts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Reject empty list only if there are no env-defined hosts to fall back on.
	if len(req.Hosts) == 0 && len(ar.manager.EnvDockerHostNames()) == 0 {
		http.Error(w, "at least one Docker host is required", http.StatusBadRequest)
		return
	}

	seen := make(map[string]bool)
	for _, h := range req.Hosts {
		if !hostNameRegex.MatchString(h.Name) {
			http.Error(w, fmt.Sprintf("invalid host name: %q", h.Name), http.StatusBadRequest)
			return
		}
		if !isValidDockerHostURL(h.Host) {
			http.Error(w, fmt.Sprintf("invalid host URL: %q (must be a valid unix://, ssh://, or tcp:// URL)", h.Host), http.StatusBadRequest)
			return
		}
		if seen[h.Name] {
			http.Error(w, fmt.Sprintf("duplicate host name: %q", h.Name), http.StatusBadRequest)
			return
		}
		seen[h.Name] = true
	}

	if err := ar.manager.UpdateDockerHosts(req.Hosts); err != nil {
		http.Error(w, err.Error(), settingsErrorStatus(err))
		return
	}

	WriteJsonResponse(w, http.StatusOK, map[string]any{"message": "Docker hosts updated"})
}

// UpdateCoolifyHosts handles PUT /api/v1/settings/coolify-hosts.
func (ar *APIRouter) UpdateCoolifyHosts(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Hosts []struct {
			HostName string `json:"hostName"`
			APIURL   string `json:"apiURL"`
			APIToken string `json:"apiToken"`
		} `json:"hosts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Resolve masked tokens from existing file config.
	existing := ar.manager.FileConfigSnapshot()
	existingMap := make(map[string]string)
	for _, ch := range existing.CoolifyHosts {
		existingMap[ch.HostName] = ch.APIToken
	}

	hosts := make([]config.CoolifyHostConfig, 0, len(req.Hosts))
	seen := make(map[string]bool)
	for _, h := range req.Hosts {
		if h.HostName == "" || h.APIURL == "" || h.APIToken == "" {
			http.Error(w, "hostName, apiURL, and apiToken are required for each entry", http.StatusBadRequest)
			return
		}
		if !hostNameRegex.MatchString(h.HostName) {
			http.Error(w, fmt.Sprintf("invalid host name: %q", h.HostName), http.StatusBadRequest)
			return
		}
		if !isValidCoolifyURL(h.APIURL) {
			http.Error(w, fmt.Sprintf("invalid API URL: %q (must start with http:// or https://)", h.APIURL), http.StatusBadRequest)
			return
		}
		if seen[h.HostName] {
			http.Error(w, fmt.Sprintf("duplicate host name: %q", h.HostName), http.StatusBadRequest)
			return
		}
		seen[h.HostName] = true

		token := h.APIToken
		if token == secretMask {
			if stored, ok := existingMap[h.HostName]; ok {
				token = stored
			} else {
				http.Error(w, fmt.Sprintf("no existing token for host %q; provide the actual token", h.HostName), http.StatusBadRequest)
				return
			}
		}

		hosts = append(hosts, config.CoolifyHostConfig{
			HostName: h.HostName,
			APIURL:   strings.TrimRight(h.APIURL, "/"),
			APIToken: token,
		})
	}

	if err := ar.manager.UpdateCoolifyHosts(hosts); err != nil {
		http.Error(w, err.Error(), settingsErrorStatus(err))
		return
	}

	WriteJsonResponse(w, http.StatusOK, map[string]any{"message": "Coolify hosts updated"})
}

// UpdateReadOnly handles PUT /api/v1/settings/read-only.
func (ar *APIRouter) UpdateReadOnly(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Value bool `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := ar.manager.UpdateReadOnly(req.Value); err != nil {
		http.Error(w, err.Error(), settingsErrorStatus(err))
		return
	}

	WriteJsonResponse(w, http.StatusOK, map[string]any{"message": "Read-only mode updated"})
}

// maxLogStoreMB bounds a retention cap at 1 TiB — far past any sensible SQLite
// log database, but enough to keep a typo from requesting a petabyte.
const maxLogStoreMB = 1024 * 1024

// UpdateLogStorage handles PUT /api/v1/settings/log-storage. Every field is
// optional; only the ones provided change. The janitor re-reads the caps on
// each pass, so a new cap takes effect without a restart.
func (ar *APIRouter) UpdateLogStorage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled        *bool `json:"enabled"`
		PerContainerMB *int  `json:"perContainerMB"`
		TotalMB        *int  `json:"totalMB"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// A value pinned by an environment variable cannot be changed from the UI,
	// the same rule the other env-sourced settings follow.
	sources := ar.manager.LogStoreSources()
	pinned := []struct {
		provided bool
		source   config.Source
		message  string
	}{
		{req.Enabled != nil, sources.Enabled, "enabled is set via the LOG_STORE_ENABLED environment variable and cannot be changed from the UI"},
		{req.PerContainerMB != nil, sources.PerContainerMB, "perContainerMB is set via the LOG_STORE_PER_CONTAINER_MB environment variable and cannot be changed from the UI"},
		{req.TotalMB != nil, sources.TotalMB, "totalMB is set via the LOG_STORE_TOTAL_MB environment variable and cannot be changed from the UI"},
	}
	for _, f := range pinned {
		if f.provided && f.source == config.SourceEnv {
			http.Error(w, f.message, http.StatusConflict)
			return
		}
	}

	// Validate the caps the store would end up with: changing one cap still has
	// to stay consistent with the other one's current value.
	effective := ar.manager.LogStore()
	if req.PerContainerMB != nil {
		effective.PerContainerMB = *req.PerContainerMB
	}
	if req.TotalMB != nil {
		effective.TotalMB = *req.TotalMB
	}
	if err := validateLogStoreCaps(effective); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := ar.manager.UpdateLogStore(func(current config.LogStoreConfig) (config.LogStoreConfig, error) {
		if req.Enabled != nil {
			current.Enabled = req.Enabled
		}
		if req.PerContainerMB != nil {
			current.PerContainerMB = req.PerContainerMB
		}
		if req.TotalMB != nil {
			current.TotalMB = req.TotalMB
		}
		return current, nil
	})
	if err != nil {
		http.Error(w, err.Error(), settingsErrorStatus(err))
		return
	}

	WriteJsonResponse(w, http.StatusOK, map[string]any{"message": "log storage settings updated"})
}

// validateLogStoreCaps rejects caps that make retention nonsense: a non-positive
// cap would evict the whole store on the next janitor pass, and a per-container
// cap above the total cap can never be reached.
func validateLogStoreCaps(cfg config.ResolvedLogStoreConfig) error {
	if cfg.PerContainerMB < 1 || cfg.PerContainerMB > maxLogStoreMB {
		return fmt.Errorf("perContainerMB must be between 1 and %d", maxLogStoreMB)
	}
	if cfg.TotalMB < 1 || cfg.TotalMB > maxLogStoreMB {
		return fmt.Errorf("totalMB must be between 1 and %d", maxLogStoreMB)
	}
	if cfg.PerContainerMB > cfg.TotalMB {
		return fmt.Errorf("perContainerMB (%d) cannot exceed totalMB (%d)", cfg.PerContainerMB, cfg.TotalMB)
	}
	return nil
}

// UpdateAuth handles PUT /api/v1/settings/auth.
func (ar *APIRouter) UpdateAuth(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled       bool   `json:"enabled"`
		AdminUsername string `json:"adminUsername"`
		NewPassword   string `json:"newPassword,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Enabled && req.AdminUsername == "" {
		http.Error(w, "adminUsername is required when enabling auth", http.StatusBadRequest)
		return
	}

	err := ar.manager.UpdateAuth(func(authCfg *config.FileAuthConfig) (*config.FileAuthConfig, error) {
		authCfg.Enabled = req.Enabled

		if req.Enabled {
			authCfg.AdminUsername = req.AdminUsername

			if authCfg.JWTSecret == "" {
				secret, err := auth.GenerateRandomHex(32)
				if err != nil {
					return nil, fmt.Errorf("failed to generate JWT secret")
				}
				authCfg.JWTSecret = secret
			}

			if req.NewPassword != "" {
				// New passwords are stored as bcrypt hashes. Existing SHA256
				// hashes keep working (ValidateCredentials detects the format
				// by prefix) until the password is next changed.
				hash, err := auth.HashPassword(req.NewPassword)
				if err != nil {
					return nil, fmt.Errorf("failed to hash password")
				}
				authCfg.AdminPasswordHash = hash
				authCfg.AdminPasswordSalt = ""
			}

			if authCfg.AdminPasswordHash == "" {
				return nil, fmt.Errorf("newPassword is required when first enabling auth")
			}
		}

		return authCfg, nil
	})

	if err != nil {
		http.Error(w, err.Error(), settingsErrorStatus(err))
		return
	}

	WriteJsonResponse(w, http.StatusOK, map[string]any{"message": "Auth settings updated"})
}

// TestDockerHost handles POST /api/v1/settings/test/docker-host.
func (ar *APIRouter) TestDockerHost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		Host string `json:"host"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Host == "" {
		http.Error(w, "host is required", http.StatusBadRequest)
		return
	}

	if !isValidDockerHostURL(req.Host) {
		http.Error(w, "invalid host URL (must be a valid unix://, ssh://, or tcp:// URL)", http.StatusBadRequest)
		return
	}

	tempClient, err := docker.NewMultiHostClient([]config.DockerHost{
		{Name: "test", Host: req.Host},
	})
	if err != nil {
		WriteJsonResponse(w, http.StatusOK, map[string]any{
			"success": false,
			"message": fmt.Sprintf("Failed to create client: %v", err),
		})
		return
	}
	defer tempClient.Close()

	cl, err := tempClient.GetClient("test")
	if err != nil {
		WriteJsonResponse(w, http.StatusOK, map[string]any{
			"success": false,
			"message": fmt.Sprintf("Internal error: %v", err),
		})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	ping, err := cl.Ping(ctx)
	if err != nil {
		WriteJsonResponse(w, http.StatusOK, map[string]any{
			"success": false,
			"message": fmt.Sprintf("Connection failed: %v", err),
		})
		return
	}

	response := map[string]any{
		"success":       true,
		"message":       "Connection successful",
		"dockerVersion": ping.APIVersion,
	}
	// Identify the engine (Docker or Podman) so the UI can say what it
	// actually connected to. Best-effort: the connection test already passed.
	if engine, version, err := tempClient.EngineInfo(ctx, "test"); err == nil {
		response["engine"] = engine
		response["engineVersion"] = version
	}

	WriteJsonResponse(w, http.StatusOK, response)
}

// TestCoolifyHost handles POST /api/v1/settings/test/coolify-host.
func (ar *APIRouter) TestCoolifyHost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		HostName string `json:"hostName"`
		APIURL   string `json:"apiURL"`
		APIToken string `json:"apiToken"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.APIURL == "" || req.APIToken == "" {
		http.Error(w, "apiURL and apiToken are required", http.StatusBadRequest)
		return
	}

	if !isValidCoolifyURL(req.APIURL) {
		http.Error(w, "invalid API URL (must start with http:// or https://)", http.StatusBadRequest)
		return
	}

	// Resolve masked token.
	token := req.APIToken
	if token == secretMask {
		fc := ar.manager.FileConfigSnapshot()
		for _, ch := range fc.CoolifyHosts {
			if ch.HostName == req.HostName {
				token = ch.APIToken
				break
			}
		}
		if token == secretMask {
			http.Error(w, "no stored token found; provide the actual token", http.StatusBadRequest)
			return
		}
	}

	client := coolify.NewSingleClient(strings.TrimRight(req.APIURL, "/"), token)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := client.TestConnection(ctx); err != nil {
		log.Printf("Coolify test connection failed: %v", err)
		WriteJsonResponse(w, http.StatusOK, map[string]any{
			"success": false,
			"message": fmt.Sprintf("Connection failed: %v", err),
		})
		return
	}

	WriteJsonResponse(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Connection successful",
	})
}

// settingsErrorStatus maps manager errors to appropriate HTTP status codes.
// Env-override errors are 409 Conflict, everything else is 500.
func settingsErrorStatus(err error) int {
	if strings.Contains(err.Error(), "environment variable") {
		return http.StatusConflict
	}
	return http.StatusInternalServerError
}

func isValidDockerHostURL(host string) bool {
	u, err := url.Parse(host)
	if err != nil {
		return false
	}
	switch u.Scheme {
	case "unix":
		return u.Path != "" || u.Opaque != ""
	case "ssh", "tcp":
		return u.Host != ""
	default:
		return false
	}
}

func isValidCoolifyURL(raw string) bool {
	return strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "http://")
}
