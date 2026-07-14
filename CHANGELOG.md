# Changelog

All notable changes to LogDeck are documented here.

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

[0.2.0]: https://github.com/AmoabaKelvin/logdeck/releases/tag/v0.2.0
[0.1.0]: https://github.com/AmoabaKelvin/logdeck/releases/tag/v0.1.0
