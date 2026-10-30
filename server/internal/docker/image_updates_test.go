package docker

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/models"
	dockerconfig "github.com/docker/cli/cli/config"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/go-connections/nat"
	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	oldDigest = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	newDigest = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
)

// fakeEngine serves the parts of the Docker API image checks and updates use.
type fakeEngine struct {
	mu         sync.Mutex
	podman     bool
	containers map[string]map[string]any // by container ID
	images     map[string]map[string]any // by image ID or reference
	remote     string                    // the registry's digest for every tag
	pullError  string
	pulledID   string // image ID the tag resolves to after a pull
	calls      []string
	auth       string // X-Registry-Auth of the last distribution inspect
	created    *container.Config
}

func (f *fakeEngine) count(prefix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, call := range f.calls {
		if strings.HasPrefix(call, prefix) {
			n++
		}
	}
	return n
}

func (f *fakeEngine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	path := r.URL.Path
	if i := strings.Index(path[1:], "/"); strings.HasPrefix(path, "/v1.") && i > 0 {
		path = path[i+1:]
	}
	f.calls = append(f.calls, r.Method+" "+path)
	writeJSON := func(v any) { _ = json.NewEncoder(w).Encode(v) }

	switch {
	case path == "/_ping":
		w.Header().Set("API-Version", "1.47")
	case path == "/version":
		components := []map[string]any{{"Name": "Engine"}}
		if f.podman {
			components = []map[string]any{{"Name": "Podman Engine"}}
		}
		writeJSON(map[string]any{"Version": "5.0", "Components": components})
	case strings.HasPrefix(path, "/distribution/"):
		f.auth = r.Header.Get(registry.AuthHeader)
		writeJSON(map[string]any{"Descriptor": map[string]any{"digest": f.remote, "mediaType": ocispec.MediaTypeImageIndex, "size": 1}})
	case path == "/images/create":
		if f.pullError != "" {
			writeJSON(map[string]any{"status": "Pulling"})
			writeJSON(map[string]any{"error": f.pullError})
			return
		}
		writeJSON(map[string]any{"status": "Downloaded newer image"})
		ref := r.URL.Query().Get("fromImage") + ":" + r.URL.Query().Get("tag")
		f.images[strings.TrimPrefix(ref, "docker.io/library/")] = map[string]any{"Id": f.pulledID}
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/images/"):
		img, ok := f.images[strings.TrimSuffix(strings.TrimPrefix(path, "/images/"), "/json")]
		if !ok {
			http.Error(w, `{"message":"no such image"}`, http.StatusNotFound)
			return
		}
		writeJSON(img)
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/containers/"):
		ctr, ok := f.containers[strings.TrimSuffix(strings.TrimPrefix(path, "/containers/"), "/json")]
		if !ok {
			http.Error(w, `{"message":"no such container"}`, http.StatusNotFound)
			return
		}
		writeJSON(ctr)
	case path == "/containers/create":
		f.created = &container.Config{}
		_ = json.NewDecoder(r.Body).Decode(f.created)
		writeJSON(map[string]any{"Id": "replacement"})
	case strings.HasPrefix(path, "/containers/"):
		// stop, rename, start and remove need no answer
	default:
		http.Error(w, "unexpected "+path, http.StatusNotFound)
	}
}

func newFakeEngine(t *testing.T, engine *fakeEngine) *MultiHostClient {
	t.Helper()
	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)
	t.Setenv("DOCKER_API_VERSION", "1.47")
	dockerconfig.SetDir(t.TempDir())
	c, err := NewMultiHostClient([]config.DockerHost{{Name: "local", Host: "tcp://" + strings.TrimPrefix(server.URL, "http://")}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)

	imageChecks.Lock()
	clear(imageChecks.results)
	clear(imageChecks.digests)
	clear(imageChecks.pending)
	imageChecks.Unlock()
	return c
}

func fakeContainer(id, ref, imageID string, labels map[string]string) map[string]any {
	return map[string]any{
		"Id":         id,
		"Name":       "/" + id,
		"Image":      imageID,
		"State":      map[string]any{"Running": true},
		"HostConfig": map[string]any{},
		"Config":     map[string]any{"Image": ref, "Labels": labels, "Env": []string{"PATH=/usr/bin", "VERSION=1", "MINE=yes"}},
	}
}

func TestImageUpdateChecks(t *testing.T) {
	engine := &fakeEngine{
		remote: newDigest,
		containers: map[string]map[string]any{
			"current":  fakeContainer("current", "nginx", "sha256:new", nil),
			"outdated": fakeContainer("outdated", "nginx:latest", "sha256:old", nil),
			"built":    fakeContainer("built", "myapp:dev", "sha256:built", nil),
			"pinned":   fakeContainer("pinned", "nginx@"+newDigest, "sha256:new", nil),
		},
		images: map[string]map[string]any{
			"sha256:new":   {"Id": "sha256:new", "RepoDigests": []string{"nginx@" + newDigest}},
			"sha256:old":   {"Id": "sha256:old", "RepoDigests": []string{"nginx@" + oldDigest}},
			"sha256:built": {"Id": "sha256:built"},
		},
	}
	c := newFakeEngine(t, engine)

	list := []models.ContainerInfo{
		{ID: "current", ImageID: "sha256:new", Host: "local"},
		{ID: "outdated", ImageID: "sha256:old", Host: "local"},
		{ID: "built", ImageID: "sha256:built", Host: "local"},
		{ID: "pinned", ImageID: "sha256:new", Host: "local"},
	}
	c.checkImages("local", list)
	c.AnnotateImageUpdates(list)

	want := map[string]string{
		"current":  models.ImageUpdateCurrent,
		"outdated": models.ImageUpdateAvailable,
		"built":    models.ImageUpdateUnknown,
		"pinned":   models.ImageUpdateUnknown,
	}
	for _, ctr := range list {
		if ctr.ImageUpdate == nil || ctr.ImageUpdate.Status != want[ctr.ID] {
			t.Errorf("%s: image update = %+v, want status %s", ctr.ID, ctr.ImageUpdate, want[ctr.ID])
		}
	}
	// "nginx" and "nginx:latest" are one tag, so one registry call.
	if n := engine.count("GET /distribution/"); n != 1 {
		t.Errorf("registry asked %d times, want once for the shared tag", n)
	}

	// Fresh results come from the cache without touching the engine.
	calls := engine.count("")
	c.AnnotateImageUpdates(list)
	time.Sleep(20 * time.Millisecond)
	if n := engine.count(""); n != calls {
		t.Errorf("annotating fresh results made %d engine calls", n-calls)
	}

	// Checking now asks the registry again.
	engine.mu.Lock()
	engine.remote = oldDigest
	engine.mu.Unlock()
	update, err := c.CheckImageUpdate(t.Context(), "local", "outdated")
	if err != nil || update.Status != models.ImageUpdateCurrent {
		t.Errorf("CheckImageUpdate = %+v, %v; want current after the registry moved back", update, err)
	}
}

func TestImageUpdateChecksStartInTheBackground(t *testing.T) {
	engine := &fakeEngine{
		remote:     newDigest,
		containers: map[string]map[string]any{"web": fakeContainer("web", "nginx", "sha256:old", nil)},
		images:     map[string]map[string]any{"sha256:old": {"Id": "sha256:old", "RepoDigests": []string{"nginx@" + oldDigest}}},
	}
	c := newFakeEngine(t, engine)

	list := []models.ContainerInfo{{ID: "web", ImageID: "sha256:old", Host: "local"}}
	c.AnnotateImageUpdates(list)
	if list[0].ImageUpdate != nil {
		t.Fatal("the first listing must not wait for a check")
	}
	deadline := time.Now().Add(5 * time.Second)
	for list[0].ImageUpdate == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		c.AnnotateImageUpdates(list)
	}
	if list[0].ImageUpdate == nil || list[0].ImageUpdate.Status != models.ImageUpdateAvailable {
		t.Fatalf("image update = %+v, want available", list[0].ImageUpdate)
	}
}

func TestImageUpdateUnsupportedOnPodman(t *testing.T) {
	engine := &fakeEngine{
		podman:     true,
		containers: map[string]map[string]any{"web": fakeContainer("web", "nginx", "sha256:old", nil)},
		images:     map[string]map[string]any{"sha256:old": {"Id": "sha256:old", "RepoDigests": []string{"docker.io/library/nginx@" + oldDigest}}},
	}
	c := newFakeEngine(t, engine)

	update, err := c.CheckImageUpdate(t.Context(), "local", "web")
	if err != nil || update.Status != models.ImageUpdateUnsupported {
		t.Errorf("CheckImageUpdate = %+v, %v; want unsupported", update, err)
	}
	if n := engine.count("GET /distribution/"); n != 0 {
		t.Errorf("Podman was asked for distribution inspect %d times", n)
	}
}

// The daemon holds no registry credentials; LogDeck sends the ones in its own
// Docker config, as the docker CLI does.
func TestImageUpdateSendsRegistryCredentials(t *testing.T) {
	engine := &fakeEngine{
		remote:     newDigest,
		containers: map[string]map[string]any{"web": fakeContainer("web", "ghcr.io/acme/web:1", "sha256:old", nil)},
		images:     map[string]map[string]any{"sha256:old": {"Id": "sha256:old", "RepoDigests": []string{"ghcr.io/acme/web@" + oldDigest}}},
	}
	c := newFakeEngine(t, engine)
	dir := t.TempDir()
	dockerconfig.SetDir(dir)
	auth := base64.StdEncoding.EncodeToString([]byte("robot:secret"))
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"auths":{"ghcr.io":{"auth":"`+auth+`"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := c.CheckImageUpdate(t.Context(), "local", "web"); err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.URLEncoding.DecodeString(engine.auth)
	if err != nil {
		t.Fatalf("registry auth %q: %v", engine.auth, err)
	}
	var sent registry.AuthConfig
	_ = json.Unmarshal(decoded, &sent)
	if sent.Username != "robot" || sent.Password != "secret" {
		t.Errorf("registry auth = %+v, want the config's ghcr.io credentials", sent)
	}
}

func TestUpdateContainerImage(t *testing.T) {
	oldImage := map[string]any{
		"Id":           "sha256:old",
		"Os":           "linux",
		"Architecture": "amd64",
		"RepoDigests":  []string{"nginx@" + oldDigest},
		"Config":       map[string]any{"Env": []string{"PATH=/usr/bin", "VERSION=1"}},
	}

	t.Run("recreates from the new image", func(t *testing.T) {
		engine := &fakeEngine{
			pulledID:   "sha256:new",
			containers: map[string]map[string]any{"web": fakeContainer("web", "nginx", "sha256:old", nil)},
			images:     map[string]map[string]any{"sha256:old": oldImage},
		}
		c := newFakeEngine(t, engine)

		newID, err := c.UpdateContainerImage(t.Context(), "local", "web")
		if err != nil || newID != "replacement" {
			t.Fatalf("UpdateContainerImage = %q, %v", newID, err)
		}
		if engine.created == nil || !reflect.DeepEqual(engine.created.Env, []string{"MINE=yes"}) {
			t.Errorf("replacement env = %v, want only the container's own variables", engine.created)
		}
		if engine.created.Image != "nginx" {
			t.Errorf("replacement image = %q, want the tag", engine.created.Image)
		}
	})

	t.Run("leaves a current container alone", func(t *testing.T) {
		engine := &fakeEngine{
			pulledID:   "sha256:old",
			containers: map[string]map[string]any{"web": fakeContainer("web", "nginx", "sha256:old", nil)},
			images:     map[string]map[string]any{"sha256:old": oldImage},
		}
		c := newFakeEngine(t, engine)

		newID, err := c.UpdateContainerImage(t.Context(), "local", "web")
		if err != nil || newID != "" {
			t.Fatalf("UpdateContainerImage = %q, %v; want no recreate", newID, err)
		}
		if n := engine.count("POST /containers/"); n != 0 {
			t.Errorf("%d container mutations for a current image", n)
		}
	})

	t.Run("reports a failed pull without touching the container", func(t *testing.T) {
		engine := &fakeEngine{
			pullError:  "toomanyrequests: rate limit reached",
			containers: map[string]map[string]any{"web": fakeContainer("web", "nginx", "sha256:old", nil)},
			images:     map[string]map[string]any{"sha256:old": oldImage},
		}
		c := newFakeEngine(t, engine)

		_, err := c.UpdateContainerImage(t.Context(), "local", "web")
		var pullErr *ImagePullError
		if !errors.As(err, &pullErr) || !strings.Contains(err.Error(), "toomanyrequests") {
			t.Fatalf("err = %v, want the pull error", err)
		}
		if n := engine.count("POST /containers/"); n != 0 {
			t.Errorf("%d container mutations after a failed pull", n)
		}
	})

	for name, labels := range map[string]map[string]string{
		"compose": {"com.docker.compose.project": "shop"},
		"coolify": {"coolify.managed": "true"},
		"swarm":   {"com.docker.swarm.service.name": "web"},
		"dokploy": {"dokploy.application.id": "app"},
	} {
		t.Run("refuses "+name, func(t *testing.T) {
			engine := &fakeEngine{
				containers: map[string]map[string]any{"web": fakeContainer("web", "nginx", "sha256:old", labels)},
				images:     map[string]map[string]any{"sha256:old": oldImage},
			}
			c := newFakeEngine(t, engine)

			_, err := c.UpdateContainerImage(t.Context(), "local", "web")
			var refused *ImageUpdateRefusedError
			if !errors.As(err, &refused) {
				t.Fatalf("err = %v, want a refusal", err)
			}
			if n := engine.count("POST "); n != 0 {
				t.Errorf("%d mutations for a %s container", n, name)
			}
		})
	}
}

func TestConfigForImage(t *testing.T) {
	old := &dockerspec.DockerOCIImageConfig{}
	old.Env = []string{"PATH=/usr/bin", "VERSION=1"}
	old.Cmd = []string{"nginx"}
	old.WorkingDir = "/app"
	old.Labels = map[string]string{"version": "1", "vendor": "acme"}
	old.ExposedPorts = map[string]struct{}{"80/tcp": {}}

	got := configForImage(container.Config{
		Image:        "nginx",
		Env:          []string{"PATH=/usr/bin", "VERSION=1", "MINE=yes"},
		Cmd:          []string{"nginx"},
		WorkingDir:   "/app",
		Labels:       map[string]string{"version": "1", "vendor": "me", "team": "web"},
		ExposedPorts: nat.PortSet{"80/tcp": {}, "9000/tcp": {}},
	}, old)

	want := container.Config{
		Image:        "nginx",
		Env:          []string{"MINE=yes"},
		Labels:       map[string]string{"vendor": "me", "team": "web"},
		ExposedPorts: nat.PortSet{"9000/tcp": {}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("configForImage =\n%+v\nwant\n%+v", got, want)
	}

	// A command given with a custom entrypoint was set explicitly.
	custom := configForImage(container.Config{Entrypoint: []string{"/start"}, Cmd: []string{"nginx"}}, old)
	if !reflect.DeepEqual([]string(custom.Cmd), []string{"nginx"}) {
		t.Errorf("Cmd = %v, want it kept beside a custom entrypoint", custom.Cmd)
	}
}
