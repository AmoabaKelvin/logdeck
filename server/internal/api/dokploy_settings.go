package api

import (
	"encoding/json"
	"fmt"
	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/dokploy"
	"net/http"
	"net/url"
	"strings"
)

// UpdateDokployHosts handles PUT /api/v1/settings/dokploy-hosts.
func (ar *APIRouter) UpdateDokployHosts(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Hosts []struct {
			HostName string `json:"hostName"`
			APIURL   string `json:"apiURL"`
			APIToken string `json:"apiToken"`
			ServerID string `json:"serverId"`
		} `json:"hosts"`
		Revision string `json:"revision"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Resolve masked tokens from existing file config.
	existing := ar.manager.FileConfigSnapshot()
	existingMap := make(map[string]config.DokployHostConfig)
	for _, ch := range existing.DokployHosts {
		existingMap[ch.HostName] = ch
	}

	hosts := make([]config.DokployHostConfig, 0, len(req.Hosts))
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
		if !isValidPlatformURL(h.APIURL) {
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
			if stored, ok := existingMap[h.HostName]; ok && stored.APIURL == strings.TrimRight(h.APIURL, "/") {
				token = stored.APIToken
			} else {
				http.Error(w, fmt.Sprintf("no existing token for host %q; provide the actual token", h.HostName), http.StatusBadRequest)
				return
			}
		}

		knownHost := false
		for _, dh := range ar.registry.Config().DockerHosts {
			if dh.Name == h.HostName {
				knownHost = true
				break
			}
		}
		if !knownHost {
			http.Error(w, "Dokploy connection references an unknown Docker host", 400)
			return
		}
		hosts = append(hosts, config.DokployHostConfig{
			HostName: h.HostName,
			APIURL:   strings.TrimRight(h.APIURL, "/"),
			APIToken: token,
			ServerID: h.ServerID,
		})
	}

	if err := ar.manager.UpdateDokployHosts(hosts, req.Revision); err != nil {
		http.Error(w, err.Error(), settingsErrorStatus(err))
		return
	}

	WriteJsonResponse(w, http.StatusOK, map[string]any{"message": "Dokploy hosts updated"})
}

func isValidPlatformURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" && u.User == nil && u.RawQuery == "" && u.Fragment == ""
}
func (ar *APIRouter) TestDokployHost(w http.ResponseWriter, r *http.Request) {
	var req config.DokployHostConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", 400)
		return
	}
	if !isValidPlatformURL(req.APIURL) || req.APIToken == "" {
		http.Error(w, "A valid API URL and API token are required", 400)
		return
	}
	if req.APIToken == secretMask {
		for _, h := range ar.registry.Config().DokployHosts {
			if h.HostName == req.HostName && h.APIURL == req.APIURL {
				req.APIToken = h.APIToken
				break
			}
		}
		if req.APIToken == secretMask {
			http.Error(w, "Provide the API token for this instance", 400)
			return
		}
	}
	err := dokploy.NewClient(req).TestConnection(r.Context())
	message := "Connected. Resource discovery is available; save and deploy permissions are checked when used."
	if err != nil {
		message = dokployMessage(err)
	}
	WriteJsonResponse(w, 200, map[string]any{"success": err == nil, "message": message})
}
