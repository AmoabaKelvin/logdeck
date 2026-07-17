package config

import (
	"reflect"
	"testing"
)

func TestParseDockerHosts(t *testing.T) {
	// Whitespace around entries, names, and hosts is trimmed; multiple hosts
	// split on commas.
	t.Setenv("DOCKER_HOSTS", " local = unix:///var/run/docker.sock , remote=ssh://root@10.0.0.1 ")

	got := parseDockerHosts()
	want := []DockerHost{
		{Name: "local", Host: "unix:///var/run/docker.sock"},
		{Name: "remote", Host: "ssh://root@10.0.0.1"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseDockerHosts() = %+v, want %+v", got, want)
	}
}

func TestParseDockerHostsHostMayContainEquals(t *testing.T) {
	// SplitN on "=" with limit 2 keeps an "=" inside the host value intact.
	t.Setenv("DOCKER_HOSTS", "local=tcp://host:2375?opt=a=b")

	got := parseDockerHosts()
	want := []DockerHost{{Name: "local", Host: "tcp://host:2375?opt=a=b"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseDockerHosts() = %+v, want %+v", got, want)
	}
}

func TestParseDockerHostsEmpty(t *testing.T) {
	t.Setenv("DOCKER_HOSTS", "")
	if got := parseDockerHosts(); len(got) != 0 {
		t.Fatalf("expected no hosts when DOCKER_HOSTS is unset, got %+v", got)
	}
}

func TestParseCoolifyHostConfigs(t *testing.T) {
	// Trailing slashes on the API URL are trimmed; fields and entries are
	// whitespace-trimmed.
	t.Setenv("COOLIFY_CONFIGS", " a | https://coolify-a.com/ | tokenA , b|https://coolify-b.com|tokenB ")

	got := parseCoolifyHostConfigs()
	want := []CoolifyHostConfig{
		{HostName: "a", APIURL: "https://coolify-a.com", APIToken: "tokenA"},
		{HostName: "b", APIURL: "https://coolify-b.com", APIToken: "tokenB"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseCoolifyHostConfigs() = %+v, want %+v", got, want)
	}
}

func TestParseCoolifyHostConfigsEmpty(t *testing.T) {
	t.Setenv("COOLIFY_CONFIGS", "")
	if got := parseCoolifyHostConfigs(); len(got) != 0 {
		t.Fatalf("expected no configs when COOLIFY_CONFIGS is unset, got %+v", got)
	}
}
