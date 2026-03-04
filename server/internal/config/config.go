package config

import (
	"log"
	"os"
	"strings"
)

type DockerHost struct {
	Name string
	Host string
}

type CoolifyConfig struct {
	APIURL   string
	APIToken string
}

type Config struct {
	ReadOnly    bool
	DockerHosts []DockerHost
	Coolify     *CoolifyConfig
}

func NewConfig() *Config {
	isReadOnlyMode := os.Getenv("READONLY_MODE") == "true"
	dockerHosts := parseDockerHosts()

	// if we don't have any docker hosts, we should default back to
	// the unix socket on the machine running logdeck.
	if len(dockerHosts) == 0 {
		dockerHosts = []DockerHost{{Name: "local", Host: "unix:///var/run/docker.sock"}}
	}

	coolify := parseCoolifyConfig()

	return &Config{ReadOnly: isReadOnlyMode, DockerHosts: dockerHosts, Coolify: coolify}
}

func parseCoolifyConfig() *CoolifyConfig {
	apiURL := strings.TrimSpace(os.Getenv("COOLIFY_API_URL"))
	apiToken := strings.TrimSpace(os.Getenv("COOLIFY_API_TOKEN"))

	if apiURL == "" && apiToken == "" {
		return nil
	}

	if apiURL == "" || apiToken == "" {
		log.Fatalf("Partial Coolify configuration detected. Both COOLIFY_API_URL and COOLIFY_API_TOKEN must be set together.")
	}

	// Remove trailing slash from API URL
	apiURL = strings.TrimRight(apiURL, "/")

	return &CoolifyConfig{APIURL: apiURL, APIToken: apiToken}
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
