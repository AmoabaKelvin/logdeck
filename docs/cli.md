# LogDeck CLI

## Overview

`logdeck` is a command-line client for a running LogDeck server. It talks to the same HTTP API as the web interface, so everything you can see in the UI — containers, logs, stats, events, compose stacks — is available from the terminal.

The CLI is built for scripting and AI agents: it is fully non-interactive, every command supports machine-readable JSON output (`-o json`), errors always go to stderr, and exit codes are consistent (0 success, 1 runtime error, 2 usage error).

## Install

Install the latest release binary (macOS and Linux, amd64/arm64):

```bash
curl -fsSL https://raw.githubusercontent.com/AmoabaKelvin/logdeck/main/install.sh | sh
```

Binaries are published on [GitHub Releases](https://github.com/AmoabaKelvin/logdeck/releases) with checksums; the installer picks the right one for your OS/architecture and installs to `/usr/local/bin` or `~/.local/bin`. Check your version with `logdeck --version`.

Or build from source — the CLI lives in the same Go module as the server and builds to a single static binary:

```bash
cd server
go build ./cmd/logdeck
./logdeck --help
```

## Connection and Authentication

Connect once with `logdeck login` — it verifies the connection (health check, plus an authenticated call when a token is given) and saves it as a named context, kubectl-style. From then on every command uses that context:

```bash
logdeck login --url https://logdeck.example.com --token ldk_... --name prod
logdeck status        # now talks to prod
```

Contexts persist in `~/.config/logdeck/config.json` (respecting `XDG_CONFIG_HOME`). Because the file stores API tokens, it is created with `0600` permissions inside a `0700` directory, and the CLI never prints a saved token — only its `ldk_` prefix.

Manage contexts with:

```bash
logdeck context list          # name, url, token prefix, current marker
logdeck context use staging   # switch the current context
logdeck context rm old        # delete a context
logdeck logout                # remove the token from the current (or named) context, keeping its URL
```

The token is sent as `Authorization: Bearer <token>`. When the server has authentication disabled, no token is needed. On a 401 response the CLI tells you to authenticate with an API token created in LogDeck Settings, via `--token` or `LOGDECK_TOKEN`.

### Resolution order

Flags and environment variables override the saved context — useful for CI or one-off calls. URL and token resolve independently, each from the first source that provides it:

1. Explicit `--url` / `--token` flags
2. `LOGDECK_URL` / `LOGDECK_TOKEN` environment variables
3. The active context from the config file (`--context <name>` selects another saved context for one invocation)
4. Default `http://localhost:8080`

`logdeck status` shows which source supplied the URL and token.

## Output Formats

Every command accepts `-o/--output`:

- `table` (default): compact aligned columns for humans.
- `json`: a single JSON document for one-shot commands; NDJSON (one JSON object per line) for streaming commands (`logs --follow`, `events`).

Timestamps are RFC3339. There are no colors, spinners, prompts, or pagination.

## Commands

### login

Verify a server connection and save it as the current context. Fails with the Settings hint if the server requires authentication and no working token is given.

```bash
logdeck login --url https://logdeck.example.com --token ldk_... --name prod
```

### context

Manage saved contexts: `list`, `use <name>`, `rm <name>`.

```bash
logdeck context list
```

### logout

Remove the saved token from the current (or a named) context, keeping its URL.

```bash
logdeck logout prod
```

### status

Server health, version, and a per-host summary, plus where the connection settings came from (flag, env, or context). The natural first call to discover what a server manages. Exits nonzero if the server is unreachable.

```bash
logdeck status
```

### containers

List containers across all hosts, with optional filters.

```bash
logdeck containers --state running --host prod
```

### stacks

List compose projects (grouped by the `com.docker.compose.project` / `io.podman.compose.project` labels) with container counts and hosts.

```bash
logdeck stacks
```

### inspect

Full inspect data for one container. Table mode shows key facts; `-o json` prints the complete inspect document.

```bash
logdeck inspect web -o json
```

### logs

Read or follow the parsed logs of a container, or a whole compose stack with `--stack`. `--since`/`--until` accept RFC3339 timestamps or relative durations (`30s`, `15m`, `2h`, `1d`).

```bash
logdeck logs web --tail 200 --level ERROR --since 1h
logdeck logs web --follow
logdeck logs --stack myapp --search "timeout" --since 30m
```

Stack logs are merged by timestamp with the container name shown per line. Following a stack is limited to its first 20 containers (the server's per-request aggregate limit); one-shot stack reads batch beyond that automatically.

### grep

Search the recent logs of every running container across all hosts, merged by timestamp. Bounded to the last 15 minutes by default so it stays fast.

```bash
logdeck grep "connection refused" --since 1h --level ERROR
```

### stats

CPU and memory usage for all running containers, or one.

```bash
logdeck stats
logdeck stats web
```

### events

Stream container lifecycle events (start, stop, die, ...). Streams until interrupted, or use `--for` to read for a fixed duration and exit.

```bash
logdeck events --for 30s
```

### start / stop / restart / rm

Container lifecycle actions. Containers are matched by exact name first, then ID prefix; ambiguous matches list the candidates and `--host` disambiguates.

```bash
logdeck restart web
logdeck stop web --host staging
```

### stack

Start, stop, or restart every container of a compose project. Applies to every host that has the project unless `--host` narrows it.

```bash
logdeck stack restart myapp
```

### env

Print a container's environment variables as `KEY=value` lines.

```bash
logdeck env web
```

### resources

Show or update a container's resource limits and restart policy. Memory accepts human units (`512m`, `1.5g`), CPUs accept fractions.

```bash
logdeck resources web
logdeck resources set web --memory 512m --cpus 1.5 --restart on-failure --max-retries 3
```

### images / volumes / networks

Read-only listings across all hosts, with an optional `--host` filter.

```bash
logdeck images --host prod
logdeck volumes
logdeck networks
```

## Using with AI Agents

The CLI is designed so an agent can debug containerized services without a browser. A typical investigation:

```bash
# One-time setup on this machine (or use LOGDECK_URL/LOGDECK_TOKEN in CI)
logdeck login --url https://logdeck.example.com --token ldk_...

# What is running, and is the server healthy?
logdeck status -o json
logdeck containers -o json

# Anything failing right now?
logdeck grep "error|exception|panic" --since 15m -o json

# Zoom into the suspect service
logdeck logs api --tail 500 --level ERROR --since 1h -o json
logdeck inspect api -o json
logdeck stats api -o json

# Act, then confirm
logdeck restart api
logdeck logs api --follow
```

Notes for agents:

- `-o json` always emits a single JSON document on stdout for one-shot commands; streaming commands (`logs --follow`, `events`) emit NDJSON, one object per line.
- Errors go to stderr as `{"error": "..."}` in JSON mode; stdout stays clean for parsing.
- Exit codes: 0 success, 1 runtime/server error, 2 usage error.
- `--since`/`--until` accept relative durations (`15m`, `2h`, `1d`), so no date math is needed.
- `logdeck grep` is the fastest way to find which container is emitting an error across an entire deployment.
