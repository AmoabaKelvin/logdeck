# Podman Setup Guide

## Overview

LogDeck supports Podman through Podman's Docker-compatible REST API. Every
feature — container listing, log streaming, stats, lifecycle actions, events,
the interactive terminal, and environment variable editing — goes through this
API, so no LogDeck-specific configuration is needed beyond pointing it at a
Podman socket.

Podman hosts are configured exactly like Docker hosts (via `DOCKER_HOSTS` or
the Settings UI) and can be freely mixed with Docker hosts in a multi-host
setup. The host connection test in Settings reports which engine it reached,
for example `Connected (Podman 5.3.1)`.

## Prerequisites

- Podman 4.0 or later (earlier versions have incomplete Docker API support)
- The Podman API socket enabled (see below) — unlike Docker, Podman is
  daemonless and does not listen on a socket by default

## Enabling the Podman API Socket

### Rootless (recommended)

Enable the per-user socket:

```bash
systemctl --user enable --now podman.socket
```

The socket is created at `$XDG_RUNTIME_DIR/podman/podman.sock`, typically:

```
/run/user/1000/podman/podman.sock
```

To keep the socket available when you are not logged in, enable lingering for
the user:

```bash
sudo loginctl enable-linger $USER
```

### Rootful

```bash
sudo systemctl enable --now podman.socket
```

The socket is created at `/run/podman/podman.sock`.

### Verify the socket

```bash
curl --unix-socket /run/user/1000/podman/podman.sock http://d/v1.40/_ping
# OK
```

## Local Socket Auto-Detection

When no hosts are configured, LogDeck probes for a local socket in this order
and uses the first one it finds:

1. `/var/run/docker.sock` (Docker)
2. `$XDG_RUNTIME_DIR/podman/podman.sock` (rootless Podman)
3. `/run/podman/podman.sock` (rootful Podman)

On a Podman-only machine with the socket enabled, LogDeck therefore works with
no configuration at all.

## Configuration Examples

Podman sockets use the same `DOCKER_HOSTS` format as Docker (see the
[Multi-Host Setup Guide](./multi-host.md)):

```bash
# Rootless Podman
DOCKER_HOSTS=local=unix:///run/user/1000/podman/podman.sock

# Rootful Podman
DOCKER_HOSTS=local=unix:///run/podman/podman.sock

# Mixed Docker and Podman hosts
DOCKER_HOSTS=docker=unix:///var/run/docker.sock,podman=unix:///run/podman/podman.sock,prod=ssh://deploy@prod.example.com
```

## Running LogDeck as a Container Under Podman

Mount the Podman socket where LogDeck expects the default Docker socket:

```bash
# Rootless
podman run -d \
  --name logdeck \
  -p 8123:8080 \
  -v $XDG_RUNTIME_DIR/podman/podman.sock:/var/run/docker.sock \
  --security-opt label=disable \
  docker.io/amoabakelvin/logdeck:latest

# Rootful
sudo podman run -d \
  --name logdeck \
  -p 8123:8080 \
  -v /run/podman/podman.sock:/var/run/docker.sock \
  --security-opt label=disable \
  docker.io/amoabakelvin/logdeck:latest
```

Notes:

- `--security-opt label=disable` is needed on SELinux systems (Fedora, RHEL,
  CentOS) so the container may use the mounted socket. Alternatively, keep
  labeling enabled and run with `podman run --privileged`, or manage the
  policy explicitly.
- The socket must belong to the same user namespace: mount the rootless
  socket into rootless containers and the rootful socket into rootful ones.

## Remote Podman Hosts over SSH

LogDeck's SSH transport runs `docker system dial-stdio` on the remote host to
reach its socket. For a Podman-only remote host, install the `podman-docker`
package, which provides a `docker` command that transparently invokes Podman:

```bash
# On the remote host
sudo dnf install podman-docker   # Fedora/RHEL
sudo apt install podman-docker   # Debian/Ubuntu

# Enable the socket for the user you SSH in as
systemctl --user enable --now podman.socket
```

Then configure the host as usual:

```bash
DOCKER_HOSTS=remote=ssh://deploy@podman-host.example.com
```

## TCP Connections

Podman can serve its API over TCP, but the endpoint is unauthenticated and
unencrypted — restrict it to trusted networks:

```bash
podman system service --time=0 tcp://0.0.0.0:2375
```

```bash
DOCKER_HOSTS=remote=tcp://podman-host.example.com:2375
```

## macOS (podman machine)

On macOS, Podman runs inside a virtual machine that forwards its API socket to
the host. Find the host-side socket path with:

```bash
podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}'
```

Use that path in your configuration:

```bash
DOCKER_HOSTS=local=unix:///Users/you/.local/share/containers/podman/machine/podman.sock
```

## Compose Project Grouping

Grouping containers by compose project works with both label conventions:

- `com.docker.compose.project` — set by Docker Compose and recent
  podman-compose releases
- `io.podman.compose.project` — set by podman-compose

## Limitations

- The Coolify integration is Docker-specific and does not apply to Podman
  hosts.
- Podman 3.x and earlier are not supported; several endpoints LogDeck relies
  on (stats, events filtering) behave differently there.
