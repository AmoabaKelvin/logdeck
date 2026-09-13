LogDeck is a free, open-source (GPL-3.0), self-hosted log viewer, alerting tool, and control panel for Docker and Podman containers. It stores container logs so they survive restarts, rebuilds, and removal, alerts you when a container dies or starts logging errors, and lets you fix things from the browser, the `logdeck` CLI, or an AI agent over MCP.

The server is a single Go binary with the web frontend embedded, published as the `amoabakelvin/logdeck` Docker image. With Docker Compose it runs in a few minutes.

## What LogDeck does

- Streams container logs to the browser in real time, with automatic scrolling
- Stores logs locally, so you can still read them after a container restarts, gets rebuilt, or is removed
- Alerts you when a container dies, gets OOM-killed, or starts logging errors
- Connects to many Docker or Podman daemons, over a local socket, TCP, or SSH, from one UI
- Searches and filters logs by text, regex, and log level
- Starts, stops, restarts, and removes containers, or whole Compose stacks at once
- Shows live CPU and memory per container, with sparklines of recent history
- Edits memory limits, CPU limits, and restart policies with no container downtime
- Gives you a scriptable CLI that sees everything the UI sees, with JSON output for scripts and AI agents
- Runs an MCP server, so an AI assistant can read your containers and, with an admin token, act on them
- Works out of the box with sensible defaults, and needs no configuration
- Supports optional login sessions and scoped API tokens, or runs completely open

## What LogDeck is not

- It is not a deploy tool. Keep Dockge, Komodo, or plain Compose for shipping containers. LogDeck picks up after `docker compose up -d`.
- It has one admin login, with no multi-user accounts or SSO. Scripts, agents, and teammates get scoped `read` or `admin` API tokens instead.
- It keeps logs on the disk of the host running LogDeck, capped by the retention limits you set.
- Its container health badges are Docker-only. Everything else works the same on Podman.

## Quick start

The fastest way to run LogDeck is Docker Compose:

```yaml
services:
  logdeck:
    image: amoabakelvin/logdeck:latest
    container_name: logdeck
    ports:
      - "8123:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /proc:/host/proc:ro
      # Config file, stored log history, and alert history
      - logdeck-data:/data
    restart: unless-stopped

volumes:
  logdeck-data:
```

Save this as `docker-compose.yml` and run:

```bash
docker compose up -d
```

Then open [http://localhost:8123](http://localhost:8123) in your browser.

> **Keep the `logdeck-data` volume.** It is where LogDeck keeps its config file, its stored log history, and its alert history. Leave it out and all three are lost the next time the container is recreated.

## Next steps

- [Installation](/docs/installation): Docker Compose and `docker run` setups, more hosts, updating, and troubleshooting
- [Features](/docs/features): everything LogDeck can do
- [Log history](/docs/log-history): how stored logs work, and how to tune retention
- [Alerting](/docs/alerting): alerts on container deaths, OOM kills, health checks, and log patterns
- [CLI](/docs/cli): install the `logdeck` command-line client and work from the terminal
- [MCP server](/docs/mcp): let an AI assistant read and manage your containers
- [Configuration](/docs/configuration): environment variables, authentication, API tokens, and reverse proxies

## System requirements

- Docker Engine 20.10 or later, or Podman through its Docker-compatible API socket
- Access to the engine socket, `/var/run/docker.sock` for Docker
- A modern web browser: Chrome, Firefox, Safari, or Edge
- Network access and credentials for any remote hosts, if you manage more than one

## Getting help

If something doesn't work, search the [GitHub issues](https://github.com/AmoabaKelvin/logdeck/issues) for known problems and fixes, or [open a new issue](https://github.com/AmoabaKelvin/logdeck/issues/new).
