The audit log records every change made through LogDeck: who made it, what it was, which host and container or setting it touched, when, and whether it worked. Open it under **Settings > Audit log**. Only admins see it: read-scoped API tokens are refused, the same as for the rest of Settings.

## What is recorded

Every request that can change something is recorded once it has been handled, whether it came from the UI, the [CLI](/docs/cli), [MCP](/docs/mcp), or a script calling the API:

- Container start, stop, restart, and remove, and Compose stack start, stop, and restart
- Opening a terminal and running a command in a container. A terminal is recorded as soon as it opens.
- Env var and resource limit changes, and Coolify and Dokploy deploys
- Settings changes: hosts and connections, read-only mode, sign-in, API tokens, redaction, log storage, backups and restores, and alert rules, channels, and history
- Deleting stored logs

Refused and failed attempts are recorded too, for example a container stop blocked by [read-only mode](/docs/configuration#read-only-mode), with the HTTP status that was returned.

Reads are not recorded: viewing or streaming logs, listing containers, and opening Settings leave no trace in the audit log. Neither do redaction previews and connection tests, which change nothing.

Each entry holds:

- **Who**: the username for a signed-in user, `token:<name>` for an [API token](/docs/configuration#api-tokens), along with its role or scope. With authentication disabled there is no one to name, and the entry says so.
- **Action**: a short name such as `container.stop`, `settings.read_only`, or `api_token.create`
- **Target**: the container ID, Compose project, alert rule or channel ID, API token, or backup it applied to, and the host
- **Result**: the HTTP status and how long the request took

Request bodies are never recorded, so env values, passwords, and tokens do not end up in the audit log. It records that the env vars of a container changed, not what they changed to.

Requests that fail authentication, such as a wrong password, an expired session, or a read-scoped token trying to make a change, are refused before LogDeck knows who sent them and are not recorded.

## Storage

The audit log is kept in `audit.jsonl` in the [data directory](/docs/configuration#the-data-directory), one JSON object per line, so it survives restarts. LogDeck keeps the newest 10,000 entries and drops older ones.

Entries are written by a single background writer, so a change never waits on the disk. They are appended to the file as they happen, and the file is rewritten only once it has grown well past the cap.

The audit log goes into every [backup](/docs/backups). Restoring a backup adds the entries it holds that are older than anything in the current audit log, and keeps every current entry, so the changes made since the backup, including the restore itself, stay on record.

## CLI and API

```bash
logdeck audit
logdeck audit --actor token:ci --failed --since 1d
logdeck audit --action container.stop --target web -o json
```

`GET /api/v1/settings/audit` returns `{"entries": [...], "total": n}`, newest first. Every filter is optional:

| Parameter | What it matches                                                        |
| --------- | ---------------------------------------------------------------------- |
| `actor`   | Part of who made the change, ignoring case                             |
| `action`  | Part of the action name, so `container` matches every container action |
| `target`  | Part of the target                                                     |
| `host`    | Part of the host                                                       |
| `result`  | `success` or `failure`                                                 |
| `since`   | Changes at or after this RFC 3339 time                                 |
| `until`   | Changes at or before this RFC 3339 time                                |
| `limit`   | Page size, 50 by default and at most 500                               |
| `offset`  | Entries to skip, for the next page                                     |
