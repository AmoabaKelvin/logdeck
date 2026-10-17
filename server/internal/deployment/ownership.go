// Package deployment describes container ownership shared by platform integrations.
package deployment

import "strings"

// DokployResourceID is the ID of the Dokploy application, Compose deployment
// or database a container was deployed from. Dokploy labels containers with it
// after v0.30.8, and only once a resource is redeployed. A preview carries its
// parent application's ID.
func DokployResourceID(labels map[string]string) string {
	return labels["dokploy.application.id"]
}

// DokployHint identifies metadata that only Dokploy leaves on a container.
// Swarm metadata alone identifies Swarm ownership, not Dokploy ownership.
func DokployHint(labels map[string]string) bool {
	if DokployResourceID(labels) != "" {
		return true
	}
	for _, key := range []string{"com.docker.compose.project.working_dir", "com.docker.compose.project.config_files"} {
		if strings.Contains(labels[key], "/etc/dokploy/") {
			return true
		}
	}
	return false
}

func SwarmManaged(labels map[string]string) bool {
	return labels["com.docker.swarm.service.name"] != "" || labels["com.docker.swarm.service.id"] != ""
}

// SwarmDeploymentNames are the names Dokploy could have deployed a Swarm task
// under: a stack is deployed as its Compose appName, an application or
// database as a service named after its appName.
func SwarmDeploymentNames(labels map[string]string) []string {
	names := make([]string, 0, 2)
	for _, key := range []string{"com.docker.stack.namespace", "com.docker.swarm.service.name"} {
		if labels[key] != "" {
			names = append(names, labels[key])
		}
	}
	return names
}
