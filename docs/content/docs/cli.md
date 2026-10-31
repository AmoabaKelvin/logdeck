`logdeck` is the command-line client for a running LogDeck server. It talks to the same HTTP API as the web interface, so the containers, logs, stats, events, and Compose stacks you see in the UI are all available from the terminal.

It is fully non-interactive and built for scripting and AI agents. Every command supports machine-readable JSON output (`-o json`), errors always go to stderr, and exit codes are consistent: 0 for success, 1 for a runtime error, 2 for a usage error.

## Install

Install the latest release binary (macOS and Linux, amd64/arm64):

```bash
curl -fsSL https://raw.githubusercontent.com/AmoabaKelvin/logdeck/main/install.sh | sh
```

Binaries ship on [GitHub Releases](https://github.com/AmoabaKelvin/logdeck/releases) with checksums. The installer picks the right one for your OS and architecture, verifies it against the release's `checksums.txt`, and installs it to `/usr/local/bin` or `~/.local/bin`. Check your version with `logdeck --version`.

Or build from source. The CLI lives in the same Go module as the server and builds to a single static binary:

```bash
cd server
go build ./cmd/logdeck
./logdeck --help
```

## Connection and authentication

Connect once with `logdeck login`. It verifies the connection (a health check, plus an authenticated call when you pass a token) and saves it as a named context, kubectl-style. From then on every command uses that context:

```bash
logdeck login --url https://logdeck.example.com --token ldk_... --name prod
logdeck status        # now talks to prod
```

Contexts persist in `~/.config/logdeck/config.json` (respecting `XDG_CONFIG_HOME`). The file stores API tokens, so the CLI creates it with `0600` permissions inside a `0700` directory. It never prints a saved token, only its `ldk_` prefix.

Manage contexts with:

```bash
logdeck context list          # name, url, token prefix, current marker
logdeck context use staging   # switch the current context
logdeck context rm old        # delete a context
logdeck logout                # remove the token from the current (or named) context, keeping its URL
```

Create API tokens in the LogDeck web UI under **Settings > API Access**. The CLI sends them as `Authorization: Bearer <token>`. Tokens have a scope. **Admin** tokens have full access. **Read-only** tokens can read logs, stats, container details, and events, but cannot mutate anything, use the web terminal, or read container environment variables or settings. That makes them a good fit for CI jobs and AI agents that only need to read. When the server has authentication disabled, no token is needed.

On a 401 response, the CLI tells you to authenticate with an API token created in LogDeck Settings, passed with `--token` or `LOGDECK_TOKEN`.

### Resolution order

Flags and environment variables override the saved context, which helps in CI or for one-off calls. The URL and the token resolve independently, each from the first source that provides it:

1. Explicit `--url` / `--token` flags
2. `LOGDECK_URL` / `LOGDECK_TOKEN` environment variables
3. The active context from the config file (`--context <name>` selects another saved context for one invocation)
4. Default `http://localhost:8080`

`logdeck status` shows which source supplied the URL and the token.

## Output formats

Every command accepts `-o/--output`:

- `table` (default): compact aligned columns for humans.
- `json`: a single JSON document for one-shot commands, and NDJSON (one JSON object per line) for streaming commands (`logs --follow`, `events`).

Timestamps are RFC3339. There are no colors, spinners, prompts, or pagination.

## Commands

### `login`

Verify a server connection and save it as the current context. If the server requires authentication and you give no working token, it fails with a hint pointing at Settings.

```bash
logdeck login --url https://logdeck.example.com --token ldk_... --name prod
```

### `context`

Manage saved contexts: `list`, `use <name>`, `rm <name>`.

```bash
logdeck context list
```

### `logout`

Remove the saved token from the current (or a named) context, keeping its URL.

```bash
logdeck logout prod
```

### `status`

Server health, version, and a per-host summary, plus where the connection settings came from (flag, env, or context). Run it first to discover what a server manages. Exits nonzero if the server is unreachable.

```bash
logdeck status
```

### `containers`

List containers across all hosts, with optional filters.

```bash
logdeck containers --state running --host prod
```

### `stacks`

List Compose projects, grouped by the `com.docker.compose.project` / `io.podman.compose.project` labels, with container counts and hosts.

```bash
logdeck stacks
```

### `inspect`

Full inspect data for one container. Table mode shows key facts. `-o json` prints the complete inspect document.

```bash
logdeck inspect web -o json
```

### `logs`

Read or follow the parsed logs of a container, or of a whole Compose stack with `--stack`. `--since` and `--until` accept RFC3339 timestamps or relative durations (`30s`, `15m`, `2h`, `1d`). Stack logs merge by timestamp and show the container name on each line. Following a stack covers its first 20 containers, the server's per-request aggregate limit. One-shot stack reads batch beyond that automatically.

```bash
logdeck logs web --tail 200 --level ERROR --since 1h
logdeck logs web --follow
logdeck logs --stack myapp --search "timeout" --since 30m
logdeck logs api --where level=error --where user_id=42
```

`--where` filters JSON and logfmt lines by field: `key=value` or `key!=value`, with nested JSON keys as `a.b`. Repeat it to add terms; every term must match. See [field filters](/docs/features#field-filters).

### `grep`

Search the recent logs of every running container across all hosts, merged by timestamp. It looks at the last 15 minutes by default so it stays fast.

```bash
logdeck grep "connection refused" --since 1h --level ERROR
logdeck grep --where http.status=500 --since 1h
```

`grep` takes the same `--where` as `logs`. With `--where`, the regex is optional.

An empty result still exits 0. Stdout stays empty, so pipelines stay clean, and a one-line hint goes to stderr: `no running containers to search` or `no matches in <N> containers since <time>`. `<N>` counts only the containers that were actually searched.

When some containers' logs cannot be read, `grep` and `logs --stack` still print what they could read and add one warning per missing container to stderr: `warning: incomplete results: logs of <name> on <host> could not be read: <reason>`. The exit code stays 0. In JSON mode the document also carries `sources`, with each container's status (`ok`, `no_matches`, `removed`, or `failed` with an `error`), and `incomplete: true`. Check that field before treating an empty result as "no matches". If no container could be read at all, the command fails with exit code 1.

### `stats`

CPU and memory usage for all running containers, or for one.

```bash
logdeck stats
logdeck stats web
```

### `events`

Stream container lifecycle events (start, stop, die, and so on). Streams until interrupted, or use `--for` to read for a fixed duration and exit.

```bash
logdeck events --for 30s
```

### `start / stop / restart / rm`

Container lifecycle actions. The CLI matches containers by exact name first, then by ID prefix. Ambiguous matches list the candidates, and `--host` disambiguates.

```bash
logdeck restart web
logdeck stop web --host staging
```

### `stack`

Start, stop, or restart every container of a Compose project. Applies to every host that has the project unless `--host` narrows it.

```bash
logdeck stack restart myapp
```

### `env`

Print a container's environment variables as `KEY=value` lines. For Coolify-managed applications and services, read the saved Coolify configuration. Each line includes its production or preview scope, and hidden values print as `<unknown>`. JSON output includes the saved records and their settings.

```bash
logdeck env web
```

### `resources`

Show or update a container's resource limits and restart policy. Memory accepts human units (`512m`, `1.5g`). CPUs accept fractions. For a Coolify application, `set` saves the limits in Coolify without applying them; deploy from the container page.

```bash
logdeck resources web
logdeck resources set web --memory 512m --cpus 1.5 --restart on-failure --max-retries 3
```

### `images / volumes / networks`

Read-only listings across all hosts, with an optional `--host` filter.

```bash
logdeck images --host prod
logdeck volumes
logdeck networks
```

### `alerts`

Manage alerting: rules, notification channels, and fired-alert history. Rules match container events (`die`, `oom`, `unhealthy`, `restart`), log lines (a minimum level, a regex, or both), or CPU and memory use held above a limit (`--type resource --metric cpu|memory --above 90%|1.5g`), and can require a threshold of matches within a window. LogDeck delivers every fired alert to each enabled channel. Channel types are `webhook` (a generic JSON POST that Slack and Discord incoming webhooks accept), `slack`, `discord`, `email`, `pagerduty`, `opsgenie`, `ntfy`, `gotify`, and `telegram`.

`--host`, `--container`, and `--project` are repeatable and narrow which containers a rule watches. A rule without them watches everything. `--window` and `--cooldown` accept durations (`60s`, `5m`) or bare seconds. When `--cooldown` is 0 or omitted, the server applies its default of 300 seconds between deliveries for the same rule and container.

```bash
logdeck alerts rules
logdeck alerts rules create --type event --name oom-watch --events oom
logdeck alerts rules create --type log --name errors --min-level ERROR --threshold 5 --window 60s
logdeck alerts rules create --type event --name restart-loop --events restart --threshold 3 --window 10m
logdeck alerts rules create --type resource --name memory --metric memory --above 90% --window 5m
logdeck alerts rules disable <id>
logdeck alerts channels list
logdeck alerts channels add --type webhook --endpoint https://hooks.example.com/logdeck
logdeck alerts channels add --type ntfy --endpoint https://ntfy.sh/mytopic
logdeck alerts channels add --type gotify --endpoint https://gotify.example.com --secret <app-token>
logdeck alerts channels add --type telegram --secret "$TELEGRAM_BOT_TOKEN" --target "$CHAT_ID"
logdeck alerts channels add --type slack --endpoint https://hooks.slack.com/services/...
logdeck alerts channels add --type email --endpoint smtps://smtp.example.com --username alerts --secret "$SMTP_PASSWORD" --from alerts@example.com --target oncall@example.com
logdeck alerts channels add --type pagerduty --secret "$PAGERDUTY_ROUTING_KEY"
logdeck alerts channels add --type opsgenie --secret "$OPSGENIE_API_KEY"
logdeck alerts channels test <id>
logdeck alerts channels delete <id>
logdeck alerts silences
logdeck alerts silences add --project checkout --for 2h --comment "deploy"
logdeck alerts silences add --host prod --every sat,sun --at 02:00 --duration 2h --tz Europe/Berlin
logdeck alerts silences delete <id>
logdeck alerts history --limit 20
```

`silences add` mutes the alerts that match every `--rule`, `--host`, `--container` and `--project` you give (none mutes everything). Without time flags the silence lasts until you delete it. `--for` or `--until` ends it, and `--from` delays its start. `--every`, `--at`, `--duration` and `--tz` make it a weekly maintenance window instead.

`channels test` sends a test delivery and exits 1 if it fails. See [Alerting](/docs/alerting) for how rules, thresholds, and channels behave.

### `audit`

List recorded changes, newest first: who made each one, the action, the target and host, and the HTTP status. When authentication is enabled, it needs an admin session or an admin-scoped token. `--actor`, `--action`, `--target`, and `--host` match part of the value, ignoring case. `--failed` keeps only changes that failed or were refused, and `--since` takes an RFC 3339 time or a relative one such as `2h` or `1d`. `--limit` returns at most 500 entries at a time, and `--offset` skips to older ones. See [Audit log](/docs/audit-log).

```bash
logdeck audit
logdeck audit --actor token:ci --since 1d
logdeck audit --action container.remove --failed --limit 100
logdeck audit --limit 500 --offset 500
```

## Using with AI agents

The CLI lets an agent debug containerized services without a browser. A typical investigation:

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

- `-o json` always emits a single JSON document on stdout for one-shot commands. Streaming commands (`logs --follow`, `events`) emit NDJSON, one object per line. A followed stack also emits `{"type":"source",...}` lines when a container's stream opens, fails, or stops.
- `grep` and `logs --stack` in JSON mode report `incomplete: true` when some containers could not be read, so a partial result is never mistaken for a complete one.
- Runtime and usage errors go to stderr as `{"error": "..."}` in JSON mode, so stdout stays clean for parsing. If `-o` itself fails to parse, the error falls back to plain text.
- Exit codes: 0 success, 1 runtime or server error, 2 usage error.
- `--since` and `--until` accept relative durations (`15m`, `2h`, `1d`), so no date math is needed.
- `logdeck grep` is the fastest way to find which container is emitting an error across an entire deployment.
- For an MCP client instead of shell commands, see the [MCP server](/docs/mcp).
