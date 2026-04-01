package config

import (
	"log"
	"os"
	"strings"
)

type DockerHost struct {
	Name string `json:"name"`
	Host string `json:"host"`
}

type CoolifyHostConfig struct {
	HostName string `json:"hostName"`
	APIURL   string `json:"apiURL"`
	APIToken string `json:"apiToken"`
}

type Config struct {
	ReadOnly     bool
	DockerHosts  []DockerHost
	CoolifyHosts []CoolifyHostConfig
}

func NewConfig() *Config {
	isReadOnlyMode := os.Getenv("READONLY_MODE") == "true"
	dockerHosts := parseDockerHosts()

	// if we don't have any docker hosts, we should default back to
	// the unix socket on the machine running logdeck.
	if len(dockerHosts) == 0 {
		dockerHosts = []DockerHost{{Name: "local", Host: "unix:///var/run/docker.sock"}}
	}

	coolifyHosts := parseCoolifyHostConfigs()

	// Warn if COOLIFY_CONFIGS references host names not in DOCKER_HOSTS
	if len(coolifyHosts) > 0 {
		hostSet := make(map[string]bool, len(dockerHosts))
		for _, dh := range dockerHosts {
			hostSet[dh.Name] = true
		}
		for _, ch := range coolifyHosts {
			if !hostSet[ch.HostName] {
				log.Printf("Warning: COOLIFY_CONFIGS references unknown Docker host %q (not found in DOCKER_HOSTS)", ch.HostName)
			}
		}
	}

	return &Config{ReadOnly: isReadOnlyMode, DockerHosts: dockerHosts, CoolifyHosts: coolifyHosts}
}

func parseDockerHosts() []DockerHost {
	// Format: DOCKER_HOSTS=local=unix:///var/run/docker.sock,remote=ssh://root@X.X.X.X
	dockerHosts := os.Getenv("DOCKER_HOSTS")
	if dockerHosts == "" {
		return []DockerHost{}
	}

	dockerHostsList := []DockerHost{}

	dockerHostStrings := strings.SplitSeq(dockerHosts, ",")
	for dockerHostString := range dockerHostStrings {
		parts := strings.SplitN(strings.TrimSpace(dockerHostString), "=", 2)
		if len(parts) != 2 {
			log.Fatalf("Invalid DOCKER_HOSTS format: %s (expected format: name=host)", dockerHostString)
		}

		name := strings.TrimSpace(parts[0])
		host := strings.TrimSpace(parts[1])
		if name == "" || host == "" {
			log.Fatalf("Invalid DOCKER_HOSTS format: %s (name and host cannot be empty)", dockerHostString)
		}

		dockerHostsList = append(dockerHostsList, DockerHost{Name: name, Host: host})
	}

	return dockerHostsList
}

func parseCoolifyHostConfigs() []CoolifyHostConfig {
	// Format: COOLIFY_CONFIGS=hostA|https://coolify-a.com|tokenA,hostB|https://coolify-b.com|tokenB
	raw := os.Getenv("COOLIFY_CONFIGS")
	if raw == "" {
		return []CoolifyHostConfig{}
	}

	var configs []CoolifyHostConfig
	seen := make(map[string]bool)

	entries := strings.SplitSeq(raw, ",")
	for entry := range entries {
		entry = strings.TrimSpace(entry)
		parts := strings.SplitN(entry, "|", 3)
		if len(parts) != 3 {
			log.Fatalf("Invalid COOLIFY_CONFIGS format: %s (expected format: hostName|apiURL|apiToken)", entry)
		}

		hostName := strings.TrimSpace(parts[0])
		apiURL := strings.TrimSpace(parts[1])
		apiToken := strings.TrimSpace(parts[2])

		if hostName == "" || apiURL == "" || apiToken == "" {
			log.Fatalf("Invalid COOLIFY_CONFIGS format: %s (hostName, apiURL, and apiToken cannot be empty)", entry)
		}

		if seen[hostName] {
			log.Fatalf("Duplicate host name in COOLIFY_CONFIGS: %s", hostName)
		}
		seen[hostName] = true

		apiURL = strings.TrimRight(apiURL, "/")
		configs = append(configs, CoolifyHostConfig{HostName: hostName, APIURL: apiURL, APIToken: apiToken})
	}

	return configs
}
