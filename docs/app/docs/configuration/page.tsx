import type { Metadata } from "next"
import { CodeBlock } from "@/components/landing/code-block"
import { Separator } from "@/components/ui/separator"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { AlertTriangle, CloudCog, Database, Globe, Info, Lock, Server } from "lucide-react"

export const metadata: Metadata = {
  title: "Configuration",
  description: "Configure LogDeck: the /data directory, DOCKER_HOSTS for multi-host setups, authentication, scoped API tokens, log retention, Coolify sync, read-only mode, and reverse proxy examples.",
  alternates: { canonical: "/docs/configuration" },
}

export default function ConfigurationPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="scroll-m-20 text-4xl font-bold tracking-tight">
          Configuration
        </h1>
        <p className="text-lg text-muted-foreground mt-2">
          Configure LogDeck to match your environment and security requirements.
        </p>
      </div>

      <Separator />

      <div className="prose prose-neutral dark:prose-invert max-w-none">
        <h2 className="mb-4 text-3xl font-bold tracking-tight">Two ways to configure</h2>
        <p className="mb-4 text-base">
          LogDeck reads its configuration from <strong>environment variables</strong> and from a{" "}
          <strong>JSON config file</strong> that the Settings page writes. Environment variables win:
          anything pinned by the environment is shown in the UI but cannot be changed there, which is
          what you want when the deployment, not the admin, should have the final say.
        </p>
        <p className="mb-6 text-base">
          Hosts, Coolify hosts, read-only mode, and authentication can come from either source. API
          tokens, alert rules, and log retention live only in the config file — set through the UI,
          the CLI, or (for retention) an environment override.
        </p>

        <h2 className="mb-4 mt-10 text-3xl font-bold tracking-tight">The data directory</h2>
        <p className="mb-4 text-base">
          Everything LogDeck persists lives in one directory — the directory of its config file,{" "}
          <code>/data</code> by default:
        </p>
        <ul className="mb-4 space-y-2">
          <li>
            <code>config.json</code> — hosts, Coolify hosts, read-only mode, auth, API tokens, alert
            rules, and log-store settings
          </li>
          <li>
            <code>logs.db</code> — the SQLite <a href="/docs/log-history">log store</a>
          </li>
          <li>
            <code>alerts-history.json</code> — recently fired <a href="/docs/alerting">alerts</a>
          </li>
        </ul>
        <p className="mb-4 text-base">
          <strong>Mount it as a volume.</strong> Without one, all of the above is written inside the
          container and destroyed the next time you recreate it — you would lose your API tokens,
          your alert rules, and all stored log history.
        </p>

        <div className="not-prose mb-6">
          <CodeBlock code={`volumes:
  - logdeck-data:/data`} language="yaml" />
        </div>

        <p className="mb-6 text-base">
          Set <code>CONFIG_PATH</code> to move the config file (and with it the whole directory)
          somewhere else, for example <code>CONFIG_PATH=/config/logdeck.json</code>.
        </p>

        <h2 className="mb-4 mt-10 text-3xl font-bold tracking-tight">Environment Variables</h2>
      </div>

      <div className="grid gap-4 mt-6">
        <Card>
          <CardHeader>
            <div className="flex items-start gap-2">
              <Server className="h-5 w-5 text-primary mt-0.5" />
              <div>
                <CardTitle>Server Configuration</CardTitle>
                <CardDescription>Configure the backend server and Docker connection</CardDescription>
              </div>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            <div>
              <div className="flex items-baseline gap-2 mb-1">
                <code className="text-sm font-mono bg-muted px-2 py-1 rounded">DOCKER_HOSTS</code>
                <span className="text-xs text-muted-foreground">Optional</span>
              </div>
              <p className="text-sm text-muted-foreground">
                Comma-separated list of Docker hosts to manage. Each entry uses <code>name=host</code> format and supports <code>unix://</code>, <code>tcp://</code>, and <code>ssh://</code> URLs.
              </p>
              <div className="mt-2">
                <span className="text-xs font-medium">Default:</span>{" "}
                <code className="text-xs bg-muted px-1.5 py-0.5 rounded">local=unix:///var/run/docker.sock</code>
              </div>
              <div className="mt-2">
                <CodeBlock code='DOCKER_HOSTS=local=unix:///var/run/docker.sock' language="bash" />
              </div>
              <div className="mt-2 text-sm">
                <p className="font-medium">Examples:</p>
                <CodeBlock
                  code={`# Local only
DOCKER_HOSTS=local=unix:///var/run/docker.sock

# Mix of local and remote TCP
DOCKER_HOSTS=local=unix:///var/run/docker.sock,staging=tcp://192.168.1.100:2375

# SSH connection (mount your SSH keys or forward agent)
DOCKER_HOSTS=local=unix:///var/run/docker.sock,prod=ssh://deploy@prod.example.com`}
                  language="bash"
                />
              </div>
              <p className="mt-2 text-sm text-muted-foreground">
                Host names appear in the UI and in the container list so you always know which Docker daemon you are interacting with.
                Hosts defined here cannot be edited or removed from the Settings page; hosts added in Settings are merged with them.
              </p>
              <p className="mt-2 text-sm text-muted-foreground">
                Podman is supported through its Docker-compatible API socket. When{" "}
                <code>DOCKER_HOSTS</code> is unset, LogDeck probes for a local socket in order:
                Docker, then rootless Podman (<code>$XDG_RUNTIME_DIR/podman/podman.sock</code>), then
                rootful Podman (<code>/run/podman/podman.sock</code>).
              </p>
            </div>

            <Separator />

            <div>
              <div className="flex items-baseline gap-2 mb-1">
                <code className="text-sm font-mono bg-muted px-2 py-1 rounded">CONFIG_PATH</code>
                <span className="text-xs text-muted-foreground">Optional</span>
              </div>
              <p className="text-sm text-muted-foreground">
                Path to the JSON config file. Its directory also holds the log store and the alert
                history, so it should be on a mounted volume.
              </p>
              <div className="mt-2">
                <span className="text-xs font-medium">Default:</span>{" "}
                <code className="text-xs bg-muted px-1.5 py-0.5 rounded">/data/config.json</code>
              </div>
            </div>

            <Separator />

            <div>
              <div className="flex items-baseline gap-2 mb-1">
                <code className="text-sm font-mono bg-muted px-2 py-1 rounded">READONLY_MODE</code>
                <span className="text-xs text-muted-foreground">Optional</span>
              </div>
              <p className="text-sm text-muted-foreground">
                Set to <code>true</code> to block every mutating operation. See{" "}
                <a href="#read-only-mode">Read-Only Mode</a> below. Default: <code>false</code>.
              </p>
            </div>

            <Separator />

            <div>
              <div className="flex items-baseline gap-2 mb-1">
                <code className="text-sm font-mono bg-muted px-2 py-1 rounded">PORT</code>
                <span className="text-xs text-muted-foreground">Not configurable</span>
              </div>
              <p className="text-sm text-muted-foreground">
                The server always listens on <code>:8080</code> inside the container. Publish it on
                whichever host port you like (<code>-p 8123:8080</code>).
              </p>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <div className="flex items-start gap-2">
              <Database className="h-5 w-5 text-primary mt-0.5" />
              <div>
                <CardTitle>Log Persistence (Optional)</CardTitle>
                <CardDescription>
                  Retention limits for the stored logs that power History mode
                </CardDescription>
              </div>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            <p className="text-sm text-muted-foreground">
              Persistence is <strong>on by default</strong> and needs no configuration. These
              variables override the <code>logStore</code> section of the config file. See{" "}
              <a href="/docs/log-history">Log History</a> for the full picture.
            </p>

            <div>
              <div className="flex items-baseline gap-2 mb-1">
                <code className="text-sm font-mono bg-muted px-2 py-1 rounded">LOG_STORE_ENABLED</code>
                <span className="text-xs text-muted-foreground">Optional</span>
              </div>
              <p className="text-sm text-muted-foreground">
                <code>false</code> disables log persistence entirely: no database file, no History
                mode. Default: <code>true</code>.
              </p>
            </div>

            <div>
              <div className="flex items-baseline gap-2 mb-1">
                <code className="text-sm font-mono bg-muted px-2 py-1 rounded">LOG_STORE_PER_CONTAINER_MB</code>
                <span className="text-xs text-muted-foreground">Optional</span>
              </div>
              <p className="text-sm text-muted-foreground">
                Retention cap per container, in MB. Oldest lines are evicted first. Default:{" "}
                <code>50</code>.
              </p>
            </div>

            <div>
              <div className="flex items-baseline gap-2 mb-1">
                <code className="text-sm font-mono bg-muted px-2 py-1 rounded">LOG_STORE_TOTAL_MB</code>
                <span className="text-xs text-muted-foreground">Optional</span>
              </div>
              <p className="text-sm text-muted-foreground">
                Retention cap for the whole store, in MB. Default: <code>1024</code>.
              </p>
            </div>

            <div className="mt-2">
              <CodeBlock code={`LOG_STORE_ENABLED=true
LOG_STORE_PER_CONTAINER_MB=50
LOG_STORE_TOTAL_MB=1024`} language="bash" />
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <div className="flex items-start gap-2">
              <Globe className="h-5 w-5 text-primary mt-0.5" />
              <div>
                <CardTitle>Proxies and CORS (Optional)</CardTitle>
                <CardDescription>Only needed in specific deployments</CardDescription>
              </div>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            <div>
              <div className="flex items-baseline gap-2 mb-1">
                <code className="text-sm font-mono bg-muted px-2 py-1 rounded">TRUST_PROXY_HEADERS</code>
                <span className="text-xs text-muted-foreground">Optional</span>
              </div>
              <p className="text-sm text-muted-foreground">
                Set to <code>true</code> to trust <code>X-Forwarded-For</code> /{" "}
                <code>X-Real-IP</code> when identifying a client. LogDeck uses the client IP to rate
                limit the login endpoint. Enable this <strong>only</strong> behind a reverse proxy
                (Coolify, Traefik, Nginx); on a directly exposed server a client could spoof these
                headers to sidestep the rate limit.
              </p>
            </div>

            <Separator />

            <div>
              <div className="flex items-baseline gap-2 mb-1">
                <code className="text-sm font-mono bg-muted px-2 py-1 rounded">CORS_ALLOWED_ORIGINS</code>
                <span className="text-xs text-muted-foreground">Optional</span>
              </div>
              <p className="text-sm text-muted-foreground">
                Comma-separated origins allowed to call the API from a browser. Only needed when the
                frontend is served from a different origin than the backend, such as a local dev
                server. The shipped image serves both from the same origin.
              </p>
              <div className="mt-2">
                <span className="text-xs font-medium">Default:</span>{" "}
                <code className="text-xs bg-muted px-1.5 py-0.5 rounded">
                  http://localhost:5173,http://127.0.0.1:5173
                </code>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <div className="flex items-start gap-2">
              <Lock className="h-5 w-5 text-primary mt-0.5" />
              <div>
                <CardTitle>Authentication (Optional)</CardTitle>
                <CardDescription>
                  Secure your LogDeck instance with JWT-based authentication
                </CardDescription>
              </div>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="rounded-lg border border-blue-200 bg-blue-50 dark:border-blue-900 dark:bg-blue-950/20 p-4">
              <div className="flex gap-2">
                <Info className="h-5 w-5 text-blue-600 dark:text-blue-500 shrink-0 mt-0.5" />
                <div className="text-sm text-blue-900 dark:text-blue-200">
                  <p className="font-medium">Authentication is completely optional</p>
                  <p className="mt-1">
                    If these variables are not set, LogDeck runs without authentication — fine for
                    local development or a trusted network. You can also enable it from the{" "}
                    <strong>Settings</strong> page instead, with no environment variables at all;
                    those credentials are stored in the config file. Setting them here pins auth so
                    the UI cannot change it.
                  </p>
                </div>
              </div>
            </div>

            <p className="text-sm text-muted-foreground">
              These three go together: set all of them, or none of them. Setting some but not all is
              a startup error.
            </p>

            <div>
              <div className="flex items-baseline gap-2 mb-1">
                <code className="text-sm font-mono bg-muted px-2 py-1 rounded">JWT_SECRET</code>
                <span className="text-xs text-amber-600 dark:text-amber-500 font-medium">Required for auth</span>
              </div>
              <p className="text-sm text-muted-foreground">
                Secret key used to sign session tokens. Sessions last 7 days.
              </p>
              <div className="mt-2">
                <CodeBlock code='JWT_SECRET=your-super-secret-key-change-this-to-something-random-min-32-chars' language="bash" />
              </div>
              <div className="mt-2 text-sm">
                <p className="font-medium">Generate a random secret:</p>
                <CodeBlock code='openssl rand -base64 32' language="bash" />
              </div>
            </div>

            <Separator />

            <div>
              <div className="flex items-baseline gap-2 mb-1">
                <code className="text-sm font-mono bg-muted px-2 py-1 rounded">ADMIN_USERNAME</code>
                <span className="text-xs text-amber-600 dark:text-amber-500 font-medium">Required for auth</span>
              </div>
              <p className="text-sm text-muted-foreground">
                Admin username for logging in.
              </p>
              <div className="mt-2">
                <CodeBlock code='ADMIN_USERNAME=admin' language="bash" />
              </div>
            </div>

            <Separator />

            <div>
              <div className="flex items-baseline gap-2 mb-1">
                <code className="text-sm font-mono bg-muted px-2 py-1 rounded">ADMIN_PASSWORD</code>
                <span className="text-xs text-amber-600 dark:text-amber-500 font-medium">Required for auth</span>
              </div>
              <p className="text-sm text-muted-foreground">
                A <strong>bcrypt hash</strong> of the admin password — never plain text. LogDeck
                validates the hash at startup and refuses to start if it is malformed.
              </p>
              <div className="mt-2">
                <p className="text-sm font-medium">Generate the hash:</p>
                <CodeBlock code={`htpasswd -bnBC 10 '' yourPassword | tr -d ':'

# ADMIN_PASSWORD=$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy`} language="bash" />
              </div>
              <p className="text-sm text-muted-foreground mt-2">
                <code>htpasswd</code> ships with <code>apache2-utils</code> on Debian/Ubuntu,{" "}
                <code>httpd-tools</code> on RHEL, and is preinstalled on macOS. In a compose file,
                escape the <code>$</code> characters as <code>$$</code>, or keep the hash in an{" "}
                <code>.env</code> file.
              </p>
            </div>

            <Separator />

            <div>
              <div className="flex items-baseline gap-2 mb-1">
                <code className="text-sm font-mono bg-muted px-2 py-1 rounded">ADMIN_PASSWORD_SALT</code>
                <span className="text-xs text-muted-foreground">Legacy</span>
              </div>
              <p className="text-sm text-muted-foreground">
                Older LogDeck deployments hashed the admin password as SHA256(password + salt). That
                combination — <code>ADMIN_PASSWORD_SALT</code> set, with{" "}
                <code>ADMIN_PASSWORD</code> holding the resulting hex digest — is still accepted, so
                existing setups keep working. Use bcrypt for new ones and leave this unset.
              </p>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <div className="flex items-start gap-2">
              <CloudCog className="h-5 w-5 text-primary mt-0.5" />
              <div>
                <CardTitle>Coolify Integration (Optional)</CardTitle>
                <CardDescription>
                  Persist environment variable changes across Coolify redeployments
                </CardDescription>
              </div>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="rounded-lg border border-blue-200 bg-blue-50 dark:border-blue-900 dark:bg-blue-950/20 p-4">
              <div className="flex gap-2">
                <Info className="h-5 w-5 text-blue-600 dark:text-blue-500 shrink-0 mt-0.5" />
                <div className="text-sm text-blue-900 dark:text-blue-200">
                  <p className="font-medium">Only needed for Coolify-managed servers</p>
                  <p className="mt-1">
                    If you deploy containers through Coolify, enabling this integration ensures that
                    environment variable changes made in LogDeck are synced to Coolify and persist
                    across redeployments. Without it, changes are lost when Coolify redeploys.
                  </p>
                </div>
              </div>
            </div>

            <div>
              <div className="flex items-baseline gap-2 mb-1">
                <code className="text-sm font-mono bg-muted px-2 py-1 rounded">COOLIFY_CONFIGS</code>
                <span className="text-xs text-amber-600 dark:text-amber-500 font-medium">Required for Coolify</span>
              </div>
              <p className="text-sm text-muted-foreground">
                Per-host Coolify configuration. Each entry maps a Docker host name (from <code>DOCKER_HOSTS</code>)
                to a Coolify instance URL and API token. Generate API tokens from your Coolify dashboard
                under <strong>Settings &rarr; API Tokens</strong>.
              </p>
              <div className="mt-2">
                <CodeBlock code={`# Format: hostName|apiURL|apiToken,hostName|apiURL|apiToken
# Single host
COOLIFY_CONFIGS=local|https://your-coolify-instance.com|your-api-token

# Multiple hosts with different Coolify instances
COOLIFY_CONFIGS=prod|https://coolify-prod.example.com|token-abc,staging|https://coolify-staging.example.com|token-xyz`} language="bash" />
              </div>
            </div>

            <Separator />

            <div>
              <p className="text-sm font-medium mb-2">How it works</p>
              <ul className="text-sm text-muted-foreground space-y-1.5">
                <li className="flex items-start gap-2">
                  <div className="h-1.5 w-1.5 rounded-full bg-primary mt-1.5 shrink-0" />
                  <span>LogDeck detects Coolify-managed containers automatically via Docker labels</span>
                </li>
                <li className="flex items-start gap-2">
                  <div className="h-1.5 w-1.5 rounded-full bg-primary mt-1.5 shrink-0" />
                  <span>When you update environment variables, changes are synced to the Coolify API</span>
                </li>
                <li className="flex items-start gap-2">
                  <div className="h-1.5 w-1.5 rounded-full bg-primary mt-1.5 shrink-0" />
                  <span>Sync is best-effort: if the Coolify API is unreachable, the container update still succeeds</span>
                </li>
                <li className="flex items-start gap-2">
                  <div className="h-1.5 w-1.5 rounded-full bg-primary mt-1.5 shrink-0" />
                  <span>Coolify-managed containers are marked with a badge in the container list</span>
                </li>
              </ul>
            </div>
          </CardContent>
        </Card>
      </div>

      <Separator className="my-12" />

      <div className="prose prose-neutral dark:prose-invert max-w-none">
        <h2 className="mb-4 text-3xl font-bold tracking-tight">Password Hashing</h2>
        <p className="mb-6 text-base">
          When you configure authentication through the environment, <code>ADMIN_PASSWORD</code>{" "}
          must hold a <strong>bcrypt hash</strong> — never a plain-text password. LogDeck checks the
          hash at startup and exits with an error if it is not a valid bcrypt string.
        </p>
        <p className="mb-6 text-base">
          If instead you enable authentication from the <strong>Settings</strong> page, you type the
          password into the form and LogDeck hashes and stores it in the config file for you. There
          is nothing to generate.
        </p>

        <h3 className="mb-4 mt-8 text-xl font-semibold">Generate a bcrypt hash</h3>

        <div className="not-prose mb-6">
          <CodeBlock
            code={`htpasswd -bnBC 10 '' yourPassword | tr -d ':'
# $2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy`}
            language="bash"
          />
        </div>
        <p className="mb-6 text-sm text-muted-foreground">
          <code>htpasswd</code> ships with <code>apache2-utils</code> on Debian/Ubuntu,{" "}
          <code>httpd-tools</code> on RHEL, and is preinstalled on macOS.
        </p>

        <div className="not-prose mt-8">
          <Card className="border-amber-200 bg-amber-50 dark:border-amber-900 dark:bg-amber-950/20">
            <CardHeader>
              <div className="flex items-start gap-2">
                <AlertTriangle className="h-5 w-5 text-amber-600 dark:text-amber-500 mt-0.5" />
                <div>
                  <CardTitle className="text-amber-900 dark:text-amber-200">
                    Important Security Notes
                  </CardTitle>
                </div>
              </div>
            </CardHeader>
            <CardContent className="text-sm text-amber-900 dark:text-amber-200 space-y-2">
              <p>
                • A bcrypt hash contains <code>$</code> characters. In a compose file, escape them as{" "}
                <code>$$</code>, or put the hash in an <code>.env</code> file instead.
              </p>
              <p>
                • Treat the hash and the JWT secret like passwords — keep them out of version
                control.
              </p>
              <p>
                • Use a different <code>JWT_SECRET</code> per deployment. Changing it invalidates
                every existing session.
              </p>
              <p>
                • The legacy SHA256(password + salt) scheme is still accepted when{" "}
                <code>ADMIN_PASSWORD_SALT</code> is set, so older deployments keep working. Prefer
                bcrypt for new ones.
              </p>
            </CardContent>
          </Card>
        </div>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">Complete Example</h2>
        <p className="mb-6 text-base">Here&apos;s a complete <code>docker-compose.yml</code> with all configuration options:</p>

        <div className="mb-8">
          <CodeBlock
          code={`services:
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
      # Config file, stored logs, and alert history — do not skip this
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
  logdeck-data:`}
          language="yaml"
          />
        </div>

        <p className="mb-8 text-base">
          The server exposes <code>GET /api/v1/healthz</code> if you want to wire up an external
          health check. The runtime image is minimal and ships no <code>curl</code> or{" "}
          <code>wget</code>, so a compose <code>healthcheck</code> has to come from outside the
          container.
        </p>

        <Separator className="my-12" />

        <h2 id="read-only-mode" className="mb-4 text-3xl font-bold tracking-tight">Read-Only Mode</h2>
        <p className="mb-4 text-base">
          Read-only mode prevents LogDeck from changing anything on your containers. It is useful in
          production, where you want to read logs but not touch what is running. When it is on, these
          are blocked for everyone, session or token:
        </p>
        <ul className="mb-4 space-y-2">
          <li>Starting, stopping, restarting, and removing containers</li>
          <li>Compose stack start, stop, and restart</li>
          <li>Environment variable and resource-limit edits</li>
          <li>The web terminal (opening a shell in a container)</li>
        </ul>
        <p className="mb-4 text-base">
          Reading is unaffected: logs, history, stats, events, and container details all work as
          usual. LogDeck&apos;s own settings and alert rules also remain editable — read-only mode is
          about your containers, not about LogDeck&apos;s configuration.
        </p>
        <p className="mb-4 text-base">
          Toggle it from the <strong>Settings</strong> page in the UI, or pin it with an environment variable:
        </p>

        <div className="not-prose mb-4">
          <CodeBlock code="READONLY_MODE=true" language="bash" />
        </div>

        <p className="mb-8 text-base">
          When <code>READONLY_MODE</code> is set, it takes precedence and the UI toggle is disabled —
          useful when the environment, not the admin, should have the final say.
        </p>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">API Tokens</h2>
        <p className="mb-4 text-base">
          API tokens give the <a href="/docs/cli">LogDeck CLI</a> and external tools their own
          credentials, so you never have to share your admin login. They are managed entirely in
          the UI — no environment variables involved:
        </p>
        <ul className="mb-4 space-y-2">
          <li>Create and revoke tokens under <strong>Settings &rarr; API Access</strong></li>
          <li>Tokens are prefixed <code>ldk_</code> and shown in full only once, at creation</li>
          <li>Only a hash is stored; a lost token cannot be recovered, only revoked and replaced</li>
          <li>Requests authenticate with an <code>Authorization: Bearer &lt;token&gt;</code> header</li>
        </ul>

        <h3 className="mb-4 mt-8 text-xl font-semibold">Scopes</h3>
        <p className="mb-4 text-base">
          Each token is created with one of two scopes.
        </p>
        <p className="mb-4 text-base">
          <strong>admin</strong> — full access, equivalent to a logged-in admin session.
        </p>
        <p className="mb-2 text-base">
          <strong>read</strong> — read-only. Give this one to CI jobs, dashboards, and AI agents that
          only need to look. A read token <em>can</em>:
        </p>
        <ul className="mb-4 space-y-2">
          <li>Read live logs, stored log history, container details, stats, and events</li>
          <li>List images, volumes, networks, and hosts</li>
          <li>Read alert rules, the alert webhook URL, and alert history</li>
        </ul>
        <p className="mb-2 text-base">A read token <strong>cannot</strong>:</p>
        <ul className="mb-4 space-y-2">
          <li>
            Mutate anything — every request that is not a <code>GET</code> is rejected with{" "}
            <code>403</code>, so no start/stop/restart/remove, no stack actions, no environment or
            resource edits, and no changes to settings or alert rules
          </li>
          <li>Open the web terminal</li>
          <li>Read a container&apos;s environment variables (they carry secrets)</li>
          <li>Read the settings endpoint, which exposes host topology and the token inventory</li>
        </ul>
        <p className="mb-4 text-base">
          Note that the webhook URL <em>is</em> readable with a read token. If your webhook URL
          embeds a secret (Slack and Discord URLs do), treat a read token as sensitive accordingly.
        </p>
        <p className="mb-8 text-base">
          Tokens only matter when authentication is enabled; on an open instance the API is
          reachable without them.
        </p>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">Docker Socket Permissions</h2>
        <p className="mb-6 text-base">
          LogDeck needs access to the Docker socket to interact with containers. Here are some
          important considerations:
        </p>

        <h3 className="mb-4 mt-8 text-xl font-semibold">Security Best Practices</h3>
        <ul className="mb-8 space-y-2">
          <li>Run LogDeck only on trusted networks</li>
          <li>Enable authentication if exposing LogDeck to untrusted users</li>
          <li>If you only need log viewing (no container management), mount the socket as read-only (<code>:ro</code>)</li>
          <li>Use Docker&apos;s built-in authorization plugins for fine-grained access control</li>
          <li>Keep LogDeck behind a reverse proxy with TLS in production</li>
        </ul>

        <h3 className="mb-4 mt-8 text-xl font-semibold">Permission Issues</h3>
        <p className="mb-4 text-sm">
          If you encounter permission errors accessing the Docker socket, ensure the user running
          LogDeck has appropriate permissions:
        </p>
        <div className="mb-8">
          <CodeBlock
          code={`# Check socket permissions
ls -l /var/run/docker.sock

# If needed, add user to docker group (Linux)
sudo usermod -aG docker $USER`}
          language="bash"
          />
        </div>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">Reverse Proxy Setup</h2>
        <p className="mb-8 text-base">
          For production deployments, it&apos;s recommended to run LogDeck behind a reverse proxy
          like Nginx or Traefik with TLS enabled.
        </p>

        <h3 className="mb-4 mt-8 text-xl font-semibold">Nginx Example</h3>
        <div className="mb-8">
          <CodeBlock
          code={`server {
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
}`}
          language="nginx"
          />
        </div>

        <h3 className="mb-4 mt-8 text-xl font-semibold">Traefik Example (Docker Labels)</h3>
        <div className="mb-8">
          <CodeBlock
          code={`services:
  logdeck:
    image: amoabakelvin/logdeck:latest
    container_name: logdeck
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /proc:/host/proc:ro
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.logdeck.rule=Host(\`logdeck.example.com\`)"
      - "traefik.http.routers.logdeck.entrypoints=websecure"
      - "traefik.http.routers.logdeck.tls.certresolver=letsencrypt"
      - "traefik.http.services.logdeck.loadbalancer.server.port=8080"
    restart: unless-stopped`}
          language="yaml"
          />
        </div>
      </div>
    </div>
  )
}
