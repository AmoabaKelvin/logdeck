Mounting `/data` as a volume keeps LogDeck's state across container recreation, but it does not protect you from losing or corrupting that volume. Backups copy that state into one file you can keep somewhere else and restore from.

Open **Settings → Backups** to make, download, schedule, and restore backups. Only admins see the section: read-scoped API tokens are refused, the same as for the rest of Settings.

## What a backup holds

A backup is a single `.tar` file (`.tar.enc` when encrypted) with these entries:

- `manifest.json`: the backup format version, the LogDeck version that made it, when it was made, what it contains, and a SHA-256 checksum and size for every other file
- `config.json`: hosts, Coolify and Dokploy connections, read-only mode, alert rules and channels, the log-store and backup settings, and, if you choose, credentials
- `alerts-history.json`: the recently fired alerts
- `logs.db`: the [stored logs](/docs/log-history), only if you choose to include them

Some things are not in a backup, and you need to recover them with your deployment files:

- Settings that come from environment variables, such as `DOCKER_HOSTS`, `JWT_SECRET`, `ADMIN_USERNAME`, `ADMIN_PASSWORD`, `COOLIFY_CONFIGS`, and `DOKPLOY_CONFIGS`. Only what the Settings page saved to `config.json` is backed up.
- Files mounted into the container: the Docker or Podman socket, SSH keys, and TLS certificates for remote hosts.

## Credentials and encryption

**Include credentials** controls whether the backup carries the sign-in settings (the admin password hash and the JWT secret), API token hashes, Coolify and Dokploy API tokens, and alert channel URLs and tokens. A webhook or ntfy URL is itself a credential, so channel URLs count.

When a backup has no credentials, restoring it never changes who can sign in. The instance keeps the sign-in settings and API tokens it has when the restore is applied, so a token revoked while the restore waits for a restart stays revoked. A Coolify or Dokploy connection takes its token from the one with the same name and API URL on the instance you restore onto, and an alert channel takes its URL and token from the one with the same ID. One with no match comes back without its secret, and a channel without one comes back disabled. The restore preview counts how many need their secret entered again.

**Passphrase** encrypts the whole backup with AES-256-GCM, using a key derived from the passphrase with scrypt. LogDeck does not store the passphrase. A lost passphrase means the backup cannot be restored. An encrypted backup also hides its contents in the list until you enter the passphrase to restore it.

Stored logs and alert history can hold whatever your containers printed, including secrets. Redacting a log line later does not clean older backups. Encrypt any backup that includes logs before it leaves the host.

## Consistency and performance

LogDeck backs up while it keeps running. The stored logs are copied with SQLite's `VACUUM INTO`, which reads one consistent snapshot of the database while ingestion goes on writing. Log ingestion and the API do not pause. The snapshot is written next to `logs.db` first, so make sure the data volume has room for a second copy of the database while a backup with logs runs. LogDeck does not check free space first, and a full volume also stops the log store from writing. The archive is then streamed to the backup folder and renamed into place once it is complete, so a half-written backup never shows up in the list.

## Where backups go

Backups are written to `backups/` inside the data directory, `/data/backups` by default. That folder sits on the same disk as the data it protects. To keep backups off the host, point `BACKUP_DIR` at a mounted network share or a folder that a sync tool copies elsewhere:

```yaml
services:
  logdeck:
    environment:
      - BACKUP_DIR=/backups
    volumes:
      - logdeck-data:/data
      - /mnt/nas/logdeck:/backups
```

Give each LogDeck instance its own backup folder. Retention deletes the oldest `logdeck-scheduled-*` files it finds there, whichever instance wrote them. If the folder can't be written, the backup fails and Settings shows the error next to the list.

## Scheduled backups

Set **Every (hours)** above zero to take a backup on that interval, and **Keep the latest** to choose how many scheduled backups to keep. After each scheduled backup, LogDeck deletes the oldest scheduled ones beyond that count. Backups you made by hand are never deleted for you. The schedule continues from the newest scheduled backup on disk, so a restart does not take an extra one.

Scheduled backups are encrypted when the `BACKUP_PASSPHRASE` environment variable is set. It is read from the environment so that it never ends up in `config.json`, and so never inside a backup.

## Restoring

Restore a saved backup from its row in the list, or upload a file with **Restore from file**. Restoring takes three steps, and nothing on the instance changes until the last one:

1. **Check.** LogDeck reads the whole backup into a staging folder. It verifies the checksum and size of every file, rejects unknown, missing, or extra files, refuses a format version or log database schema newer than it supports, and parses each file the way it will load it. A truncated, corrupted, or tampered backup, or a wrong passphrase, is rejected with a message saying why.
2. **Preview and confirm.** The preview compares what the instance has now with what the backup will put in its place, and lists anything to watch for: a backup from a different LogDeck version, missing credentials, or no stored logs. A backup without stored logs keeps the instance's current logs.
3. **Restart.** The confirmed restore is applied the next time LogDeck starts, before anything opens the files, for example with `docker restart logdeck`. Settings shows that a restore is waiting until then, and you can still cancel it.

At startup, LogDeck moves each current file into a `pre-restore-<time>` folder in the data directory before it moves the restored copy into place. That folder is your way back, and LogDeck does not delete it. If LogDeck stops part-way, the next start finishes the job. If a file can't be moved, LogDeck puts back what it already moved, keeps the backup in a `restore-failed-<time>` folder, and starts with the data it had.

Restoring does not replay anything. Past alerts in the restored history are not sent again, and alert rules only fire on new events. LogDeck never stores container actions, so none are resumed.

If you upload a large backup through a reverse proxy, raise its request body limit (for example nginx's `client_max_body_size`) or restore from the backup folder instead.

## API

All routes need an admin session or an admin-scoped API token.

| Method   | Path                                       | What it does                                                                           |
| -------- | ------------------------------------------ | -------------------------------------------------------------------------------------- |
| `GET`    | `/api/v1/settings/backups`                 | The backup folder, its backups, the schedule, the last result, and any waiting restore |
| `POST`   | `/api/v1/settings/backups`                 | Start a backup: `{"includeLogs": true, "includeSecrets": false, "passphrase": ""}`     |
| `GET`    | `/api/v1/settings/backups/{name}`          | Download a backup                                                                      |
| `DELETE` | `/api/v1/settings/backups/{name}`          | Delete a backup                                                                        |
| `PUT`    | `/api/v1/settings/backups/schedule`        | `{"intervalHours": 24, "keep": 7, "includeLogs": false, "includeSecrets": false}`      |
| `POST`   | `/api/v1/settings/backups/restore`         | Check an uploaded backup sent as the request body                                      |
| `POST`   | `/api/v1/settings/backups/{name}/restore`  | Check a backup from the backup folder                                                  |
| `POST`   | `/api/v1/settings/backups/restore/confirm` | Apply the checked backup on the next start                                             |
| `DELETE` | `/api/v1/settings/backups/restore`         | Cancel a checked or waiting restore                                                    |

Send the passphrase of an encrypted backup in the `X-Backup-Passphrase` header. Restoring and deleting are blocked in [read-only mode](/docs/configuration#read-only-mode).

```bash
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "X-Backup-Passphrase: $PASSPHRASE" \
  --data-binary @logdeck-manual-20261030T120000Z.tar.enc \
  https://logdeck.example.com/api/v1/settings/backups/restore
```
