import { CodeBlock } from "@/components/landing/code-block";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { AlertTriangle } from "lucide-react";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Installation",
  description:
    "Install the self-hosted LogDeck server with Docker Compose or docker run, mount the Docker socket, configure environment variables, and update or troubleshoot.",
  alternates: { canonical: "/docs/installation" },
};

export default function InstallationPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="scroll-m-20 text-4xl font-bold tracking-tight">
          Installation
        </h1>
        <p className="text-lg text-muted-foreground mt-2">
          Multiple ways to install and deploy LogDeck for your environment.
        </p>
      </div>

      <Separator />

      <div className="prose prose-neutral dark:prose-invert max-w-none">
        <h2 className="mb-4 text-3xl font-bold tracking-tight">
          Docker Compose (Recommended)
        </h2>
        <p className="mb-8 text-base">
          Docker Compose is the recommended way to deploy LogDeck. It provides
          easy configuration and management.
        </p>

        <h3 className="mb-4 mt-8 text-xl font-semibold">
          Step 1: Create docker-compose.yml
        </h3>
        <p className="mb-4 text-sm">
          Create a <code>docker-compose.yml</code> file with the following
          content:
        </p>

        <div className="mb-8">
          <CodeBlock
            code={`services:
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
  logdeck-data:`}
            language="yaml"
          />
        </div>

        <div className="not-prose mb-8">
          <Card className="border-amber-200 bg-amber-50 dark:border-amber-900 dark:bg-amber-950/20">
            <CardHeader>
              <div className="flex items-start gap-2">
                <AlertTriangle className="h-5 w-5 text-amber-600 dark:text-amber-500 mt-0.5" />
                <div>
                  <CardTitle className="text-amber-900 dark:text-amber-200">
                    Do not skip the <code>/data</code> volume
                  </CardTitle>
                </div>
              </div>
            </CardHeader>
            <CardContent className="text-sm text-amber-900 dark:text-amber-200">
              <p>
                LogDeck keeps its config file (hosts, API tokens, alert rules), its{" "}
                <a href="/docs/log-history" className="underline underline-offset-2">
                  stored log history
                </a>
                , and its alert history in <code>/data</code>. Without a volume, all of it is written
                inside the container and lost the moment you recreate it — including every log line
                you were counting on being able to read back.
              </p>
            </CardContent>
          </Card>
        </div>

        <h3 className="mb-4 mt-8 text-xl font-semibold">
          Step 2: Start LogDeck
        </h3>
        <div className="mb-8">
          <CodeBlock code="docker-compose up -d" language="bash" />
        </div>

        <h3 className="mb-4 mt-8 text-xl font-semibold">
          Step 3: Verify It&apos;s Running
        </h3>
        <div className="mb-8">
          <CodeBlock code="docker-compose ps" language="bash" />
        </div>

        <h3 className="mb-4 mt-8 text-xl font-semibold">
          Step 4: Access the Interface
        </h3>
        <p className="mb-8 text-sm">
          Open your browser and navigate to{" "}
          <a
            href="http://localhost:8123"
            target="_blank"
            rel="noopener noreferrer"
          >
            http://localhost:8123
          </a>
        </p>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">Docker Run</h2>
        <p className="mb-8 text-base">
          For a quick deployment without Docker Compose, use the{" "}
          <code>docker run</code> command:
        </p>

        <h3 className="mb-4 mt-8 text-xl font-semibold">Basic Deployment</h3>
        <div className="mb-8">
          <CodeBlock
            code={`docker run -d \\
  --name logdeck \\
  -p 8123:8080 \\
  -v /var/run/docker.sock:/var/run/docker.sock \\
  -v /proc:/host/proc:ro \\
  -v logdeck-data:/data \\
  --restart unless-stopped \\
  amoabakelvin/logdeck:latest`}
            language="bash"
          />
        </div>

        <h3 className="mb-4 mt-8 text-xl font-semibold">With Authentication</h3>
        <p className="mb-4 text-sm">
          <code>ADMIN_PASSWORD</code> takes a bcrypt hash — generate one with{" "}
          <code>htpasswd -bnBC 10 &apos;&apos; yourPassword | tr -d &apos;:&apos;</code>. See the{" "}
          <a href="/docs/configuration">configuration guide</a>.
        </p>
        <div className="mb-8">
          <CodeBlock
            code={`docker run -d \\
  --name logdeck \\
  -p 8123:8080 \\
  -v /var/run/docker.sock:/var/run/docker.sock \\
  -v /proc:/host/proc:ro \\
  -v logdeck-data:/data \\
  -e JWT_SECRET=your-super-secret-key-min-32-chars \\
  -e ADMIN_USERNAME=admin \\
  -e ADMIN_PASSWORD='your-bcrypt-hash' \\
  --restart unless-stopped \\
  amoabakelvin/logdeck:latest`}
            language="bash"
          />
        </div>

        <h3 className="mb-4 mt-8 text-xl font-semibold">
          Configure Multiple Docker Hosts
        </h3>
        <p className="mb-4 text-sm">
          Manage more than one Docker daemon by setting <code>DOCKER_HOSTS</code> with comma-separated <code>name=host</code> entries.
        </p>
        <div className="mb-4">
          <CodeBlock
            code={`# Local Docker + remote SSH host
export DOCKER_HOSTS="local=unix:///var/run/docker.sock,prod=ssh://deploy@prod.example.com"
# Then start LogDeck with docker run or docker compose`}
            language="bash"
          />
        </div>
        <p className="mb-8 text-sm">
          For <code>ssh://</code> targets, mount your SSH keys (e.g., <code>~/.ssh</code>) or forward your SSH agent socket into the container.
        </p>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">
          Environment Variables
        </h2>
        <p className="mb-6 text-base">
          Every variable is optional — LogDeck runs with none of them set. The{" "}
          <a href="/docs/configuration">configuration guide</a> covers each one in full, including
          the config file that the Settings page writes.
        </p>

        <div className="not-prose">
          <Card className="mb-6">
            <CardHeader>
              <CardTitle className="text-lg">Server Configuration</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <div>
                <code className="text-sm bg-muted px-2 py-1 rounded">
                  DOCKER_HOSTS
                </code>
                <p className="text-sm text-muted-foreground mt-1">
                  Comma-separated Docker hosts using <code>name=host</code> format (supports <code>unix://</code>, <code>tcp://</code>, and <code>ssh://</code>).
                  When unset, LogDeck auto-detects a local Docker or Podman socket.
                </p>
              </div>
              <div>
                <code className="text-sm bg-muted px-2 py-1 rounded">
                  CONFIG_PATH
                </code>
                <p className="text-sm text-muted-foreground mt-1">
                  Path to the JSON config file. Its directory also holds the log store and alert
                  history. Defaults to <code>/data/config.json</code>.
                </p>
              </div>
              <div>
                <code className="text-sm bg-muted px-2 py-1 rounded">
                  READONLY_MODE
                </code>
                <p className="text-sm text-muted-foreground mt-1">
                  <code>true</code> blocks container actions, stack actions, environment and resource
                  edits, and the web terminal.
                </p>
              </div>
            </CardContent>
          </Card>

          <Card className="mb-6">
            <CardHeader>
              <CardTitle className="text-lg">
                Log Persistence (Optional)
              </CardTitle>
              <CardDescription>
                On by default; these only change retention
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div>
                <code className="text-sm bg-muted px-2 py-1 rounded">
                  LOG_STORE_ENABLED
                </code>
                <p className="text-sm text-muted-foreground mt-1">
                  <code>false</code> turns off log persistence and History mode. Default{" "}
                  <code>true</code>.
                </p>
              </div>
              <div>
                <code className="text-sm bg-muted px-2 py-1 rounded">
                  LOG_STORE_PER_CONTAINER_MB
                </code>
                <p className="text-sm text-muted-foreground mt-1">
                  Retention cap per container, in MB. Default <code>50</code>.
                </p>
              </div>
              <div>
                <code className="text-sm bg-muted px-2 py-1 rounded">
                  LOG_STORE_TOTAL_MB
                </code>
                <p className="text-sm text-muted-foreground mt-1">
                  Retention cap for the whole store, in MB. Default <code>1024</code>.
                </p>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-lg">
                Authentication (Optional)
              </CardTitle>
              <CardDescription>
                Leave these unset to run without authentication, or to enable it from the Settings
                page instead
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div>
                <code className="text-sm bg-muted px-2 py-1 rounded">
                  JWT_SECRET
                </code>
                <p className="text-sm text-muted-foreground mt-1">
                  Secret key for signing session tokens.
                </p>
              </div>
              <div>
                <code className="text-sm bg-muted px-2 py-1 rounded">
                  ADMIN_USERNAME
                </code>
                <p className="text-sm text-muted-foreground mt-1">
                  Admin username for authentication.
                </p>
              </div>
              <div>
                <code className="text-sm bg-muted px-2 py-1 rounded">
                  ADMIN_PASSWORD
                </code>
                <p className="text-sm text-muted-foreground mt-1">
                  A bcrypt hash of the admin password — never plain text. LogDeck refuses to start if
                  it is malformed. See the configuration guide for how to generate one.
                </p>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-lg">
                Coolify Integration (Optional)
              </CardTitle>
              <CardDescription>
                Leave unset if not using Coolify.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div>
                <code className="text-sm bg-muted px-2 py-1 rounded">
                  COOLIFY_CONFIGS
                </code>
                <p className="text-sm text-muted-foreground mt-1">
                  Per-host Coolify configuration. Format: <code>hostName|apiURL|apiToken,...</code>
                </p>
                <p className="text-sm text-muted-foreground mt-1">
                  Host names must match those in <code>DOCKER_HOSTS</code>. Generate API tokens from
                  your Coolify dashboard under Settings &rarr; API Tokens.
                </p>
              </div>
            </CardContent>
          </Card>
        </div>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">
          Docker Socket Access
        </h2>
        <div className="not-prose mb-6">
          <Card className="border-amber-200 bg-amber-50 dark:border-amber-900 dark:bg-amber-950/20">
            <CardHeader>
              <div className="flex items-start gap-2">
                <AlertTriangle className="h-5 w-5 text-amber-600 dark:text-amber-500 mt-0.5" />
                <div>
                  <CardTitle className="text-amber-900 dark:text-amber-200">
                    Security Note
                  </CardTitle>
                  <CardDescription className="text-amber-800 dark:text-amber-300">
                    Mounting the Docker socket gives LogDeck access to your
                    Docker daemon
                  </CardDescription>
                </div>
              </div>
            </CardHeader>
            <CardContent className="text-sm text-amber-900 dark:text-amber-200">
              <p>
                The Docker socket (<code>/var/run/docker.sock</code>) provides
                full access to the Docker daemon. Only run LogDeck on trusted
                networks or enable authentication to protect access.
              </p>
              <p className="mt-2">
                LogDeck needs write access to the Docker socket for container management features
                (start, stop, restart). If you only need log viewing, you can mount it
                read-only (<code>:ro</code>) and enable read-only mode in LogDeck.
              </p>
            </CardContent>
          </Card>
        </div>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">
          Updating LogDeck
        </h2>
        <p className="mb-6 text-base">
          To update to the latest version of LogDeck:
        </p>

        <h3 className="mb-4 mt-8 text-xl font-semibold">With Docker Compose</h3>
        <div className="mb-8">
          <CodeBlock
            code={`docker-compose pull
docker-compose up -d`}
            language="bash"
          />
        </div>

        <h3 className="mb-4 mt-8 text-xl font-semibold">With Docker Run</h3>
        <div className="mb-8">
          <CodeBlock
            code={`docker stop logdeck
docker rm logdeck
docker pull amoabakelvin/logdeck:latest
# Then run your docker run command again`}
            language="bash"
          />
        </div>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">
          Installing the CLI (Optional)
        </h2>
        <p className="mb-6 text-base">
          The <code>logdeck</code> command-line client talks to your running
          LogDeck server, so you can read logs, check stats, and manage
          containers from the terminal or from scripts:
        </p>
        <div className="mb-6">
          <CodeBlock
            code="curl -fsSL https://raw.githubusercontent.com/AmoabaKelvin/logdeck/main/install.sh | sh"
            language="bash"
          />
        </div>
        <p className="mb-8 text-base">
          It installs a single binary for macOS or Linux (amd64/arm64). See the{" "}
          <a href="/docs/cli">CLI guide</a> for connecting to your server and
          the full command reference.
        </p>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">
          Troubleshooting
        </h2>

        <h3 className="mb-4 mt-8 text-xl font-semibold">
          Container Won&apos;t Start
        </h3>
        <p className="mb-4 text-sm">Check the logs:</p>
        <div className="mb-8">
          <CodeBlock code="docker logs logdeck" language="bash" />
        </div>

        <h3 className="mb-4 mt-8 text-xl font-semibold">
          Can&apos;t See Any Containers
        </h3>
        <p className="mb-4 text-sm">
          Ensure the Docker socket is properly mounted:
        </p>
        <div className="mb-8">
          <CodeBlock
            code="docker inspect logdeck | grep docker.sock"
            language="bash"
          />
        </div>

        <h3 className="mb-4 mt-8 text-xl font-semibold">Port Already in Use</h3>
        <p className="mb-4 text-sm">
          If port 8123 is already in use, change it in your{" "}
          <code>docker-compose.yml</code> or <code>docker run</code> command:
        </p>
        <div className="mb-8">
          <CodeBlock
            code='- "8124:8080"  # Use port 8124 instead'
            language="yaml"
          />
        </div>

        <div className="not-prose mt-8">
          <Card className="border-green-200 bg-green-50 dark:border-green-900 dark:bg-green-950/20">
            <CardHeader>
              <CardTitle className="text-green-900 dark:text-green-200">
                Need More Help?
              </CardTitle>
            </CardHeader>
            <CardContent className="text-sm text-green-900 dark:text-green-200">
              If you&apos;re still having issues, please{" "}
              <a
                href="https://github.com/AmoabaKelvin/logdeck/issues"
                target="_blank"
                rel="noopener noreferrer"
                className="underline hover:no-underline underline-offset-2 font-medium"
              >
                open an issue on GitHub
              </a>{" "}
              with details about your setup and the error you&apos;re seeing.
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
