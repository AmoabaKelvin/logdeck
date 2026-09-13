LogDeck runs as one container, `amoabakelvin/logdeck`, next to the containers it watches. This page covers installing it with Docker Compose or `docker run`, connecting more hosts, updating, and fixing common problems.

## Docker Compose

Docker Compose is the recommended way to run LogDeck, because the whole setup lives in one file you can edit and redeploy.

### Step 1: Create docker-compose.yml

Create a `docker-compose.yml` file with this content:

```yaml
services:
  logdeck:
    image: amoabakelvin/logdeck:latest
    container_name: logdeck
    ports:
      - "8123:8080"
    environment:
      # Optional: Manage multiple Docker hosts
      # DOCKER_HOSTS: local=unix:///var/run/docker.sock,prod=ssh://deploy@prod.example.com

      # Optional: Enable authentication (or enable it later in Settings)
      # JWT_SECRET: your-super-secret-key-min-32-chars
      # ADMIN_USERNAME: admin
      # ADMIN_PASSWORD: your-bcrypt-hash

      # Optional: Coolify integration (persists env var changes across redeployments)
      # Host names must match DOCKER_HOSTS
      # COOLIFY_CONFIGS: local|https://your-coolify-instance.com|your-api-token
    volumes:
      # Mount the Docker socket for container management
      - /var/run/docker.sock:/var/run/docker.sock
      # Mount /proc for system stats (CPU, memory usage)
      - /proc:/host/proc:ro
      # Persist the config file, stored logs, and alert history
      - logdeck-data:/data
      # Mount SSH keys if you use ssh:// hosts
      # - ~/.ssh:/root/.ssh:ro
    restart: unless-stopped

volumes:
  logdeck-data:
```

> **Do not skip the `/data` volume.** LogDeck keeps its config file (hosts, API tokens, alert rules), its [stored log history](/docs/log-history), and its alert history in `/data`. Without a volume, all of it is written inside the container and lost the moment you recreate it, including every log line you were counting on reading back.

### Step 2: Start LogDeck

```bash
docker compose up -d
```

### Step 3: Check it is running

```bash
docker compose ps
```

### Step 4: Open the interface

Open [http://localhost:8123](http://localhost:8123) in your browser.

## Docker run

To run LogDeck without Compose, use `docker run`.

### Basic deployment

```bash
docker run -d \
  --name logdeck \
  -p 8123:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /proc:/host/proc:ro \
  -v logdeck-data:/data \
  --restart unless-stopped \
  amoabakelvin/logdeck:latest
```

### With authentication

`ADMIN_PASSWORD` takes a bcrypt hash. Generate one with `htpasswd -bnBC 10 '' yourPassword | tr -d ':'`, and see [password hashing](/docs/configuration#password-hashing) for details.

```bash
docker run -d \
  --name logdeck \
  -p 8123:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /proc:/host/proc:ro \
  -v logdeck-data:/data \
  -e JWT_SECRET=your-super-secret-key-min-32-chars \
  -e ADMIN_USERNAME=admin \
  -e ADMIN_PASSWORD='your-bcrypt-hash' \
  --restart unless-stopped \
  amoabakelvin/logdeck:latest
```

### Multiple Docker hosts

To manage more than one Docker daemon, set `DOCKER_HOSTS` to comma-separated `name=host` entries.

```bash
# Local Docker + remote SSH host
export DOCKER_HOSTS="local=unix:///var/run/docker.sock,prod=ssh://deploy@prod.example.com"
# Then start LogDeck with docker run or docker compose
```

For `ssh://` targets, mount your SSH keys, for example `~/.ssh`, or forward your SSH agent socket into the container.

## Environment variables

Every variable is optional, and LogDeck runs with none of them set. The [configuration reference](/docs/configuration) covers each one in full, along with the config file that the Settings page writes.

- [`DOCKER_HOSTS`](/docs/configuration#docker_hosts): hosts to manage, as `name=host` entries with `unix://`, `tcp://`, or `ssh://` URLs. When unset, LogDeck auto-detects a local Docker or Podman socket.
- [`CONFIG_PATH`](/docs/configuration#config_path): path to the JSON config file. Its directory also holds the log store and alert history. Default `/data/config.json`.
- [`READONLY_MODE`](/docs/configuration#readonly_mode): `true` blocks container actions, stack actions, environment and resource edits, and the web terminal.
- [`LOG_STORE_ENABLED`, `LOG_STORE_PER_CONTAINER_MB`, `LOG_STORE_TOTAL_MB`](/docs/configuration#log-persistence): log persistence is on by default, and these turn it off or change its retention caps. Defaults `true`, `50` MB, and `1024` MB.
- [`JWT_SECRET`, `ADMIN_USERNAME`, `ADMIN_PASSWORD`](/docs/configuration#authentication): pin authentication from the environment. Leave them unset to run without authentication, or to enable it from the Settings page instead.
- [`COOLIFY_CONFIGS`](/docs/configuration#coolify-integration): per-host Coolify configuration in `hostName|apiURL|apiToken` format. Host names must match those in `DOCKER_HOSTS`.

## Docker socket access

> **The Docker socket gives full access to the Docker daemon.** Mounting `/var/run/docker.sock` hands LogDeck control of your Docker daemon. Only run LogDeck on trusted networks, or enable authentication to protect access.

LogDeck needs write access to the Docker socket for container management, such as start, stop, and restart. If you only need to read logs, mount the socket read-only with `:ro` and turn on [read-only mode](/docs/configuration#read-only-mode).

## Updating LogDeck

### With Docker Compose

```bash
docker compose pull
docker compose up -d
```

### With docker run

```bash
docker stop logdeck
docker rm logdeck
docker pull amoabakelvin/logdeck:latest
# Then run your docker run command again
```

## Installing the CLI

The optional `logdeck` command-line client talks to your running LogDeck server, so you can read logs, check stats, and manage containers from the terminal or from scripts.

```bash
curl -fsSL https://raw.githubusercontent.com/AmoabaKelvin/logdeck/main/install.sh | sh
```

It installs a single binary for macOS or Linux, on amd64 or arm64. The [CLI reference](/docs/cli) covers connecting it to your server and every command.

## Troubleshooting

### Container won't start

Check its logs:

```bash
docker logs logdeck
```

### No containers show up

Check that the Docker socket is mounted:

```bash
docker inspect logdeck | grep docker.sock
```

### Port already in use

If port 8123 is taken, publish a different host port in your `docker-compose.yml` or `docker run` command:

```yaml
- "8124:8080"  # Use port 8124 instead
```

Still stuck? [Open an issue on GitHub](https://github.com/AmoabaKelvin/logdeck/issues) with details about your setup and the error you see.
