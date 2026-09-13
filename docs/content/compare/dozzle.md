LogDeck and Dozzle are both free, self-hosted tools for reading Docker container logs in a browser. Dozzle is a live log viewer. LogDeck also stores log history on disk, sends alerts, and changes containers, from the browser, a CLI, or an AI agent. This page compares the two feature by feature, including the places where Dozzle is the better pick.

Checked against the Dozzle v11 documentation in September 2026. If something here is out of date, [open an issue](https://github.com/AmoabaKelvin/logdeck/issues).

## At a glance

| Feature | LogDeck | Dozzle |
| --- | --- | --- |
| Stored log history | On disk, searchable, survives rebuild and removal | None: live only, history via Dozzle Cloud |
| Search | Regex, level, and time range on the server, across history | Regex on the live view, SQL over JSON logs in the browser |
| Alerts | Event and log rules, rate windows, cooldowns, delivery history | Event, log, and metric rules with an expression language |
| Alert channels | Webhook (Slack and Discord as-is), ntfy, Gotify, Telegram | Webhook, Slack, Discord, ntfy; Telegram and email via Cloud |
| Edit env vars and resource limits | Yes, with .env import and Coolify sync | No |
| Compose stacks | Start, stop, restart a whole stack; merged logs | Merged group logs |
| Multi-host | Local, TCP, or SSH; no agents | TCP, or an agent container on each host |
| Web terminal | Yes | Yes |
| Split-screen log view | No | Yes |
| Swarm and Kubernetes | No | Yes |
| CLI and MCP | logdeck CLI on the server API; MCP with read and action tools | No API CLI; read-only MCP |
| Users and auth | Single admin, scoped API tokens, read-only mode | Multiple users, OIDC, forward proxy, per-user roles |
| Hosted tier | None | Dozzle Cloud (paid tiers) |
| License | GPL-3.0 | MIT |

## Pick Dozzle if

- Several people need their own accounts, with OIDC single sign-on, forward-proxy auth, or per-user roles.
- You run Docker Swarm or Kubernetes.
- You want split-screen log views, or SQL queries over JSON logs in the browser.
- You want alert rules on metrics, not only on events and log lines.
- You prefer the MIT license.

## Pick LogDeck if

- You need to read a container's logs after it restarted, was rebuilt, or was removed, without a hosted tier.
- You want alerts with rate windows and cooldowns, sent to ntfy, Gotify, Telegram, or a webhook, with a history of every delivery.
- You want to fix things from the same screen: raise a memory limit live, edit environment variables, or restart a whole Compose stack.
- You manage remote hosts over SSH and don't want an agent container on each one.
- You want a CLI and an MCP server that can act, limited by scoped read and admin API tokens.

## Log history

Dozzle shows the logs the container engine still holds. Remove a container and its logs go with it, and keeping history means Dozzle Cloud.

LogDeck writes every container's logs to a SQLite database on its own disk, keyed by container name. A restart, a rebuild, or `docker compose up -d --build` appends to the same timeline, and a removed container's stored logs stay readable. Search, level filters, and time ranges run on the server across everything stored. Retention caps (50 MB per container and 1024 MB in total by default) bound the disk use. See [Log history](/docs/log-history).

## Alerts

Both tools alert on container events and log lines. Dozzle's rules use an expression language and also cover metrics. LogDeck's rules match container deaths with a non-zero exit code, OOM kills, failing health checks, and log lines by level or regex, with a threshold like "5 matches in 60 seconds" and a per-rule cooldown that reports how many matches it suppressed. See [Alerting](/docs/alerting).

## Changing containers

Both have a web terminal. LogDeck also edits memory limits, CPU limits, and restart policies live through the engine's update API, edits environment variables (recreating the container, with optional Coolify sync), and starts, stops, or restarts a whole Compose stack. Dozzle does not change containers beyond that.

## CLI, API, and AI agents

LogDeck ships the `logdeck` CLI, which covers everything the UI can see with JSON output on every command, and `logdeck mcp`, a Model Context Protocol server. A read-scoped API token keeps an agent read-only. An admin token lets it restart containers, run commands, and edit environment variables. Dozzle has a read-only MCP server and no CLI for its API. See the [CLI reference](/docs/cli) and [MCP server](/docs/mcp).

## Users and access

Dozzle supports multiple users, OIDC, forward-proxy auth, and per-user roles. LogDeck has one admin login, scoped `read` and `admin` API tokens for scripts and people, and a read-only mode for an instance everyone can see. If several people need their own logins, Dozzle fits better.

## Trying LogDeck

Both tools run as a single container with the Docker socket mounted, so you can run them side by side while you decide. The [installation guide](/docs/installation) has a Docker Compose file, and the [live demo](/demo) runs the LogDeck UI in your browser without installing anything.
