// Package deployment describes container ownership shared by platform integrations.
package deployment

import "strings"

// DokployHint identifies metadata left by Dokploy's deployment directories.
// Swarm metadata alone identifies Swarm ownership, not Dokploy ownership.
func DokployHint(labels map[string]string) bool {
	if labels["dokploy.managed"] == "true" {
		return true
	}
	for _, key := range []string{"com.docker.compose.project.working_dir", "com.docker.compose.project.config_files"} {
		if strings.Contains(labels[key], "/dokploy/") {
			return true
		}
	}
	return false
}

func SwarmManaged(labels map[string]string) bool {
	return labels["com.docker.swarm.service.name"] != "" || labels["com.docker.swarm.service.id"] != ""
}
