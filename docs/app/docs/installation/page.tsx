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
    "Complete guide to installing LogDeck using Docker Compose, Docker Run, or from source.",
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
            code={`version: '3.8'

services:
  logdeck:
    image: logdeck/logdeck:latest
    container_name: logdeck
    ports:
      - "8123:8123"
    environment:
      # Optional: Configure backend port
      # BACKEND_PORT: 8080

      # Optional: Manage multiple Docker hosts
      # DOCKER_HOSTS: local=unix:///var/run/docker.sock,prod=ssh://deploy@prod.example.com

      # Optional: Enable authentication
      # JWT_SECRET: your-super-secret-key-min-32-chars
      # ADMIN_USERNAME: admin
      # ADMIN_PASSWORD_SALT: your-random-salt-change-this
      # ADMIN_PASSWORD: your-sha256-hash
    volumes:
      # Mount the Docker socket (read-only for security)
      - /var/run/docker.sock:/var/run/docker.sock:ro
      # Mount SSH keys if you use ssh:// hosts
      # - ~/.ssh:/root/.ssh:ro
    restart: unless-stopped`}
            language="yaml"
          />
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
  -p 8123:8123 \\
  -v /var/run/docker.sock:/var/run/docker.sock:ro \\
  --restart unless-stopped \\
  logdeck/logdeck:latest`}
            language="bash"
          />
        </div>

        <h3 className="mb-4 mt-8 text-xl font-semibold">With Authentication</h3>
        <div className="mb-8">
          <CodeBlock
            code={`docker run -d \\
  --name logdeck \\
  -p 8123:8123 \\
  -v /var/run/docker.sock:/var/run/docker.sock:ro \\
  -e JWT_SECRET=your-super-secret-key-min-32-chars \\
  -e ADMIN_USERNAME=admin \\
  -e ADMIN_PASSWORD_SALT=your-random-salt-change-this \\
  -e ADMIN_PASSWORD=your-sha256-hash \\
  --restart unless-stopped \\
  logdeck/logdeck:latest`}
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
          LogDeck can be configured using the following environment variables:
        </p>

        <div className="not-prose">
          <Card className="mb-6">
            <CardHeader>
              <CardTitle className="text-lg">Server Configuration</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <div>
                <code className="text-sm bg-muted px-2 py-1 rounded">
                  BACKEND_PORT
              </code>
              <p className="text-sm text-muted-foreground mt-1">
                Port for the backend server. Default: <code>8080</code>
              </p>
            </div>
            <div>
              <code className="text-sm bg-muted px-2 py-1 rounded">
                  DOCKER_HOSTS
                </code>
                <p className="text-sm text-muted-foreground mt-1">
                  Comma-separated Docker hosts using <code>name=host</code> format (supports <code>unix://</code>, <code>tcp://</code>, and <code>ssh://</code>).
                  Defaults to <code>local=unix:///var/run/docker.sock</code> when unset.
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
                Leave these unset to disable authentication completely
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div>
                <code className="text-sm bg-muted px-2 py-1 rounded">
                  JWT_SECRET
                </code>
                <p className="text-sm text-muted-foreground mt-1">
                  Secret key for JWT token signing. Must be at least 32
                  characters.
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
                  ADMIN_PASSWORD_SALT
                </code>
                <p className="text-sm text-muted-foreground mt-1">
                  Random salt for password hashing. Use a strong, random string.
                </p>
              </div>
              <div>
                <code className="text-sm bg-muted px-2 py-1 rounded">
                  ADMIN_PASSWORD
                </code>
                <p className="text-sm text-muted-foreground mt-1">
                  SHA256 hash of (password + salt). See configuration guide for
                  generation instructions.
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
                We mount it as read-only (<code>:ro</code>) by default, but
                LogDeck needs write access for container management features
                (start, stop, restart). If you only need log viewing, keep it
                read-only and enable read-only mode in LogDeck.
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
docker pull logdeck/logdeck:latest
# Then run your docker run command again`}
            language="bash"
          />
        </div>

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
            code='- "8124:8123"  # Use port 8124 instead'
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
