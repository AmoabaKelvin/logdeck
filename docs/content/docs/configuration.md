LogDeck runs with no configuration at all, and every environment variable on this page is optional. This is the reference for configuring it: the config file and data directory, every environment variable, authentication, API tokens, read-only mode, and reverse proxy setups.

## Two ways to configure

LogDeck reads its configuration from environment variables and from a JSON config file that the Settings page writes. Environment variables win. A value pinned by the environment shows in the UI but can't be changed there, which is what you want when the deployment, not the admin, should have the final say.

Hosts, Coolify hosts, read-only mode, and authentication can come from either source. API tokens, alert rules, and log retention live only in the config file. Manage tokens in the UI, alert rules in the UI or the CLI, and retention through the config file or its [environment overrides](#log-persistence).

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

## Server

### `DOCKER_HOSTS`

Optional. A comma-separated list of hosts to manage. Each entry uses `name=host` format and supports `unix://`, `tcp://`, and `ssh://` URLs.

```bash
# Local only
DOCKER_HOSTS=local=unix:///var/run/docker.sock

# Mix of local and remote TCP
DOCKER_HOSTS=local=unix:///var/run/docker.sock,staging=tcp://192.168.1.100:2375

# SSH connection (mount your SSH keys or forward agent)
DOCKER_HOSTS=local=unix:///var/run/docker.sock,prod=ssh://deploy@prod.example.com
```

Host names appear in the UI and the container list, so you always know which daemon you are working with. Hosts defined here can't be edited or removed from the Settings page, and hosts added in Settings are merged with them. For `ssh://` targets, mount your SSH keys or forward an SSH agent into the container.

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
      ADMIN_PASSWORD: "your-bcrypt-hash"   # $$ escapes the $ in a compose file

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
