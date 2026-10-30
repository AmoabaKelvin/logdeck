package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/coolify"
	"github.com/AmoabaKelvin/logdeck/internal/deployment"
	"github.com/AmoabaKelvin/logdeck/internal/models"
	"github.com/distribution/reference"
	"github.com/docker/cli/cli/config"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
)

// imageCheckTTL is how long an update check is trusted. Docker Hub counts a
// manifest lookup against its pull rate limit, so a registry is asked about a
// tag at most once per host in this window.
const imageCheckTTL = 6 * time.Hour

// imageChecks caches update checks. results is keyed by host|container
// ID|image ID, so a recreated container is checked again; digests by
// host|image reference, so containers on the same tag share one registry
// call. Package-level so the cache survives client hot-swaps, which happen on
// every settings save.
var imageChecks = struct {
	sync.Mutex
	results map[string]models.ImageUpdate
	digests map[string]registryAnswer
	pending map[string]bool
}{
	results: make(map[string]models.ImageUpdate),
	digests: make(map[string]registryAnswer),
	pending: make(map[string]bool),
}

// imageCheckLocks runs one check at a time per host, so checks on a host
// reuse each other's registry answers and a slow registry holds up only it.
var imageCheckLocks sync.Map

// registryAnswer is the digest a registry gave for a tag, or why it gave none.
type registryAnswer struct {
	digest  string
	status  string
	reason  string
	checked time.Time
}

func imageCheckKey(host, id, imageID string) string {
	return host + "|" + id + "|" + imageID
}

// AnnotateImageUpdates fills in each container's last image check and queues
// a background check for those without a fresh one. It never waits on the
// daemon or a registry.
func (c *MultiHostClient) AnnotateImageUpdates(containers []models.ContainerInfo) {
	now := time.Now()
	stale := make(map[string][]models.ContainerInfo)
	imageChecks.Lock()
	for i := range containers {
		ctr := containers[i]
		key := imageCheckKey(ctr.Host, ctr.ID, ctr.ImageID)
		if update, ok := imageChecks.results[key]; ok {
			containers[i].ImageUpdate = &update
			if now.Sub(time.Unix(update.CheckedAt, 0)) < imageCheckTTL {
				continue
			}
		}
		if !imageChecks.pending[key] {
			imageChecks.pending[key] = true
			stale[ctr.Host] = append(stale[ctr.Host], ctr)
		}
	}
	imageChecks.Unlock()

	for host, ctrs := range stale {
		go c.checkImages(host, ctrs)
	}
}

func (c *MultiHostClient) checkImages(host string, containers []models.ContainerInfo) {
	lock, _ := imageCheckLocks.LoadOrStore(host, &sync.Mutex{})
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()

	pruneImageChecks()
	for _, ctr := range containers {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		// A failed daemon call is not remembered, so the next listing retries.
		_, _ = c.checkImage(ctx, host, ctr.ID, false)
		cancel()
		imageChecks.Lock()
		delete(imageChecks.pending, imageCheckKey(host, ctr.ID, ctr.ImageID))
		imageChecks.Unlock()
	}
}

// pruneImageChecks drops expired entries, which is all that is left of
// removed containers and images no longer run.
func pruneImageChecks() {
	imageChecks.Lock()
	defer imageChecks.Unlock()
	for key, update := range imageChecks.results {
		if time.Since(time.Unix(update.CheckedAt, 0)) >= imageCheckTTL && !imageChecks.pending[key] {
			delete(imageChecks.results, key)
		}
	}
	for key, answer := range imageChecks.digests {
		if time.Since(answer.checked) >= imageCheckTTL {
			delete(imageChecks.digests, key)
		}
	}
}

// CheckImageUpdate checks one container's image now, asking the registry
// again even if it answered recently.
func (c *MultiHostClient) CheckImageUpdate(ctx context.Context, hostName, id string) (models.ImageUpdate, error) {
	return c.checkImage(ctx, hostName, id, true)
}

// checkImage compares the digest the registry has for a container's tag with
// the digests its image was pulled by, and remembers the result.
func (c *MultiHostClient) checkImage(ctx context.Context, hostName, id string, force bool) (models.ImageUpdate, error) {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return models.ImageUpdate{}, err
	}
	inspect, err := apiClient.ContainerInspect(ctx, id)
	if err != nil {
		return models.ImageUpdate{}, err
	}
	if inspect.Config == nil {
		return models.ImageUpdate{}, fmt.Errorf("container configuration is unavailable")
	}

	update := models.ImageUpdate{Status: models.ImageUpdateUnknown, CheckedAt: time.Now().Unix()}
	named, reason := registryRef(inspect.Config.Image)
	if named == nil {
		update.Reason = reason
	} else {
		img, err := apiClient.ImageInspect(ctx, inspect.Image)
		if err != nil {
			return models.ImageUpdate{}, err
		}
		pulled := repoDigests(img.RepoDigests, named)
		if len(pulled) == 0 {
			update.Reason = "The image was not pulled from " + reference.FamiliarName(named) + ", so it was likely built or loaded locally"
		} else {
			answer, err := c.registryDigest(ctx, apiClient, hostName, named, force)
			if err != nil {
				return models.ImageUpdate{}, err
			}
			switch {
			case answer.digest == "":
				update.Status, update.Reason = answer.status, answer.reason
			case slices.Contains(pulled, answer.digest):
				update.Status = models.ImageUpdateCurrent
			default:
				update.Status = models.ImageUpdateAvailable
			}
		}
	}

	imageChecks.Lock()
	imageChecks.results[imageCheckKey(hostName, inspect.ID, inspect.Image)] = update
	imageChecks.Unlock()
	return update, nil
}

// registryRef parses the tag a container was created from, or says why it
// has none that could move.
func registryRef(ref string) (reference.Named, string) {
	if strings.HasPrefix(ref, "sha256:") {
		return nil, "The container was created from an image ID, not a tag"
	}
	named, err := reference.ParseNormalizedNamed(ref)
	if err != nil {
		return nil, "The container's image is not a registry reference"
	}
	if _, ok := named.(reference.Digested); ok {
		return nil, "The container is pinned to an image digest"
	}
	return reference.TagNameOnly(named), ""
}

// repoDigests returns the digests a local image was pulled by from named's
// repository. Locally built images have none.
func repoDigests(repoDigests []string, named reference.Named) []string {
	var digests []string
	for _, repoDigest := range repoDigests {
		parsed, err := reference.ParseNormalizedNamed(repoDigest)
		if err != nil {
			continue
		}
		if canonical, ok := parsed.(reference.Canonical); ok && canonical.Name() == named.Name() {
			digests = append(digests, canonical.Digest().String())
		}
	}
	return digests
}

// registryDigest returns the digest the registry has for a tag now, from the
// cache while it is fresh. An error means the daemon could not be asked; that
// is not cached.
func (c *MultiHostClient) registryDigest(ctx context.Context, apiClient *client.Client, hostName string, named reference.Named, force bool) (registryAnswer, error) {
	key := hostName + "|" + named.String()
	imageChecks.Lock()
	answer, ok := imageChecks.digests[key]
	imageChecks.Unlock()
	if ok && !force && time.Since(answer.checked) < imageCheckTTL {
		return answer, nil
	}

	engine, _, err := c.EngineInfo(ctx, hostName)
	if err != nil {
		return registryAnswer{}, err
	}
	answer = registryAnswer{status: models.ImageUpdateUnknown, checked: time.Now()}
	if engine == "Podman" {
		// Podman's Docker-compatible API answers distribution inspect with
		// "not implemented".
		answer.status = models.ImageUpdateUnsupported
		answer.reason = "Podman cannot look up registry digests through its Docker-compatible API"
	} else {
		dist, err := apiClient.DistributionInspect(ctx, reference.FamiliarString(named), registryAuth(named))
		if client.IsErrConnectionFailed(err) {
			return registryAnswer{}, err
		}
		if err != nil {
			answer.reason = "The registry could not be checked: " + err.Error()
		} else {
			answer.digest = dist.Descriptor.Digest.String()
		}
	}

	imageChecks.Lock()
	imageChecks.digests[key] = answer
	imageChecks.Unlock()
	return answer, nil
}

// registryAuth returns the credentials LogDeck's own Docker config
// ($DOCKER_CONFIG or ~/.docker/config.json, with any credential helpers) holds
// for the image's registry. The Docker daemon keeps none: the docker CLI sends
// them with each pull, so LogDeck has to as well. Without any, the registry is
// asked anonymously.
func registryAuth(named reference.Named) string {
	server := reference.Domain(named)
	if server == "docker.io" {
		server = "https://index.docker.io/v1/"
	}
	cfg, err := config.Load(config.Dir())
	if err != nil {
		return ""
	}
	auth, err := cfg.GetAuthConfig(server)
	if err != nil || (auth.Username == "" && auth.IdentityToken == "" && auth.RegistryToken == "") {
		return ""
	}
	encoded, err := registry.EncodeAuthConfig(registry.AuthConfig{
		Username:      auth.Username,
		Password:      auth.Password,
		ServerAddress: auth.ServerAddress,
		IdentityToken: auth.IdentityToken,
		RegistryToken: auth.RegistryToken,
	})
	if err != nil {
		return ""
	}
	return encoded
}

// ImageUpdateRefusedError refuses an image update that the container's
// owner has to make, or that has no tag to pull.
type ImageUpdateRefusedError struct{ Reason string }

func (e *ImageUpdateRefusedError) Error() string { return e.Reason }

// ImagePullError is a pull that failed before the container was touched.
type ImagePullError struct{ Err error }

func (e *ImagePullError) Error() string { return "pulling the image failed: " + e.Err.Error() }
func (e *ImagePullError) Unwrap() error { return e.Err }

// imageUpdateOwner refuses containers whose owner would undo or fight a
// direct recreate. Platforms deploy them; Compose recreates them itself.
func imageUpdateOwner(labels map[string]string) error {
	if err := checkNotSystemdManaged(labels, "update"); err != nil {
		return err
	}
	switch {
	case labels[coolify.LabelManaged] == "true":
		return &ImageUpdateRefusedError{"this container is managed by Coolify; deploy it in Coolify to pull the new image"}
	case deployment.DokployHint(labels):
		return &ImageUpdateRefusedError{"this container is managed by Dokploy; deploy it in Dokploy to pull the new image"}
	case deployment.SwarmManaged(labels):
		return &ImageUpdateRefusedError{"this container is a Swarm task; update its service instead, for example with \"docker service update --image\""}
	case ComposeProject(labels) != "":
		return &ImageUpdateRefusedError{fmt.Sprintf("this container belongs to the Compose project %s; update it with \"docker compose pull\" and \"docker compose up -d\" in the project's directory", ComposeProject(labels))}
	}
	return nil
}

// UpdateContainerImage pulls the newest image for a standalone container's
// tag and, if it changed, recreates the container from it with the same
// settings. It returns the new container ID, or "" when the container already
// runs the newest image.
func (c *MultiHostClient) UpdateContainerImage(ctx context.Context, hostName, id string) (string, error) {
	apiClient, err := c.GetClient(hostName)
	if err != nil {
		return "", err
	}

	lock := envEditLock(hostName, id)
	lock.Lock()
	defer lock.Unlock()

	inspect, err := apiClient.ContainerInspect(ctx, id)
	if err != nil {
		return "", err
	}
	if inspect.Config == nil {
		return "", fmt.Errorf("container configuration is unavailable")
	}
	if err := imageUpdateOwner(inspect.Config.Labels); err != nil {
		return "", err
	}
	named, reason := registryRef(inspect.Config.Image)
	if named == nil {
		return "", &ImageUpdateRefusedError{reason}
	}
	ref := reference.FamiliarString(named)
	current, err := apiClient.ImageInspect(ctx, inspect.Image)
	if err != nil {
		return "", err
	}

	// Pull for the platform the container runs, which need not be the host's.
	var platform string
	if current.Os != "" && current.Architecture != "" {
		platform = current.Os + "/" + current.Architecture
		if current.Variant != "" {
			platform += "/" + current.Variant
		}
	}
	pull, err := apiClient.ImagePull(ctx, ref, image.PullOptions{RegistryAuth: registryAuth(named), Platform: platform})
	if err != nil {
		return "", &ImagePullError{err}
	}
	err = readPullStream(pull)
	_ = pull.Close()
	if err != nil {
		return "", &ImagePullError{err}
	}

	pulled, err := apiClient.ImageInspect(ctx, ref)
	if err != nil {
		return "", err
	}
	if pulled.ID == inspect.Image {
		return "", nil
	}

	config := configForImage(*inspect.Config, current.Config)
	c.stops.Store(hostName+"|"+inspect.ID, time.Now())
	newID, err := recreateContainer(ctx, apiClient, inspect, &config)
	if err != nil {
		return "", fmt.Errorf("the new image was pulled, but recreating the container failed, so the original was kept: %w", err)
	}
	return newID, nil
}

// readPullStream waits for a pull to finish. The daemon reports a failed pull
// inside its progress stream, after answering 200.
func readPullStream(stream io.Reader) error {
	decoder := json.NewDecoder(stream)
	for {
		var message struct {
			Error string `json:"error"`
		}
		if err := decoder.Decode(&message); err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		if message.Error != "" {
			return errors.New(message.Error)
		}
	}
}

// configForImage carries a container's config over to a new image. Inspect
// merges the old image's defaults (env, labels, command, ...) into the config;
// those are dropped so the new image's own defaults apply. Values the user
// set to the same as the old image's default go with them.
func configForImage(cfg container.Config, old *dockerspec.DockerOCIImageConfig) container.Config {
	if old == nil {
		return cfg
	}
	cfg.Env = slices.DeleteFunc(slices.Clone(cfg.Env), func(env string) bool {
		return slices.Contains(old.Env, env)
	})
	// Without an entrypoint of its own, the container's command is the
	// image's unless it was set; with one, the command was set explicitly.
	if slices.Equal(cfg.Entrypoint, old.Entrypoint) {
		cfg.Entrypoint = nil
		if slices.Equal(cfg.Cmd, old.Cmd) {
			cfg.Cmd = nil
		}
	}
	if cfg.WorkingDir == old.WorkingDir {
		cfg.WorkingDir = ""
	}
	if cfg.User == old.User {
		cfg.User = ""
	}
	if cfg.StopSignal == old.StopSignal {
		cfg.StopSignal = ""
	}
	if reflect.DeepEqual(cfg.Healthcheck, old.Healthcheck) {
		cfg.Healthcheck = nil
	}
	cfg.Labels = maps.Clone(cfg.Labels)
	maps.DeleteFunc(cfg.Labels, func(key, value string) bool {
		imageValue, ok := old.Labels[key]
		return ok && imageValue == value
	})
	cfg.ExposedPorts = maps.Clone(cfg.ExposedPorts)
	maps.DeleteFunc(cfg.ExposedPorts, func(port nat.Port, _ struct{}) bool {
		_, ok := old.ExposedPorts[string(port)]
		return ok
	})
	cfg.Volumes = maps.Clone(cfg.Volumes)
	maps.DeleteFunc(cfg.Volumes, func(path string, _ struct{}) bool {
		_, ok := old.Volumes[path]
		return ok
	})
	return cfg
}
