import { ArrowRight } from "lucide-react";
import Link from "next/link";

import { CodeBlock } from "./code-block";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

const dockerComposeExample = `services:
  logdeck:
    image: amoabakelvin/logdeck:latest
    container_name: logdeck
    ports:
      - "8123:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /proc:/host/proc:ro
    environment:
      # Optional: manage multiple hosts
      # DOCKER_HOSTS: local=unix:///var/run/docker.sock,prod=ssh://deploy@prod.example.com
      # Optional: enable authentication
      # JWT_SECRET: your-super-secret-key-min-32-chars
      # ADMIN_USERNAME: admin
      # ADMIN_PASSWORD_SALT: your-random-salt-change-this
      # ADMIN_PASSWORD: your-sha256-hash
    restart: unless-stopped`;

const dockerRunCommand = `docker run -d \\
  --name logdeck \\
  -p 8123:8080 \\
  -v /var/run/docker.sock:/var/run/docker.sock \\
  -v /proc:/host/proc:ro \\
  amoabakelvin/logdeck:latest`;

const cliInstall = `curl -fsSL https://raw.githubusercontent.com/AmoabaKelvin/logdeck/main/install.sh | sh`;

const cliLogin = `logdeck login --url https://logdeck.example.com --token ldk_... --name prod
logdeck status`;

const tabContentClass =
  "mt-4 space-y-4 animate-in fade-in slide-in-from-bottom-2 duration-300";

export function Installation() {
  return (
    <section id="installation" className="border-t bg-muted/30 py-20 sm:py-24">
      <div className="container">
        <div className="grid gap-12 lg:grid-cols-[1fr_1.5fr] lg:gap-20">
          <div>
            <h2 className="text-2xl font-semibold tracking-tight sm:text-3xl">
              Install
            </h2>
            <p className="mt-3 text-muted-foreground">
              One container for the server, one command for the CLI. No
              database, nothing to migrate.
            </p>

            <Button asChild variant="outline" className="mt-8">
              <Link href="/docs/getting-started" className="gap-2">
                Read the docs
                <ArrowRight className="h-4 w-4" />
              </Link>
            </Button>
          </div>

          <Tabs defaultValue="compose" className="min-w-0">
            <TabsList>
              <TabsTrigger value="compose">Docker Compose</TabsTrigger>
              <TabsTrigger value="docker">Docker run</TabsTrigger>
              <TabsTrigger value="cli">CLI</TabsTrigger>
            </TabsList>

            <div className="lg:min-h-[33rem]">
              <TabsContent value="compose" className={tabContentClass}>
                <CodeBlock code={dockerComposeExample} language="yaml" />
                <CodeBlock code="docker compose up -d" language="bash" />
                <p className="text-sm text-muted-foreground">
                  Then open{" "}
                  <a
                    href="http://localhost:8123"
                    className="font-medium text-foreground hover:underline"
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    http://localhost:8123
                  </a>
                  . Your containers are already there.
                </p>
              </TabsContent>

              <TabsContent value="docker" className={tabContentClass}>
                <CodeBlock code={dockerRunCommand} language="bash" />
                <p className="text-sm text-muted-foreground">
                  Then open{" "}
                  <a
                    href="http://localhost:8123"
                    className="font-medium text-foreground hover:underline"
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    http://localhost:8123
                  </a>
                  . To add authentication or more hosts, see the{" "}
                  <Link
                    href="/docs/configuration"
                    className="font-medium text-foreground hover:underline"
                  >
                    configuration guide
                  </Link>
                  .
                </p>
              </TabsContent>

              <TabsContent value="cli" className={tabContentClass}>
                <CodeBlock code={cliInstall} language="bash" />
                <p className="text-sm text-muted-foreground">
                  Installs the{" "}
                  <code className="rounded bg-muted px-1 py-0.5 font-mono text-xs">
                    logdeck
                  </code>{" "}
                  binary for macOS or Linux. Then connect it to a running
                  server:
                </p>
                <CodeBlock code={cliLogin} language="bash" />
                <p className="text-sm text-muted-foreground">
                  Every command takes{" "}
                  <code className="rounded bg-muted px-1 py-0.5 font-mono text-xs">
                    -o json
                  </code>{" "}
                  for scripts and agents. Full reference in the{" "}
                  <Link
                    href="/docs/cli"
                    className="font-medium text-foreground hover:underline"
                  >
                    CLI guide
                  </Link>
                  .
                </p>
              </TabsContent>
            </div>
          </Tabs>
        </div>
      </div>
    </section>
  );
}
