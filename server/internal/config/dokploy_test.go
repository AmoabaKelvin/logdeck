package config

import (
	"errors"
	"testing"
)

func TestDokployConnectionsMergePersistAndRejectStaleUpdates(t *testing.T) {
	t.Setenv("DOKPLOY_CONFIGS", "local|https://dokploy.test|env-key|")
	fileHost := DokployHostConfig{HostName: "remote", APIURL: "https://dokploy.test", APIToken: "file-key", ServerID: "server"}
	m := newManagerWithFile(t, FileConfig{DokployHosts: []DokployHostConfig{fileHost}})
	if len(m.Config().DokployHosts) != 2 || m.Sources().DokployHosts != SourceMixed {
		t.Fatalf("merged=%#v sources=%#v", m.Config(), m.Sources())
	}
	revision := HostsRevision(m.Config().DokployHosts)
	fileHost.ServerID = "new-server"
	if err := m.UpdateDokployHosts([]DokployHostConfig{fileHost}, revision); err != nil {
		t.Fatal(err)
	}
	if err := m.UpdateDokployHosts(nil, revision); !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("expected stale rejection, got %v", err)
	}
	reloaded := NewManager()
	if reloaded.Config().DokployHosts[1].ServerID != "new-server" {
		t.Fatalf("settings didn't survive restart: %#v", reloaded.Config().DokployHosts)
	}
	if err := m.UpdateDokployHosts([]DokployHostConfig{{HostName: "local"}}, ""); err == nil {
		t.Fatal("environment-owned connection was overwritten")
	}
}
