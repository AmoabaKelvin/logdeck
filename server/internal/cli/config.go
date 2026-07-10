package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const defaultServerURL = "http://localhost:8080"

// contextConfig is one saved server connection.
type contextConfig struct {
	URL   string `json:"url"`
	Token string `json:"token,omitempty"`
}

// cliConfig is the on-disk CLI configuration (saved contexts).
type cliConfig struct {
	CurrentContext string                   `json:"currentContext,omitempty"`
	Contexts       map[string]contextConfig `json:"contexts,omitempty"`
}

// configPath returns ~/.config/logdeck/config.json, respecting XDG_CONFIG_HOME.
func configPath() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "logdeck", "config.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %v", err)
	}
	return filepath.Join(home, ".config", "logdeck", "config.json"), nil
}

// loadConfig reads the config file; a missing file is an empty config.
func loadConfig(path string) (cliConfig, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cliConfig{}, nil
	}
	if err != nil {
		return cliConfig{}, err
	}
	var cfg cliConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cliConfig{}, fmt.Errorf("invalid config file %s: %v", path, err)
	}
	return cfg, nil
}

// saveConfig writes the config with restrictive permissions: the file stores
// API tokens, so 0600 for the file and 0700 for its directory.
func saveConfig(path string, cfg cliConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return err
	}
	// os.WriteFile only applies the mode on create; tighten pre-existing files.
	return os.Chmod(path, 0o600)
}

// setContext saves (or overwrites) a context and makes it current.
func (c *cliConfig) setContext(name string, ctx contextConfig) {
	if c.Contexts == nil {
		c.Contexts = map[string]contextConfig{}
	}
	c.Contexts[name] = ctx
	c.CurrentContext = name
}

func (c *cliConfig) useContext(name string) error {
	if _, ok := c.Contexts[name]; !ok {
		return fmt.Errorf("no context named %q (see: logdeck context list)", name)
	}
	c.CurrentContext = name
	return nil
}

// removeContext deletes a context, clearing currentContext if it was current.
func (c *cliConfig) removeContext(name string) error {
	if _, ok := c.Contexts[name]; !ok {
		return fmt.Errorf("no context named %q (see: logdeck context list)", name)
	}
	delete(c.Contexts, name)
	if c.CurrentContext == name {
		c.CurrentContext = ""
	}
	return nil
}

// clearToken removes the token from a context, keeping its URL.
func (c *cliConfig) clearToken(name string) error {
	ctx, ok := c.Contexts[name]
	if !ok {
		return fmt.Errorf("no context named %q (see: logdeck context list)", name)
	}
	ctx.Token = ""
	c.Contexts[name] = ctx
	return nil
}

// connection is a resolved server connection plus where each value came from.
type connection struct {
	url         string
	token       string
	urlSource   string // "flag", "env", `context "name"`, or "default"
	tokenSource string // "flag", "env", `context "name"`, or "none"
}

// resolveConnection applies the precedence order: explicit flags >
// LOGDECK_URL/LOGDECK_TOKEN env > active context (or the --context override)
// > default. URL and token resolve independently.
func resolveConnection(flagURL string, urlSet bool, flagToken string, tokenSet bool, envURL, envToken string, cfg cliConfig, contextOverride string) (connection, error) {
	var active contextConfig
	var activeName string
	if contextOverride != "" {
		found, ok := cfg.Contexts[contextOverride]
		if !ok {
			return connection{}, fmt.Errorf("no context named %q (see: logdeck context list)", contextOverride)
		}
		active, activeName = found, contextOverride
	} else if cfg.CurrentContext != "" {
		// A dangling currentContext is ignored rather than fatal.
		if found, ok := cfg.Contexts[cfg.CurrentContext]; ok {
			active, activeName = found, cfg.CurrentContext
		}
	}

	conn := connection{url: defaultServerURL, urlSource: "default", tokenSource: "none"}
	if activeName != "" {
		source := fmt.Sprintf("context %q", activeName)
		if active.URL != "" {
			conn.url, conn.urlSource = active.URL, source
		}
		if active.Token != "" {
			conn.token, conn.tokenSource = active.Token, source
		}
	}
	if envURL != "" {
		conn.url, conn.urlSource = envURL, "env"
	}
	if envToken != "" {
		conn.token, conn.tokenSource = envToken, "env"
	}
	if urlSet {
		conn.url, conn.urlSource = flagURL, "flag"
	}
	if tokenSet {
		conn.token, conn.tokenSource = flagToken, "flag"
	}
	return conn, nil
}

// tokenPreview shows enough of a token to identify it (its ldk_ prefix)
// without ever printing the secret itself.
func tokenPreview(token string) string {
	if token == "" {
		return "none"
	}
	if len(token) <= 8 {
		return "****"
	}
	return token[:8] + "..."
}
