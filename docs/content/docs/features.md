This page lists everything LogDeck does, grouped by area. LogDeck is a self-hosted log viewer, alerting tool, and control panel for Docker and Podman, and each section links to its full guide where one exists.

## Real-time log streaming

Container logs stream to the browser over a WebSocket as they are written.

- Live log streaming with automatic updates
- Auto-scroll toggle for following new lines
- Configurable tail size, and pause with a count of buffered lines
- Timestamps you can show or hide
- Both stdout and stderr streams

## Log persistence and history

LogDeck stores logs locally, so the history outlives the container that wrote it.

- LogDeck tails every container on every host into a local SQLite store, enabled by default
- A Live | History toggle in the log viewer searches everything stored, on the server
- History survives restarts, and rebuilds that give a container a new ID, such as `docker compose up --build`
- Containers that no longer exist appear under a Removed filter, with their stored logs still readable
- Retention caps evict the oldest lines first, at 50 MB per container and 1024 MB in total by default
- Aggregated stack logs stay live-only

[Read the guide](/docs/log-history)

## Alerting

LogDeck tells you when a container dies, gets OOM-killed, fails its health check, or starts logging errors.

- Event rules on container death with a non-zero exit, OOM kills, and unhealthy health checks
- Log rules on a minimum level, a regex pattern, or both
- Rate thresholds such as "5 matches in 60 seconds", and per-rule cooldowns that report how many matches they suppressed
- Rules can target hosts, container names, or Compose projects
- Notification channels: a generic JSON webhook that Slack and Discord accept unchanged, ntfy, Gotify, and Telegram
- Alert history with the delivery result of every notification

[Read the guide](/docs/alerting)

## Filtering and search

Search and filters narrow a log view down to the lines you need.

- Full-text search with match navigation, in highlight or exclude mode
- Log level filter: TRACE, DEBUG, INFO, WARN, ERROR, FATAL, PANIC, and unclassified lines
- Regex matching
- Time range presets and a custom calendar range
- Color-coded log levels, collapsible JSON lines, and line pinning

## Log export

Download container logs for offline analysis or to archive them.

- Download the filtered view as JSON or TXT
- Works in both Live and History mode
- Keeps timestamps and log levels
- Copy single lines or a multi-line selection to the clipboard

## Container discovery

LogDeck finds every container on your hosts and keeps its status current.

- Status updates driven by the engine's event stream
- Container details: name, image, status, and uptime
- Health badges (healthy, unhealthy, starting) for containers with a healthcheck, on Docker only, see [engine support](#engine-support-and-caveats)
- Host information: engine version and container count
- System resource usage: CPU and memory
- Grouping by Compose project

## Multi-host management

One LogDeck dashboard works across many hosts and keeps every action scoped to the right daemon.

- Connect local sockets, remote TCP endpoints, or SSH hosts with `DOCKER_HOSTS`
- One container list, with host badges that show where each container runs
- Lifecycle actions, environment variable edits, and log streaming all go to the container's own host
- With no hosts configured, LogDeck auto-detects a local Docker or Podman socket

## Container management

Start, stop, and inspect containers from the UI.

- Start, stop, and restart containers
- Remove containers, with a confirmation step
- Detailed container information
- Environment variables, mounted volumes, exposed ports, labels, and metadata

## Environment variable management

View and change a container's environment variables from the UI.

- Display all environment variables
- Add, edit, and delete variables. Saving recreates the container
- Bulk import from a `.env` file
- Coolify integration syncs changes to Coolify, so they persist across redeployments
- LogDeck detects Coolify-managed containers and labels them in the UI
- Works with Docker Compose setups

## Compose stack tools

Act on a whole Compose stack instead of one container at a time.

- Start, stop, or restart every container in a stack from its group header
- Aggregated stack logs merge all of a stack's containers into one stream, ordered by timestamp
- Color-coded container badges show which container wrote each line
- Works with Docker Compose and podman-compose projects

## Stats and trends

Live resource usage with a short history, across all your hosts.

- Live CPU and memory readings per container
- Sparklines covering the last five minutes
- Per-host engine stats in multi-host setups: CPUs, memory, container counts, and version
- System stats for the machine running LogDeck

## Resource limits and restart policies

Change container resources without recreating or restarting anything.

- Edit memory limits, CPU limits, and restart policies from the container page
- LogDeck applies changes live through the engine's update API, so there is no downtime
- Human-friendly inputs such as `512m` and `1g`, with validation
- Blocked in read-only mode

## Images, volumes, and networks

See what else lives on your hosts besides containers.

- Read-only listings of images, volumes, and networks
- Aggregated across all configured hosts
- Text filtering and per-host error reporting

## Command-line interface

The `logdeck` CLI talks to the server's HTTP API and is built for scripts and AI agents.

- List containers and stacks, inspect containers, read, follow, and search logs, and check stats from the terminal
- `logdeck grep` searches the recent logs of every running container across all hosts
- Lifecycle actions, resource limits, and Compose stack controls
- `logdeck alerts` manages alert rules, notification channels, and alert history
- `logdeck login` saves named contexts, kubectl-style
- Table output for people, and JSON or NDJSON output (`-o json`) for machines

[Read the guide](/docs/cli)

## MCP server

`logdeck mcp` lets an AI assistant query and manage your containers over the Model Context Protocol.

- A stdio MCP server for Claude Desktop, Cursor, Claude Code, and other clients
- Read tools for containers, logs, cross-container search, events, stats, and stored history
- Action tools for lifecycle, removal, one-shot exec, environment variables, and settings
- The API token decides what it can do, with no flags to configure. A read-scoped token cannot change anything, and an admin token has the same reach it has in the UI
- Uses the same HTTP API as the web UI and CLI, so there is no new server and no new auth

[Read the guide](/docs/mcp)

## Web terminal

Open a real shell in any running container from the browser.

- Terminal access over WebSocket
- Full terminal emulation with XTerm.js
- 10,000 lines of scrollback
- Copy to clipboard

## Scoped API tokens

The CLI and other tools get their own credentials, so nobody has to share your login.

- Create and revoke tokens under Settings > API Access
- Tokens start with `ldk_` and are shown only once, at creation
- Two scopes: `admin` for full access and `read` for read-only
- A read token cannot change anything, open the web terminal, read container environment variables, read settings, or read alert rules, channels, and history
- Sent as an `Authorization: Bearer` header on the HTTP API
- Work alongside the JWT sessions the web UI uses

[Read the guide](/docs/configuration#api-tokens)

## Optional authentication

Protect a LogDeck instance with a login, or run it open.

- JWT-based login with a 7-day session token
- Enable it from the Settings page, or pin it with environment variables
- Environment-configured passwords are bcrypt hashes
- Rate-limited login endpoint
- Can be turned off entirely

## Read-only mode

Read-only mode stops LogDeck from changing your containers, while you keep reading their logs.

- Blocks starting, stopping, restarting, and removing containers
- Blocks Compose stack actions, environment and resource edits, and the web terminal
- Toggle it from the Settings page, or pin it with the `READONLY_MODE` environment variable

[Read the guide](/docs/configuration#read-only-mode)

## Interface

- Dark and light mode, following your system preference, with a manual toggle
- Responsive layout for desktop, tablet, and mobile
- Toast notifications that confirm actions
- Keyboard shortcuts, with a cheat sheet on `?`
- Virtualized log lists that handle thousands of lines without slowing down

## Engine support and caveats

LogDeck talks to Podman through its Docker-compatible API socket, rootless or rootful, and can mix Docker and Podman hosts in one multi-host setup. It auto-detects local sockets in this order: Docker, then rootless Podman, then rootful Podman.

Health badges are Docker-only. LogDeck reads a container's health from the engine's container list. Docker includes it there, but Podman's Docker-compatible list API does not, so containers on a Podman host show no health badge even when they define a healthcheck. Everything else works the same on both engines.

## Technical details

- The web frontend is embedded in the Go server binary, so there is one binary to deploy.
- There is no external database. The config file, the SQLite log store, and the alert history all live in one directory, `/data` by default. Mount it as a volume and there is nothing else to manage.
- LogDeck is open source under the GPL-3.0 license.

Have a feature request? [Open an issue on GitHub](https://github.com/AmoabaKelvin/logdeck/issues/new).
