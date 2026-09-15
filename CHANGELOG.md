# Changelog

All notable changes to LogDeck are documented here.

## [0.3.0] - 2026-09-15

### Added

**MCP server.** `logdeck mcp` runs a Model Context Protocol server over stdio, so an AI assistant can list containers, read live and stored logs, check stats and history, edit environment variables and settings, and start, stop, restart, remove, or run commands in containers through the same API the CLI uses. Every tool is registered. The API token you hand it decides what works. A read-scoped token gets a read-only assistant, an admin token gets everything, and the server checks the scope on each call. See the MCP server docs page for client setup.

**Notification channels.** The single alert webhook is now a list of channels: generic webhook (Slack and Discord accept it as-is), ntfy, Gotify, and Telegram. Each alert goes to every enabled channel. Every channel has a test button in Settings and `logdeck alerts channels test`. An existing webhook config becomes a webhook channel on upgrade.

**Unhealthy alerts.** Event rules can fire when a container's healthcheck goes unhealthy, next to died and OOM-killed, with a "Health check failing" preset. Recoveries do not fire. Works on Podman, which leaves the state out of the event.

**Container detail page.** It opens with the container itself: name, state, image, host, live CPU, memory, and uptime, with the lifecycle actions in the header and the log stream taking the rest of the page. Panels below show what Docker's inspect payload always carried but the UI never read: restart count, exit code, OOM state, the healthcheck probe and its last five results with output, published ports as links, networks with addresses, mounts, a filterable environment editor that masks credential-looking values, and memory and CPU limits drawn against live usage. The terminal is a full-height dialog.

**Multi-line log events.** Stack traces, progress bars, and other spill-over lines fold into the entry they belong to instead of showing as separate rows with their own timestamps. The grouping recognises Python, Java, and Go tracebacks, and a progress bar rewritten with carriage returns shows its final state. Live streams fold on the server too, so live and history agree.

**Published ports** in the container list and the API.

**`PPROF_ADDR`** exposes Go heap and goroutine profiles on a loopback address. Off unless set.

### Changed

- Dashboard redesign. Navigation and global actions moved into a header, filters share one toolbar row, and status has its own column. Memory shows as an absolute against its limit (`52 MB of 64 MB`) instead of a percentage of an unstated ceiling, amber past 75% and rose past 90%. Long sha256 image references show in the short form Docker prints.
- Stored log history is compressed. Once a container has a thousand stored lines, the store seals them into one zstd block. On real Docker output that holds 8 to 16 times more history under the same retention cap, and full-history search reads far fewer pages. Lines stay byte-for-byte what the engine sent. The database file does not shrink, since SQLite reuses freed pages, but the caps now bound the compressed size.
- Log parsing costs a fraction of what it did. A line used to spend about 400 allocations on failed timestamp parses and seven regex passes on level detection. It now takes 3 allocations, and a line with no level keyword skips the regexes. A backfill that allocated 8 GB now allocates 0.8 GB, and memory after startup is about a third lower.
- The Docker image ships a stripped binary, 22 MB down to 16 MB.
- Go 1.25.

### Fixed

- Retention could not keep up under sustained writes. Its separate transaction lost the SQLite write lock to ingestion, nothing was evicted, and the file grew past the cap. Eviction now runs on the writer and the file stays near the cap under load.
- Stored logs had silent gaps. A container that stopped could lose lines Docker still had buffered, a burst that overflowed the live buffer dropped its oldest lines without repair, and a container that logged between its first backfill and the live stream attaching could lose that window. All three are detected now and re-read from the engine, with duplicates dropped.
- LogDeck over-reported container memory on cgroup v2 hosts, the modern default, by counting page cache as used. It now matches `docker stats`.
- Editing environment variables on Podman failed with `NanoCpus conflicts with CpuPeriod and CpuQuota` for any container with a CPU limit, from the web UI as well.
- Removing an environment variable you had just added still counted as a change and offered to recreate the container.

### Upgrading

The log database migrates itself. After the upgrade, every stored container is re-read once from the engine so the first-backfill fix takes effect. Inserts are deduplicated, so nothing is stored twice. Expect one burst of engine reads on first start.

An existing alert webhook becomes a webhook channel automatically. `logdeck alerts channels` manages the list.

## [0.2.0] - 2026-07-14

### Added

**Log persistence and history.** Container logs are stored on disk, so they stay readable after a container is restarted, rebuilt, or removed. A rebuilt container keeps one continuous timeline under its name, even though the engine gives it a new ID. A `Live | History` toggle in the log viewer pages back through stored logs with server-side level, regex, and time-range filtering. Containers that no longer exist appear on the dashboard under a "Removed" filter, and their logs remain readable until you delete them. Retention defaults to 50 MB per container and 1 GB total, editable in Settings.

**Alerting.** Rules fire on container events (died, OOM-killed) or on log output (level threshold plus an optional regex). A rate condition ("5 in 60 seconds") ignores noise, and a per-rule cooldown means a crash-looping container sends one alert instead of hundreds — repeat occurrences are counted and reported on the next alert. Alerts go to a JSON webhook that Slack, Discord, and ntfy accept as-is, and every alert is recorded in a history you can browse. Manage rules from Settings or the `logdeck alerts` CLI.

**Scoped API tokens.** Tokens are now `admin` or `read`. A read token can fetch logs, stats, events, and container details, but cannot mutate anything, use the web terminal, or read container environment variables, settings, or alert configuration — safe to hand to a CI job or an AI agent.

**Container health.** Healthy, unhealthy, and starting badges from Docker healthchecks, in the containers table and on the container page. (Podman's Docker-compatible list API does not report health, so badges there are Docker-only.)

### Changed

- Settings is organized into tabs: Connections, Access, Alerts, and Log storage. Each tab is linkable (`/settings?tab=alerts`).
- The `logdeck` CLI gains `alerts` (rules, history, webhook, test).

### Fixed

- `DOCKER_HOST` silently overrode every configured non-SSH host, collapsing a multi-host setup onto a single socket. An explicitly configured host now always wins.
- Read-scoped tokens could read the alert webhook URL, which is a secret.
- `env.example` advertised `BACKEND_PORT` and `FRONTEND_PORT`, which the server never reads, and omitted `CONFIG_PATH`, `READONLY_MODE`, and the retention caps.
- Documentation examples omitted the `/data` volume, so following them cost you your configuration on every upgrade.

### Upgrading

**Mount a volume at `/data`.** It holds the config file (Docker hosts, API tokens, alert rules), the stored logs, and the alert history. Without it, all of that is lost every time the container is recreated. LogDeck now warns at startup if the directory is not a mounted volume.

```
-v logdeck-data:/data
```

Log persistence is enabled by default. To turn it off, set `LOG_STORE_ENABLED=false`.

If you script the CLI with a read-scoped token, note that `logdeck alerts` now requires an admin token.

## [0.1.0] - 2026-07-10

First release. Multi-host Docker and Podman support, live and historical log viewing with level classification and search, compose stack grouping and actions, container lifecycle actions, environment and resource editing, a web terminal, image/volume/network views, host and container stats, read-only mode, authentication, API tokens, and the `logdeck` CLI.

[0.3.0]: https://github.com/AmoabaKelvin/logdeck/releases/tag/v0.3.0
[0.2.0]: https://github.com/AmoabaKelvin/logdeck/releases/tag/v0.2.0
[0.1.0]: https://github.com/AmoabaKelvin/logdeck/releases/tag/v0.1.0
