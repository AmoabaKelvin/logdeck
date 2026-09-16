LogDeck runs with no configuration at all, and every environment variable on this page is optional. This is the reference for configuring it: the data directory, every key in the config file, every environment variable, authentication, API tokens, read-only mode, and reverse proxy setups.

## Two ways to configure

LogDeck reads its configuration from environment variables and from a JSON config file that the Settings page writes. Environment variables win. A value pinned by the environment shows in the UI but can't be changed there, which is what you want when the deployment, not the admin, should have the final say.

Hosts, Coolify hosts, read-only mode, and authentication can come from either source. API tokens, alert rules, and log retention live only in the config file. Manage tokens in the UI, alert rules in the UI or the CLI, and retention through the config file or its [environment overrides](#log-persistence). [The config file](#the-config-file) section documents every key, so you can also generate the file yourself.

## The data directory

Everything LogDeck persists lives in one directory, the directory of its config file. That is `/data` by default:

- `config.json`: hosts, Coolify hosts, read-only mode, auth, API tokens, alert rules, and log-store settings
- `logs.db`: the SQLite [log store](/docs/log-history)
- `alerts-history.json`: recently fired [alerts](/docs/alerting)

> **Mount it as a volume.** Without one, all of the above is written inside the container and destroyed the next time you recreate it. You would lose your API tokens, your alert rules, and all stored log history.

```yaml
volumes:
  - logdeck-data:/data
```

Set `CONFIG_PATH` to move the config file, and with it the whole directory, somewhere else. For example, `CONFIG_PATH=/config/logdeck.json`.

## The config file

`config.json` is plain JSON, and the Settings page is only one way to write it. You can also generate it from Nix, Ansible, or a Git repo and ship it with the deployment. LogDeck reads the file once at startup, so restart it after editing the file by hand. A missing file is fine and means defaults. Invalid JSON is a startup error, so LogDeck never silently drops your settings. Unknown keys are ignored.

Every key is optional. This example sets all of them:

```json
{
  "dockerHosts": [
    { "name": "local", "host": "unix:///var/run/docker.sock" },
    { "name": "prod", "host": "ssh://deploy@prod.example.com" }
  ],
  "coolifyHosts": [
    {
      "hostName": "prod",
      "apiURL": "https://coolify.example.com",
      "apiToken": "your-coolify-token"
    }
  ],
  "readOnly": false,
  "auth": {
    "enabled": true,
    "jwtSecret": "a-long-random-string",
    "adminUsername": "admin",
    "adminPasswordHash": "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
  },
  "apiTokens": [
    {
      "name": "ci",
      "prefix": "ldk_3fK9xQ2b",
      "hash": "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
      "scope": "read",
      "createdAt": "2026-09-16T10:00:00Z"
    }
  ],
  "alerts": {
    "channels": [
      {
        "id": "c1",
        "type": "ntfy",
        "name": "Phone",
        "enabled": true,
        "url": "https://ntfy.sh/mytopic"
      },
      {
        "id": "c2",
        "type": "telegram",
        "enabled": true,
        "token": "123456:bot-token",
        "target": "987654321"
      }
    ],
    "rules": [
      {
        "id": "r1",
        "name": "crashes",
        "enabled": true,
        "type": "event",
        "events": ["die", "oom", "unhealthy"],
        "hosts": ["prod"],
        "threshold": 1,
        "cooldownSeconds": 300,
        "createdAt": "2026-09-16T10:00:00Z"
      },
      {
        "id": "r2",
        "name": "error spike",
        "enabled": true,
        "type": "log",
        "minLevel": "ERROR",
        "pattern": "timeout|refused",
        "projects": ["shop"],
        "threshold": 5,
        "windowSeconds": 60,
        "cooldownSeconds": 300,
        "createdAt": "2026-09-16T10:00:00Z"
      }
    ]
  },
  "logStore": {
    "enabled": true,
    "perContainerMB": 50,
    "totalMB": 1024
  }
}
```

> **The UI rewrites this file.** Any change saved on the Settings page, in the CLI, or by the alerts migration replaces the whole file. If a tool generates it for you, either pin the sections you care about with environment variables so the UI can't change them, or make your deploy regenerate the file and restart LogDeck. The `.tmp` file that briefly appears beside it during a write is normal.

### `dockerHosts`

A list of `{ "name", "host" }` objects, the same as [`DOCKER_HOSTS`](#docker_hosts). When `DOCKER_HOSTS` is also set, LogDeck uses both lists, and on a name collision the environment host wins and the file host is dropped. With neither set, LogDeck uses the local socket.

### `coolifyHosts`

A list of `{ "hostName", "apiURL", "apiToken" }` objects, the same as [`COOLIFY_CONFIGS`](#coolify_configs). Each `hostName` must match a Docker host name. Merged with `COOLIFY_CONFIGS` the same way Docker hosts are.

### `readOnly`

`true` or `false`. Ignored when `READONLY_MODE` is set.

### `auth`

Ignored when `JWT_SECRET`, `ADMIN_USERNAME`, or `ADMIN_PASSWORD` is set. With `enabled` true, all three of `jwtSecret`, `adminUsername`, and `adminPasswordHash` are required, or LogDeck runs without auth.

- `jwtSecret`: signs sessions. Any long random string. Changing it ends every session.
- `adminUsername`: the login name.
- `adminPasswordHash`: a [bcrypt hash](#password-hashing) of the password. JSON has no `$` escaping, so paste it as is.
- `adminPasswordSalt`: legacy only. When set, `adminPasswordHash` is read as hex SHA256(password + salt) instead of bcrypt.

### `apiTokens`

One entry per [API token](#api-tokens). The file holds only a hash, so you can mint tokens outside LogDeck and hand out the plain token yourself:

```bash
token="ldk_$(openssl rand -base64 32 | tr '+/' '-_' | tr -d '=')"
hash=$(printf %s "$token" | sha256sum | cut -d' ' -f1)
prefix=${token:0:12}
```

- `name`: a label shown in the UI.
- `prefix`: the first 12 characters of the token, including `ldk_`. It identifies the token in the UI and is the id used to revoke it.
- `hash`: hex SHA256 of the full token.
- `scope`: `admin` or `read`. Missing means `admin`. Any other value is treated as `read`, so a typo never grants admin.
- `createdAt`: RFC 3339 timestamp, display only.

### `alerts`

Rules and channels, as described on the [alerting](/docs/alerting) page. IDs are opaque strings and only need to be unique within their list. `createdAt` is display only.

`channels` entries:

- `id`, `type`, `enabled`, and an optional `name`.
- `type` is `webhook`, `ntfy`, `gotify`, or `telegram`.
- `url`: the webhook URL, the ntfy topic URL, or the Gotify server base URL.
- `token`: the Gotify app token or the Telegram bot token.
- `target`: the Telegram chat id.

`rules` entries:

- `id`, `name`, `enabled`, and `type`, which is `event` or `log`. Rules with any other type are skipped with a log line.
- `hosts`, `containers`, `projects`: optional targeting lists, combined with AND. Empty means all.
- `events`: for event rules, any of `die`, `oom`, `unhealthy`.
- `minLevel`, `pattern`: for log rules, a level name and an RE2 regex. Set either or both. A rule with an invalid pattern is skipped with a log line.
- `threshold`: matches needed before the rule fires. 0 or 1 fires on every match.
- `windowSeconds`: the threshold window. Default 60.
- `cooldownSeconds`: minimum seconds between alerts for the same rule and container. Default 300.

`webhookUrl` is a legacy key from before channels existed. LogDeck turns it into a webhook channel on the next start and rewrites the file.

### `logStore`

`enabled`, `perContainerMB`, and `totalMB`, with the defaults shown in the example. Each one is overridden independently by its [environment variable](#log-persistence). Zero or negative sizes fall back to the default.

## Server

### `DOCKER_HOSTS`

Optional. A comma-separated list of hosts to manage. Each entry uses `name=host` format and supports `unix://`, `tcp://`, and `ssh://` URLs.

```bash
# Local only
DOCKER_HOSTS=local=unix:///var/run/docker.sock

# SSH connection (mount your SSH keys or forward agent)
DOCKER_HOSTS=local=unix:///var/run/docker.sock,prod=ssh://deploy@prod.example.com

# Plain TCP: unencrypted, trusted private networks only
DOCKER_HOSTS=local=unix:///var/run/docker.sock,staging=tcp://192.168.1.100:2375
```

Host names appear in the UI and the container list, so you always know which daemon you are working with. Hosts defined here can't be edited or removed from the Settings page, and hosts added in Settings are merged with them. For `ssh://` targets, mount your SSH keys or forward an SSH agent into the container.

> **Plain `tcp://` is unencrypted and has no authentication.** Anyone who can reach a Docker daemon on port 2375 controls that host. Use `ssh://` for remote hosts where you can. If you need TCP, expose the daemon with TLS (usually port 2376) and give LogDeck the client certificates through Docker's standard `DOCKER_TLS_VERIFY=1` and `DOCKER_CERT_PATH` variables. They apply to every `tcp://` host.

When `DOCKER_HOSTS` is unset, LogDeck probes for a local socket in this order: Docker (`/var/run/docker.sock`), rootless Podman (`$XDG_RUNTIME_DIR/podman/podman.sock`), then rootful Podman (`/run/podman/podman.sock`). Podman works through its Docker-compatible API socket.

### `CONFIG_PATH`

Optional. Path to the JSON config file. Its directory also holds the log store and the alert history, so keep it on a mounted volume.

Default: `/data/config.json`

### `READONLY_MODE`

Optional. Set to `true` to block container actions. [Read-only mode](#read-only-mode) lists exactly what it blocks.

Default: `false`

### Port

The server always listens on port `8080` inside the container, and no variable changes it. Publish it on whichever host port you like, for example `-p 8123:8080`.

## Log persistence

Persistence is on by default and needs no configuration. These variables override the `logStore` section of the config file. [Log history](/docs/log-history) explains how the store works.

### `LOG_STORE_ENABLED`

Optional. `false` turns off log persistence entirely, with no database file and no History mode.

Default: `true`

### `LOG_STORE_PER_CONTAINER_MB`

Optional. Retention cap per container, in MB. LogDeck evicts the oldest lines first.

Default: `50`

### `LOG_STORE_TOTAL_MB`

Optional. Retention cap for the whole store, in MB.

Default: `1024`

```bash
LOG_STORE_ENABLED=true
LOG_STORE_PER_CONTAINER_MB=50
LOG_STORE_TOTAL_MB=1024
```

## Proxies and CORS

You only need these in specific deployments.

### `TRUST_PROXY_HEADERS`

Optional. Set to `true` to trust `X-Forwarded-For` and `X-Real-IP` when identifying a client. LogDeck uses the client IP to rate limit the login endpoint. Turn this on only behind a reverse proxy such as Coolify, Traefik, or Nginx. On a directly exposed server, a client could spoof these headers to get around the rate limit.

### `CORS_ALLOWED_ORIGINS`

Optional. Comma-separated origins allowed to call the API from a browser. You only need it when the frontend is served from a different origin than the backend, such as a local dev server. The shipped image serves both from the same origin.

Default: `http://localhost:5173,http://127.0.0.1:5173`

## Authentication

> **Authentication is optional.** If these variables are unset, LogDeck runs without authentication, which is fine for local development or a trusted network. You can also enable it from the Settings page with no environment variables at all, and LogDeck stores those credentials in the config file. Setting the variables here pins auth so the UI can't change it.

`JWT_SECRET`, `ADMIN_USERNAME`, and `ADMIN_PASSWORD` go together. Set all of them or none of them. Setting only some is a startup error.

### `JWT_SECRET`

Required for auth from the environment. The secret key that signs session tokens. Sessions last 7 days.

```bash
JWT_SECRET=your-super-secret-key-change-this-to-something-random-min-32-chars
```

Generate a random secret:

```bash
openssl rand -base64 32
```

### `ADMIN_USERNAME`

Required for auth from the environment. The admin username for logging in.

```bash
ADMIN_USERNAME=admin
```

### `ADMIN_PASSWORD`

Required for auth from the environment. A bcrypt hash of the admin password, never plain text. LogDeck validates the hash at startup and refuses to start if it is malformed. See [password hashing](#password-hashing) to generate one.

### `ADMIN_PASSWORD_SALT`

Legacy and optional. Older LogDeck deployments hashed the admin password as SHA256(password + salt). LogDeck still accepts that combination, with `ADMIN_PASSWORD_SALT` set and `ADMIN_PASSWORD` holding the resulting hex digest, so existing setups keep working. Use bcrypt for new deployments and leave this unset.

## Coolify integration

> **Only needed for Coolify-managed servers.** If you deploy containers through Coolify, this integration syncs environment variable changes made in LogDeck to Coolify, so they persist across redeployments. Without it, the next Coolify redeploy loses those changes.

### `COOLIFY_CONFIGS`

Required for Coolify integration. Per-host Coolify configuration. Each entry maps a host name from `DOCKER_HOSTS` to a Coolify instance URL and API token. Generate API tokens in your Coolify dashboard under Settings > API Tokens.

```bash
# Format: hostName|apiURL|apiToken,hostName|apiURL|apiToken
# Single host
COOLIFY_CONFIGS=local|https://your-coolify-instance.com|your-api-token

# Multiple hosts with different Coolify instances
COOLIFY_CONFIGS=prod|https://coolify-prod.example.com|token-abc,staging|https://coolify-staging.example.com|token-xyz
```

How it works:

- LogDeck detects Coolify-managed containers automatically through their Docker labels
- When you update environment variables, LogDeck syncs the change to the Coolify API
- Sync is best-effort. If the Coolify API is unreachable, the container update still succeeds
- Coolify-managed containers get a badge in the container list

## Password hashing

When you configure authentication through the environment, `ADMIN_PASSWORD` must hold a bcrypt hash, never a plain-text password. LogDeck checks the hash at startup and exits with an error if it is not a valid bcrypt string.

If you enable authentication from the Settings page instead, you type the password into the form and LogDeck hashes it and stores it in the config file for you. There is nothing to generate.

### Generate a bcrypt hash

```bash
htpasswd -bnBC 10 '' yourPassword | tr -d ':'
# $2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy
```

`htpasswd` ships with `apache2-utils` on Debian and Ubuntu and `httpd-tools` on RHEL, and comes preinstalled on macOS.

> **Treat the hash and the JWT secret like passwords.** A bcrypt hash contains `$` characters, so escape them as `$$` in a compose file or keep the hash in an `.env` file. Keep both out of version control. Use a different `JWT_SECRET` per deployment, and expect every existing session to end when you change it. The legacy SHA256(password + salt) scheme still works when `ADMIN_PASSWORD_SALT` is set, but prefer bcrypt for new deployments.

## Complete example

A `docker-compose.yml` with every configuration option:

```yaml
services:
  logdeck:
    image: amoabakelvin/logdeck:latest
    container_name: logdeck
    ports:
      - "8123:8080"
    volumes:
      # Docker socket for container management
      - /var/run/docker.sock:/var/run/docker.sock
      # /proc for system stats
      - /proc:/host/proc:ro
      # Config file, stored logs, and alert history. Do not skip this
      - logdeck-data:/data
      # SSH keys, if you use ssh:// hosts
      # - ~/.ssh:/root/.ssh:ro
    environment:
      # Docker hosts (local + remote example)
      DOCKER_HOSTS: "local=unix:///var/run/docker.sock,prod=ssh://deploy@prod.example.com"

      # Authentication (optional - remove to run without auth,
      # or enable it from the Settings page instead)
      JWT_SECRET: "your-super-secret-key-min-32-characters-long"
      ADMIN_USERNAME: "admin"
      ADMIN_PASSWORD: "your-bcrypt-hash" # $$ escapes the $ in a compose file

      # Log persistence (optional - on by default)
      # LOG_STORE_PER_CONTAINER_MB: "50"
      # LOG_STORE_TOTAL_MB: "1024"

      # Read-only mode (optional)
      # READONLY_MODE: "true"

      # Behind a reverse proxy (optional)
      # TRUST_PROXY_HEADERS: "true"

      # Coolify integration (optional - host names must match DOCKER_HOSTS)
      # COOLIFY_CONFIGS: "local|https://your-coolify-instance.com|your-api-token"
    restart: unless-stopped

volumes:
  logdeck-data:
```

The server exposes `GET /api/v1/healthz` for an external health check. The runtime image is minimal and ships no `curl` or `wget`, so a compose `healthcheck` has to come from outside the container.

## Read-only mode

Read-only mode stops LogDeck from changing anything on your containers. Use it where you want to read logs without touching what is running, such as production. When it is on, LogDeck blocks these for everyone, whether they use a session or a token:

- Starting, stopping, restarting, and removing containers
- Compose stack start, stop, and restart
- Environment variable and resource-limit edits
- The web terminal

Reading is unaffected, so logs, history, stats, events, and container details all work as usual. LogDeck's own settings and alert rules also stay editable, because read-only mode covers your containers, not LogDeck's configuration.

Toggle it from the Settings page, or pin it with an environment variable:

```bash
READONLY_MODE=true
```

When `READONLY_MODE` is set, it takes precedence and the UI toggle is disabled.

## API tokens

API tokens give the [LogDeck CLI](/docs/cli), the [MCP server](/docs/mcp), and other tools their own credentials, so you never share your admin login. You manage them entirely in the UI, with no environment variables:

- Create and revoke tokens under Settings > API Access
- Tokens start with `ldk_` and are shown in full only once, at creation
- LogDeck stores only a hash, so a lost token can't be recovered, only revoked and replaced
- Requests authenticate with an `Authorization: Bearer <token>` header

### Scopes

Each token has one of two scopes.

`admin` has full access, the same as a logged-in admin session.

`read` is read-only. Give it to CI jobs, dashboards, and AI agents that only need to look. A read token can:

- Read live logs, stored log history, container details, stats, and events
- List images, volumes, networks, and hosts

A read token cannot:

- Change anything. The server rejects every request that is not a `GET` with `403`, so there is no start, stop, restart, or remove, no stack actions, no environment or resource edits, and no changes to settings or alert rules
- Open the web terminal
- Read a container's environment variables, because they carry secrets
- Read the settings endpoint, which exposes host topology and the token inventory
- Read anything under `/alerts`. Rules, channels, and history are all denied, because channel URLs and tokens, such as Slack and Discord webhooks or bot tokens, are secrets

Tokens only matter when authentication is enabled. On an open instance, the API is reachable without them.

## Docker socket permissions

LogDeck needs access to the Docker socket to work with containers.

### Security best practices

- Run LogDeck only on trusted networks
- Enable authentication if untrusted users can reach LogDeck
- If you only need to read logs, without container management, mount the socket read-only with `:ro`
- Use Docker's built-in authorization plugins for fine-grained access control
- In production, keep LogDeck behind a reverse proxy with TLS

### Permission issues

If LogDeck gets permission errors on the Docker socket, make sure the user running it can access the socket:

```bash
# Check socket permissions
ls -l /var/run/docker.sock

# If needed, add user to docker group (Linux)
sudo usermod -aG docker $USER
```

## Reverse proxy

In production, run LogDeck behind a reverse proxy such as Nginx or Traefik with TLS enabled. The proxy must forward WebSocket upgrade headers, as the Nginx example does.

### Nginx

```nginx
server {
    listen 443 ssl http2;
    server_name logdeck.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:8123;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### Traefik

With Docker labels:

```yaml
services:
  logdeck:
    image: amoabakelvin/logdeck:latest
    container_name: logdeck
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /proc:/host/proc:ro
      - logdeck-data:/data
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.logdeck.rule=Host(`logdeck.example.com`)"
      - "traefik.http.routers.logdeck.entrypoints=websecure"
      - "traefik.http.routers.logdeck.tls.certresolver=letsencrypt"
      - "traefik.http.services.logdeck.loadbalancer.server.port=8080"
    restart: unless-stopped

volumes:
  logdeck-data:
```
