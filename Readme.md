# LogDeck

**The self-hosted control plane for Docker & Podman.** Live logs that survive redeploys, full container management, and an MCP server so your AI assistant can read logs and act on your containers, all in a single Go binary with no external dependencies.

Point it at your $5 VPS, or a whole fleet of hosts, and manage everything running on it from one place. One tool in place of the usual "logs + metrics + management" stack, on Docker *and* Podman.

![LogDeck Dashboard](./docs/landing.png)
![LogDeck Container View and Logs](./docs/logs.png)

## Quick start

```bash
docker run -d --name logdeck \
  -p 8080:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v logdeck-data:/data \
  amoabakelvin/logdeck:latest
```

Open `http://localhost:8080`. The `logdeck-data` volume holds your config, stored logs, and alert history, so keep it or you lose all of that on recreate. For multi-host, SSH hosts, auth, and host system stats, use the [docker-compose.yml](./docker-compose.yml).

## Why LogDeck

Most self-hosters end up bolting together a log viewer, a metrics tool, and a management UI. LogDeck is all of that in one binary, and three things set it apart:

- **Your logs don't vanish on redeploy.** Every container is tailed into a local store, so history survives the `docker compose up --build` that gives a container a new ID, and removed containers stay readable. A plain live viewer only shows you what the engine still holds.
- **An AI copilot, built in.** The MCP server ships inside the same free binary, so Claude or Cursor can read your logs and act on your containers, with nothing to bolt on and nothing hosted elsewhere.
- **One free, open binary.** Logs, container and Compose management, alerting, stats, and multi-host support for Docker and Podman, all GPLv3, with no open-core paywall and no external services. Small enough for the $5 VPS it's built for.

## Features

### AI & MCP

`logdeck mcp` runs a [Model Context Protocol](https://modelcontextprotocol.io) server over stdio, so an assistant like Claude Desktop, Cursor, or Claude Code can work with your containers directly:

- Read logs, search across hosts, inspect containers, and check events and stats
- Take action: start/stop/restart, run a command, edit environment variables, manage settings
- Capability follows your API token: hand it a read token and it can only look; hand it an admin token and it can act.

See the [MCP guide](https://logdeck.dev/docs/mcp).

### Logs & History

Real-time streaming with the search and persistence a `tail -f` never gives you:

- Live streaming (tail -f style) with play/pause, auto-scroll, toggleable timestamps and wrapping, and JSON/TXT download
- Full-text search with highlight/exclude modes and match navigation, log-level filtering (TRACE→PANIC), time-range presets, collapsible JSON lines, and line pinning
- **Persistent history.** Every container on every host is tailed into a local SQLite store, so history outlives the container that produced it. It survives restarts and rebuilds that change the container ID (`docker compose up --build`), and removed containers stay readable under a **Removed** filter. A **Live | History** toggle searches everything stored, server-side. Retention caps (50 MB/container, 1024 MB total by default) evict oldest-first.

See the [Log History guide](https://logdeck.dev/docs/log-history).

### Container & Compose Management

- Start, stop, restart, and remove containers, with confirmation on destructive actions and health badges
- Whole-stack Compose controls (start/stop/restart from the group header) and aggregated stack logs merged by timestamp with color-coded container badges, working with Docker Compose and podman-compose
- Interactive WebSocket terminal (XTerm.js, 10k-line scrollback, copy-to-clipboard)
- View and edit environment variables, with bulk import from `.env` files
- Edit memory limits, CPU limits, and restart policies live via the engine's update API, with no recreate and no downtime
- Read-only mode for monitoring-only deployments

### Alerting & Stats

- Event rules on container death (non-zero exit) and OOM kills; log rules on level, regex, or both
- Rate thresholds ("5 matches in 60 seconds") with per-rule cooldowns that report suppressed counts, targeted by host, container name, or Compose project
- One JSON webhook that Slack and Discord incoming webhooks accept unchanged, plus alert history with every delivery result
- Live CPU/memory per container with five-minute sparkline trends, per-host engine stats, and system stats for the machine running LogDeck

See the [Alerting guide](https://logdeck.dev/docs/alerting).

### Multi-Host, Docker & Podman

- Connect to local Unix sockets, remote SSH (key auth), or TCP endpoints; filter and view by host or across all hosts, with real-time state sync
- Works with Podman via its Docker-compatible API: automatic detection of local Docker and Podman sockets (rootless and rootful); logs, stats, lifecycle, events, and the terminal all work unchanged; mix Docker and Podman hosts in one setup
- Read-only views of images, volumes, and networks across all hosts
- Automatic container discovery, Compose-project grouping, state/name/image filtering, and shareable URL state

For setup, see the [Multi-Host](./multi-host.md) and [Podman](./podman.md) guides. (Health badges are Docker-only, since Podman's list API doesn't embed health.)

### Command-Line Interface

A scriptable `logdeck` CLI over the server's HTTP API, built for automation and agents:

- List, inspect, read/follow/search logs, and check stats from the terminal
- `logdeck grep` searches the recent logs of every running container across all hosts
- Lifecycle actions, resource limits, Compose controls, and alert management (`logdeck alerts`)
- Table output for humans, JSON/NDJSON (`-o json`) for machines

See the [CLI Guide](./docs/cli.md).

### Authentication & Security

- Optional authentication: enable it from Settings, pin it with environment variables, or run open. JWT session login with a rate-limited endpoint.
- **Scoped API tokens** (`ldk_...`) for the CLI, MCP, and external tools: `admin` (full access) or `read` (logs, history, stats, events, and details, with no mutations, terminal, env, or settings)
- Read-only mode blocks container/stack actions, env and resource edits, and the terminal

### Configuration & Storage

LogDeck reads environment variables and a JSON config file that the Settings page writes; environment variables win and pin the value so the UI cannot change it. Everything it persists lives in one directory (`/data` by default, `CONFIG_PATH` to move it): `config.json`, `logs.db` (the SQLite log store), and `alerts-history.json`. **Mount it as a volume** or all of it is lost when the container is recreated.

See the [Configuration guide](https://logdeck.dev/docs/configuration) for every environment variable.
