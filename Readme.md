# LogDeck

Logdeck is an open-source project that aims to be the most intuitive and visually appealing tool for monitoring Docker container logs as well as managing containers.

![LogDeck Landing](./docs/landing.png)
![LogDeck Container View and Logs](./docs/logs.png)

## Features

### Multi-Host Docker Support

LogDeck supports managing Docker containers across multiple Docker hosts simultaneously. Monitor and control containers on local, remote, and SSH-connected Docker daemons from a single unified interface.

Key capabilities:

- Connect to multiple Docker hosts (local Unix socket, remote SSH, TCP)
- Filter and view containers by specific host or across all hosts
- Real-time container state synchronization across all configured hosts
- Secure SSH-based connections with key authentication
- Per-host container operations (start, stop, restart, remove)
- Host-aware log streaming and environment variable management

For detailed configuration instructions and deployment examples, see the [Multi-Host Setup Guide](./multi-host.md).

## Roadmap for v1.0.0

- [ ] Container discovery

  - [ ] Automatically discover all running containers
  - [ ] Host name, docker version
  - [ ] Host system usage (CPU, memory)
  - [ ] Show container name, image, status, uptime
  - [ ] Real-time updates when containers start/stop
  - [ ] Group containers by project, network, label, etc.
  - [ ] View container details (env vars, volumes, ports, labels, etc.)

- [ ] Log viewing

  - [ ] Real-time log streaming (tail -f style)
  - [ ] Historical logs with option for getting X number of lines
  - [ ] Auto-scroll toggle for streaming logs
  - [ ] Timestamps (toggleable)
  - [ ] Color-coded log levels (ERROR, WARN, INFO, DEBUG)
  - [ ] Download logs as file (downloading both the parsed logs as well as the raw logs)
  - [ ] Pause/resume streaming

- [ ] Basic filtering

  - [ ] Search/filter logs by text
  - [ ] Filter by log level
  - [ ] Date range filtering
  - [ ] Regex support (minimal support for now)

- [ ] Container Life Cycle Management

  - [ ] Start, stop, restart, remove containers (Later on this should be a feature flag that can be enabled/disabled, sometimes we might just want a read only view of the containers and their logs)
  - [ ] View container stats (CPU, memory)
