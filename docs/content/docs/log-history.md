LogDeck stores container logs on its own disk, so you can still read them after a container restarts, gets rebuilt with a new image, or is removed entirely.

By default, LogDeck tails every container on every configured host and writes those lines to a SQLite database. That store powers **History** mode in the log viewer.

It is a local convenience store, not a log aggregation platform. Retention caps bound it, it lives on the machine running LogDeck, and nothing replicates it anywhere.

> **Mount a volume, or history is not history.** LogDeck writes `logs.db` into the same directory as its config file, which is `/data/logs.db` with the default `CONFIG_PATH`. If `/data` is not a mounted volume, the database sits inside the container's filesystem and disappears the next time you recreate LogDeck. The same volume also holds `config.json` (hosts, API tokens, alert rules) and `alerts-history.json`.

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
      # Config file, stored logs, and alert history live here
      - logdeck-data:/data
    restart: unless-stopped

volumes:
  logdeck-data:
```

## What survives what

LogDeck keys a container's stored history by its host and its **name**, not by its engine ID. It records every engine container ID as a separate generation of that name, and queries stitch the generations back together in timestamp order. That is why history survives operations that give a container a brand-new ID:

- **Restart** (`docker restart`, a crash loop, a restart policy). One continuous timeline.
- **Rebuild or recreate** (`docker compose up -d --build`, an image bump, an environment-variable edit in LogDeck). The new container appends to the same timeline under the same name.
- **Removal.** The lines already stored stay readable, even though the container is gone from the engine. See [Removed containers](#removed-containers).
- **Restarting LogDeck itself.** On startup, LogDeck re-reads each container's engine logs from where it left off, so it backfills lines emitted while it was down instead of losing them.

> **Limits worth knowing.**
>
> - Backfill reads the logs the _engine_ still holds. If you remove a container while LogDeck is down, the engine discards its logs with it, and whatever LogDeck had not already stored is gone for good.
> - LogDeck cannot read containers whose logging driver has no read API (`awslogs`, `syslog`, `none`, and so on). It leaves them out of the store and writes the reason to the server log.
> - Renaming a container starts a new timeline, because the name is the identity.

## Using History mode

On a container's log page, the toolbar shows a **Live | History** toggle whenever the store is enabled. Live streams from the engine as always. History queries the database.

In History mode:

- **The server applies search, level filter, and time range** across everything stored for that container, not just the lines loaded in the browser. Search accepts plain text or a regex.
- Results page backwards from the newest line. A **Load older** button fetches the previous page (500 lines) until you reach the start of stored history.
- Timestamps, wrapping, line selection, pinning, copying, and downloading (JSON or TXT) work the same as in Live mode.
- Streaming controls (Stream, Pause, tail size, auto-scroll) are hidden, since there is nothing to stream.

A Compose stack's log page has the same toggle. Its History mode reads every member of the stack as one merged timeline, including members that have since been torn down. The quick-look log sheet on the dashboard is live-only, so open the container's full log page for history.

## Searching every container

The **Search** page runs the same search, level filter, and time range over every container's stored logs at once, merged by timestamp, with each line labeled by its container. It answers questions like "which service logged `connection refused` between 02:40 and 02:50?" without opening containers one by one.

Each request searches a bounded slice of history, so a rare term never holds the server for long. When a request stops before it fills a page, the viewer shows how far back it searched and offers **Search older** to continue from there. Narrow the time range to search a known window in one go.

### Removed containers

When LogDeck holds stored logs for a container that no longer exists on any host, the dashboard's state summary grows a **Removed** chip. "All states" hides removed containers. Click the chip, or pick _Removed_ in the state filter, to list them.

A removed container shows how much log data LogDeck stores for it instead of CPU and memory, and offers a single action: **View stored logs**. Its log page opens locked to History. There is no live stream, no terminal, and no environment or resources tab, because there is no container left to inspect.

Removed containers' logs are dropped 30 days after the container left the engine. Change the window under Settings, Log storage, or with `LOG_STORE_REMOVED_DAYS`; `0` keeps them until a retention cap evicts them. **Delete removed** on the same page clears them all at once, and **Delete all** clears every container's stored logs. Running containers start a fresh history with their next line.

## Retention and disk use

Two caps bound the store. A sweep runs every minute to enforce them by evicting the **oldest lines first**:

- **Per container** (default `50` MB). Applies to a logical container, meaning all generations of the same name together. A rebuilt container does not get a fresh budget.
- **Total** (default `1024` MB). Applies to the whole store, across every host and container.

LogDeck never vacuums the database file. SQLite reuses freed pages, so after eviction the file plateaus at its high-water mark instead of shrinking. Size `/data` for roughly the total cap plus headroom.

## Configuration

Persistence is **enabled by default**. You configure it in the config file under `logStore`, and an environment variable can override every field. The environment variable wins over the file. There is no Settings page for it.

```json
{
  "logStore": {
    "enabled": true,
    "perContainerMB": 50,
    "totalMB": 1024
  }
}
```

Each environment variable overrides the matching config-file field:

- `LOG_STORE_ENABLED`: `false` turns persistence off entirely. LogDeck creates no database file, History mode disappears from the UI, and the history endpoints report the store as disabled. Existing data stays on disk untouched. Default: `true`.
- `LOG_STORE_PER_CONTAINER_MB`: per-container retention cap in MB. Must be a positive integer. LogDeck ignores anything else with a warning. Default: `50`.
- `LOG_STORE_TOTAL_MB`: total retention cap in MB across the whole store. Must be a positive integer. Default: `1024`.
- `LOG_STORE_REMOVED_DAYS`: how many days a removed container's logs are kept after it leaves the engine. `0` keeps them until a cap evicts them. Default: `30`.

If LogDeck cannot open the database (a read-only volume, a missing mount), it logs a warning and keeps running _without_ stored logs. Persistence never blocks startup. Look for `Log persistence is ENABLED` in the server log, with the path and the caps, to confirm it came up.

## API

The HTTP API exposes the store. These are read endpoints, so a `read`-scoped API token can call them.

- `GET /api/v1/history/status`: whether persistence is available (`{"enabled": true}`).
- `GET /api/v1/history/containers`: the logical containers the store knows about, including removed ones, with their stored size.
- `GET /api/v1/history/logs`: one page of stored logs, from one container, one Compose project, or every container. Returns `503` when persistence is disabled.

Deleting stored logs is destructive, so `read`-scoped tokens get `403` and read-only mode blocks it:

- `DELETE /api/v1/history/containers/{name}?host=...`: one container's stored logs.
- `DELETE /api/v1/history/removed`: every removed container's stored logs.
- `DELETE /api/v1/history/containers`: every container's stored logs.

`/history/containers` returns every container when called bare. These optional query parameters filter and page it:

| Parameter | Meaning                                                          |
| --------- | ---------------------------------------------------------------- |
| `search`  | Case-insensitive match on the name, host, or Compose project.    |
| `sort`    | `name` (host, then name; the default) or `size` (biggest first). |
| `limit`   | Containers per page, up to `500`. Omit for all.                  |
| `offset`  | Containers to skip.                                              |

The response carries `total` (containers matching `search`), `storedCount` (every stored container), and `removedCount` (stored containers that no longer exist on any host) next to `containers`.

```bash
curl -H "Authorization: Bearer ldk_..." \
  "http://localhost:8123/api/v1/history/containers?search=api&sort=size&limit=20"
```

`/history/logs` takes these query parameters, all optional:

| Parameter   | Meaning                                       |
| ----------- | --------------------------------------------- |
| `container` | Container name. Omit to read every container. |
| `project`   | Only this Compose project's containers.       |
| `host`      | Only containers on this host.                 |
| `search`    | Text to search for.                           |
| `regex`     | Boolean. Treat `search` as a regex.           |
| `levels`    | Comma-separated levels, including `UNKNOWN`.  |
| `since`     | Start time, RFC3339.                          |
| `until`     | End time, RFC3339.                            |
| `limit`     | Lines per page. Default `500`, max `1000`.    |
| `cursor`    | Page cursor from a previous response.         |

Pages walk backwards through history, merged by timestamp across the containers read. Follow the returned `nextCursor` for older lines. Every entry carries `containerName` and `host`.

A request reads at most about 200,000 stored lines. When that cuts a page short, the response carries `scannedTo`, the time the search reached, and `nextCursor` continues from there. Such a page can hold fewer lines than `limit`, or none.

```bash
curl -H "Authorization: Bearer ldk_..." \
  "http://localhost:8123/api/v1/history/logs?container=api&host=local&levels=ERROR,FATAL&limit=200"

curl -H "Authorization: Bearer ldk_..." \
  "http://localhost:8123/api/v1/history/logs?search=connection%20refused&since=2026-09-23T02:40:00Z&until=2026-09-23T02:50:00Z"
```
