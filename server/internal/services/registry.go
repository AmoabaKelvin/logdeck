package services

import (
	"sync"

	"github.com/AmoabaKelvin/logdeck/internal/auth"
	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/coolify"
	"github.com/AmoabaKelvin/logdeck/internal/docker"
)

// Registry holds all runtime services behind a RWMutex, allowing hot-swap.
type Registry struct {
	mu      sync.RWMutex
	docker  *docker.MultiHostClient
	coolify *coolify.MultiClient
	auth    *auth.Service
	config  *config.Config
}

func NewRegistry(
	dockerClient *docker.MultiHostClient,
	coolifyClient *coolify.MultiClient,
	authService *auth.Service,
	cfg *config.Config,
) *Registry {
	return &Registry{
		docker:  dockerClient,
		coolify: coolifyClient,
		auth:    authService,
		config:  cfg,
	}
}

func (r *Registry) Docker() *docker.MultiHostClient {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.docker
}

func (r *Registry) Coolify() *coolify.MultiClient {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.coolify
}

func (r *Registry) Auth() *auth.Service {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.auth
}

func (r *Registry) Config() *config.Config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

func (r *Registry) SwapDocker(newClient *docker.MultiHostClient) *docker.MultiHostClient {
	r.mu.Lock()
	defer r.mu.Unlock()
	old := r.docker
	r.docker = newClient
	return old
}

func (r *Registry) SwapCoolify(newClient *coolify.MultiClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.coolify = newClient
}

func (r *Registry) SwapAuth(newService *auth.Service) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.auth = newService
}

func (r *Registry) UpdateConfig(cfg *config.Config) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config = cfg
}
