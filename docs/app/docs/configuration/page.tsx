import type { Metadata } from "next"
import { CodeBlock } from "@/components/landing/code-block"
import { Separator } from "@/components/ui/separator"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { AlertTriangle, Info, Lock, Server } from "lucide-react"

export const metadata: Metadata = {
  title: "Configuration",
  description: "Complete configuration guide for LogDeck including environment variables, authentication, and Docker setup.",
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
        <h2 className="mb-4 text-3xl font-bold tracking-tight">Environment Variables</h2>
        <p className="mb-6 text-base">
          LogDeck is configured entirely through environment variables. This makes it easy to deploy
          across different environments with different configurations.
        </p>
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
                <code className="text-sm font-mono bg-muted px-2 py-1 rounded">BACKEND_PORT</code>
                <span className="text-xs text-muted-foreground">Optional</span>
              </div>
              <p className="text-sm text-muted-foreground">
                Port for the backend server to listen on.
              </p>
              <div className="mt-2">
                <span className="text-xs font-medium">Default:</span>{" "}
                <code className="text-xs bg-muted px-1.5 py-0.5 rounded">8080</code>
              </div>
              <div className="mt-2">
                <CodeBlock code='BACKEND_PORT=8080' language="bash" />
              </div>
            </div>

            <Separator />

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
              </p>
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
                    If these variables are not set, LogDeck will run without authentication.
                    This is fine for local development or trusted networks.
                  </p>
                </div>
              </div>
            </div>

            <div>
              <div className="flex items-baseline gap-2 mb-1">
                <code className="text-sm font-mono bg-muted px-2 py-1 rounded">JWT_SECRET</code>
                <span className="text-xs text-amber-600 dark:text-amber-500 font-medium">Required for auth</span>
              </div>
              <p className="text-sm text-muted-foreground">
                Secret key used to sign JWT tokens. Must be at least 32 characters long.
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
                <code className="text-sm font-mono bg-muted px-2 py-1 rounded">ADMIN_PASSWORD_SALT</code>
                <span className="text-xs text-amber-600 dark:text-amber-500 font-medium">Required for auth</span>
              </div>
              <p className="text-sm text-muted-foreground">
                Random salt for password hashing. Use a strong, random string.
              </p>
              <div className="mt-2">
                <CodeBlock code='ADMIN_PASSWORD_SALT=your-random-salt-change-this' language="bash" />
              </div>
              <div className="mt-2 text-sm">
                <p className="font-medium">Generate a random salt:</p>
                <CodeBlock code='openssl rand -hex 32' language="bash" />
              </div>
            </div>

            <Separator />

            <div>
              <div className="flex items-baseline gap-2 mb-1">
                <code className="text-sm font-mono bg-muted px-2 py-1 rounded">ADMIN_PASSWORD</code>
                <span className="text-xs text-amber-600 dark:text-amber-500 font-medium">Required for auth</span>
              </div>
              <p className="text-sm text-muted-foreground">
                SHA256 hash of (password + salt). <strong>Do not use plain text!</strong>
              </p>
              <div className="mt-2">
                <CodeBlock code='ADMIN_PASSWORD=your-sha256-hash' language="bash" />
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      <Separator className="my-12" />

      <div className="prose prose-neutral dark:prose-invert max-w-none">
        <h2 className="mb-4 text-3xl font-bold tracking-tight">Password Hashing</h2>
        <p className="mb-6 text-base">
          For security, LogDeck uses SHA256 hashing with a salt. Never use plain text passwords
          in the <code>ADMIN_PASSWORD</code> environment variable. The password is hashed as SHA256(password + salt).
        </p>

        <h3 className="mb-4 mt-8 text-xl font-semibold">Quick Method: Using Shell Commands</h3>
        <p className="mb-4 text-sm">Generate both salt and password hash in one go:</p>

        <div className="not-prose mb-4">
          <div className="space-y-3">
            <div>
              <p className="text-sm font-medium mb-2">Step 1: Generate a random salt</p>
              <CodeBlock code='openssl rand -hex 32' language="bash" />
              <p className="text-sm text-muted-foreground mt-1">
                Save this output as your <code>ADMIN_PASSWORD_SALT</code>
              </p>
            </div>

            <div>
              <p className="text-sm font-medium mb-2">Step 2: Generate the password hash</p>
              <CodeBlock
                code={`# Replace YOUR_PASSWORD and YOUR_SALT with your actual values
echo -n "YOUR_PASSWORDYOUR_SALT" | shasum -a 256 | awk '{print $1}'

# Example: If password is "admin123" and salt is "mysalt", run:
echo -n "admin123mysalt" | shasum -a 256 | awk '{print $1}'`}
                language="bash"
              />
              <p className="text-sm text-muted-foreground mt-1">
                Save this output as your <code>ADMIN_PASSWORD</code>
              </p>
            </div>
          </div>
        </div>

        <h3 className="mb-4 mt-8 text-xl font-semibold">Alternative: Using Python</h3>
        <div className="mb-6">
          <CodeBlock
          code={`import hashlib

password = "your-password"
salt = "your-salt"
hash_value = hashlib.sha256((password + salt).encode()).hexdigest()
print(hash_value)`}
          language="python"
          />
        </div>

        <h3 className="mb-4 mt-8 text-xl font-semibold">Alternative: Using Node.js</h3>
        <div className="mb-6">
          <CodeBlock
          code={`const crypto = require('crypto');

const password = 'your-password';
const salt = 'your-salt';
const hash = crypto.createHash('sha256').update(password + salt).digest('hex');
console.log(hash);`}
          language="javascript"
          />
        </div>

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
                • Generate a unique, random salt for each deployment
              </p>
              <p>
                • Never use the same salt across different environments
              </p>
              <p>
                • Keep your salt and password hash secure - treat them like passwords
              </p>
              <p>
                • The hash format is: SHA256(password + salt), where strings are concatenated directly
              </p>
            </CardContent>
          </Card>
        </div>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">Complete Example</h2>
        <p className="mb-6 text-base">Here&apos;s a complete <code>docker-compose.yml</code> with all configuration options:</p>

        <div className="mb-8">
          <CodeBlock
          code={`version: '3.8'

services:
  logdeck:
    image: logdeck/logdeck:latest
    container_name: logdeck
    ports:
      - "8123:8123"
    volumes:
      # Mount Docker socket
      - /var/run/docker.sock:/var/run/docker.sock:ro
    environment:
      # Server configuration
      BACKEND_PORT: 8080

      # Docker hosts (local + remote example)
      DOCKER_HOSTS: "local=unix:///var/run/docker.sock,prod=ssh://deploy@prod.example.com"

      # Authentication (optional - remove to disable auth)
      JWT_SECRET: "your-super-secret-key-min-32-characters-long"
      ADMIN_USERNAME: "admin"
      ADMIN_PASSWORD_SALT: "your-random-salt-change-this"
      ADMIN_PASSWORD: "your-sha256-hash"
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:8123"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s`}
          language="yaml"
          />
        </div>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">Read-Only Mode</h2>
        <p className="mb-4 text-base">
          LogDeck supports a read-only mode that prevents any container management operations.
          This is useful in production environments where you want to view logs but not modify containers.
        </p>
        <p className="mb-8 text-base">
          Read-only mode is controlled via a feature flag in the backend code. When enabled, all
          mutating operations (start, stop, restart, remove, env updates) will be blocked.
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
          <li>Consider mounting the socket as read-only (<code>:ro</code>) if you only need log viewing</li>
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
          code={`version: '3.8'

services:
  logdeck:
    image: logdeck/logdeck:latest
    container_name: logdeck
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.logdeck.rule=Host(\`logdeck.example.com\`)"
      - "traefik.http.routers.logdeck.entrypoints=websecure"
      - "traefik.http.routers.logdeck.tls.certresolver=letsencrypt"
      - "traefik.http.services.logdeck.loadbalancer.server.port=8123"
    restart: unless-stopped`}
          language="yaml"
          />
        </div>
      </div>
    </div>
  )
}
