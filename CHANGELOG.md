# Changelog

All notable changes to LogDeck are documented here.

## [0.8.0](https://github.com/AmoabaKelvin/logdeck/compare/v0.7.0...v0.8.0) (2026-09-24)


### Features

* **podman:** group Quadlet containers by pod or stack label ([0f5dc70](https://github.com/AmoabaKelvin/logdeck/commit/0f5dc705ed961d231fab9354e5756d350ccaf65a))
* **podman:** group Quadlet containers by pod or stack label ([b39011d](https://github.com/AmoabaKelvin/logdeck/commit/b39011dba4399e2c02b47d699d352eb5d263dd70)), closes [#124](https://github.com/AmoabaKelvin/logdeck/issues/124)


### Bug Fixes

* **podman:** address review on Quadlet grouping ([0cb32a6](https://github.com/AmoabaKelvin/logdeck/commit/0cb32a6801eeb4256d906ac8d360683c81948f70))

## [0.7.0](https://github.com/AmoabaKelvin/logdeck/compare/v0.6.0...v0.7.0) (2026-09-23)


### Features

* **history:** search stored logs across containers and stacks ([fad7415](https://github.com/AmoabaKelvin/logdeck/commit/fad7415d225268ce82a86e35d82dd6c4633de368))
* **history:** search stored logs across containers and stacks ([2234c1f](https://github.com/AmoabaKelvin/logdeck/commit/2234c1f677543ac2aef4b5e4c14f05ed9bafc3a7))


### Performance

* **models:** skip level and grouping regexes that cannot match ([d91e7cc](https://github.com/AmoabaKelvin/logdeck/commit/d91e7cc32bb41ef0a455f1ec8f34b1844d000bc8))

## [0.6.0](https://github.com/AmoabaKelvin/logdeck/compare/v0.5.0...v0.6.0) (2026-09-23)


### Features

* **dashboard:** history toggle in the logs sheet ([e3df945](https://github.com/AmoabaKelvin/logdeck/commit/e3df945cbd72a9b734bc6554fb8d5d9b377b9b29))
* **dashboard:** history toggle in the logs sheet ([72d8256](https://github.com/AmoabaKelvin/logdeck/commit/72d825605065f97a1d658c3bedc2bae305fe1098))
* **history:** add a delete-all for stored logs ([3f36964](https://github.com/AmoabaKelvin/logdeck/commit/3f3696476877cee86ce5b60ea82510cbb776f660))
* **history:** page and search stored containers server-side ([03ac0fc](https://github.com/AmoabaKelvin/logdeck/commit/03ac0fc80814aa05bb641fb0ccad850c00440875))
* **history:** server-side paging and delete-all for stored logs ([1dbb9d5](https://github.com/AmoabaKelvin/logdeck/commit/1dbb9d5062bcb325bcbc44f65057ce461ca86ac0))
* **logstore:** expire removed containers' logs after 30 days ([c2c565b](https://github.com/AmoabaKelvin/logdeck/commit/c2c565b5ee55fa514ec02dc4b942fd39df6622d5))
* **logstore:** expire removed containers' logs after 30 days ([5292142](https://github.com/AmoabaKelvin/logdeck/commit/52921429db1cb91b4c69c83dd80e5b99352d74ad))
* **settings:** filter stored containers, delete all removed at once ([23d3239](https://github.com/AmoabaKelvin/logdeck/commit/23d32399fc56b95f78e21f482d6932ee5597b9a8))
* **settings:** filter stored containers, delete all removed at once ([999370b](https://github.com/AmoabaKelvin/logdeck/commit/999370bf241cf50498fdfccbc926fa23368e058b))


### Bug Fixes

* **frontend:** run the unit tests again, and in CI ([d7bb912](https://github.com/AmoabaKelvin/logdeck/commit/d7bb9122be056102966877d6c259f37f55a6e0ca))
* **frontend:** run the unit tests again, and in CI ([8e6efbc](https://github.com/AmoabaKelvin/logdeck/commit/8e6efbcb727f420c9a795faa701b5687909badcd))

## [0.5.0] - 2026-09-22

### Added

**Settings page.** Settings uses the same layout as the containers and container pages: full width, ruled sections instead of cards, and a side nav in place of the tab pills. Inline forms for adding a host, token or channel sit in recessed wells and stack on phones. Log storage shows used space, the per-container cap and the stored container count as a readings strip, and the stored containers table is paginated, 10 per page. It used to render every row, which froze the page once the store held a few hundred containers.

**Logs sheet.** Opening logs from the dashboard shows a slice of the container page: the name and state in the title, the same CPU, memory, uptime and ports strip, and the same Overview, Network, Environment and Limits panels. The live stream fills the rest of the sheet instead of a fixed 400px box. The old details card, label list and environment toggle are gone, since the panels cover them. Open page links to the full container page.

### Changed

- The dashboard opens on running containers. With log history kept, exited and removed containers were crowding out the default view. All states is one click away in the state filter.
- Redis logs get a level from Redis's single-character marker, and Postgres DETAIL, HINT and STATEMENT lines fold into the ERROR or LOG line above them. Both used to show as separate UNKNOWN rows.
- Multi-line stack traces stay in one row under the right level. A Go panic used to split into seven rows and a Python traceback into three, and an exception printed after an INFO line was folded into that INFO entry, so an ERROR filter never showed it. Exception class names like `TypeError` count as ERROR, phrases like `0 failed`, `err=nil` and `error handler` no longer count as a level, and access log lines take their level from the status code (5xx ERROR, 4xx WARN).
- The README is a quick start with links; the full feature tour lives on logdeck.dev.

### Fixed

- A burst of parallel requests to an SSH host spawned an ssh process per request before the master socket was ready, which tripped sshd's MaxStartups and dropped connections. Dials to a host now go one at a time until the connection is up.
- Replaying a batch of interleaved containers into the log store decompressed the same block once per line switch. Decoded blocks are now cached across a commit, capped at 16 MB.

## [0.4.0] - 2026-09-17

### Added

**Sortable container table.** Every column header sorts: name, host, status, uptime, created, ports, and usage. Usage sorts by CPU or memory, picked from a small menu beside the header, and re-sorts as new stats arrive. Containers with nothing to sort on, such as stopped ones under CPU, stay at the bottom in both directions. The sort is part of the URL, so a bookmarked or shared link opens the same view.

**Host column and a Columns menu.** The table can show which host each container runs on. The column is off by default. A Columns menu in the toolbar shows or hides every column except the name and the row actions, and the choice is remembered in the browser.

**Collapsible project groups.** With grouping on, each compose project has a header with a chevron, a container count, and how many are running. Its containers are indented under it. Click a header to fold the project away, or use Collapse all and Expand all in the grouping menu. Collapsed projects are remembered in the browser.

**Fullscreen logs and text size.** The log viewer on the container and stack pages can fill the screen from a toolbar button (`f` toggles, `Esc` leaves). The stream, scroll position, and selection carry over, and `?fullscreen=true` in the URL reopens it after a refresh. Text size is in the View menu, 11 to 20px, and `+` and `-` also work.

### Changed

- Grouping by project covers the whole filtered list, and pagination is hidden while it is on. Groups used to be built one page at a time, so a project could be split across pages and show the wrong count.

### Fixed

- With the same container name on two hosts, the container page always opened the first host's container. The header named the wrong host, and the logs and actions went to the wrong container as well. Links to the page now carry the host. Old links without one behave as before.
- The dashboard read "LogDeck on logdeck", or a container ID, when LogDeck ran in a container, because the hostname it reported was the container's own. It now reports the machine's name, taken from the local Docker or Podman engine.
- The toolbar above the container table overflowed the screen on phones. Its controls wrap now.
- A browser with localStorage blocked or full could fail to load the dashboard. Table preferences that cannot be saved now last for the session.

## [0.3.1] - 2026-09-16

### Fixed

- Every followed log, stats, and event stream over an SSH host died after ten seconds and reconnected through a new `ssh` process, and each stats poll opened one more process per container. Remote sshd throttled the handshakes. Streams now stay open, and every connection to a host is multiplexed over one OpenSSH ControlMaster session, so the remote sees a single login.
- LogDeck could sit at 140% CPU logging `database is locked`. Container metadata updates competed with log ingestion for SQLite's write lock during heavy backfills, and a failed commit was retried at full speed, which re-read and dropped the same batch in a tight loop. Writes are serialized on one connection now, and the writer backs off 1s to 5s after a failed commit.
- Two admins or agents saving the Docker or Coolify host list at the same time silently lost one of the changes. The settings response carries a revision per host list and a stale write is rejected with 409. The MCP host tools require it; the HTTP endpoint accepts it but does not insist, so older clients keep working.

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
