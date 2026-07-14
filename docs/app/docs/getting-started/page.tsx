import type { Metadata } from "next"
import { CodeBlock } from "@/components/landing/code-block"
import { Separator } from "@/components/ui/separator"

export const metadata: Metadata = {
  title: "Getting started",
  description: "What LogDeck is and how to deploy it: a self-hosted, open-source dashboard for Docker and Podman container logs, stats, and management. Running in minutes.",
  alternates: { canonical: "/docs/getting-started" },
}

export default function GettingStartedPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="scroll-m-20 text-4xl font-bold tracking-tight">
          Getting Started
        </h1>
        <p className="text-lg text-muted-foreground mt-2">
          Welcome to LogDeck! Get started in minutes with our Docker-based deployment.
        </p>
      </div>

      <Separator />

      <div className="prose prose-neutral dark:prose-invert max-w-none">
        <h2 className="mb-4 text-3xl font-bold tracking-tight">What is LogDeck?</h2>
        <p className="mb-8 text-base">
          LogDeck is an open-source tool designed to be the most intuitive and visually appealing solution for monitoring Docker container logs and managing containers. Built with modern web technologies and a Go backend, LogDeck provides a beautiful interface for working with Docker.
        </p>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">Why LogDeck?</h2>
        <p className="mb-6 text-base">
          Docker&apos;s command-line tools are powerful but can be cumbersome for everyday use. LogDeck provides:
        </p>
        <ul className="mb-8 space-y-2">
          <li><strong>Real-time log streaming</strong> - Watch your container logs update in real-time with automatic scrolling</li>
          <li><strong>Log history</strong> - Logs are stored locally, so you can still read them after a container restarts, gets rebuilt, or is removed entirely</li>
          <li><strong>Alerting</strong> - Get notified when a container dies, gets OOM-killed, or starts logging errors</li>
          <li><strong>Multi-host management</strong> - Connect to multiple Docker or Podman daemons (local, TCP, or SSH) and manage them from one UI</li>
          <li><strong>Advanced filtering</strong> - Search, filter by log level, and use regex to find exactly what you need</li>
          <li><strong>Container management</strong> - Start, stop, restart, and remove containers, or operate on whole Compose stacks at once</li>
          <li><strong>Stats and trends</strong> - Live CPU and memory per container, with sparklines covering recent history</li>
          <li><strong>Live resource tuning</strong> - Edit memory limits, CPU limits, and restart policies with no container downtime</li>
          <li><strong>A scriptable CLI</strong> - Everything the UI can see, from your terminal, with JSON output for scripts and AI agents</li>
          <li><strong>Zero configuration</strong> - Works out of the box with sensible defaults</li>
          <li><strong>Optional authentication</strong> - Login sessions and scoped API tokens, or run it completely open</li>
        </ul>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">Quick Start</h2>
        <p className="mb-6 text-base">
          The fastest way to get LogDeck running is with Docker Compose:
        </p>

        <div className="mb-6">
          <CodeBlock
          code={`services:
  logdeck:
    image: amoabakelvin/logdeck:latest
    container_name: logdeck
    ports:
      - "8123:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /proc:/host/proc:ro
      # Config file, stored log history, and alert history
      - logdeck-data:/data
    restart: unless-stopped

volumes:
  logdeck-data:`}
          language="yaml"
          />
        </div>

        <p className="mb-4 text-sm">Save this as <code>docker-compose.yml</code> and run:</p>

        <div className="mb-6">
          <CodeBlock code="docker-compose up -d" language="bash" />
        </div>

        <p className="mb-4 text-sm">
          Then open your browser to{" "}
          <a href="http://localhost:8123" target="_blank" rel="noopener noreferrer">
            http://localhost:8123
          </a>
        </p>

        <p className="mb-8 text-sm">
          The <code>logdeck-data</code> volume is where LogDeck keeps its config file, its stored log
          history, and its alert history. Leave it out and all three are lost the next time the
          container is recreated.
        </p>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">What&apos;s Next?</h2>
        <p className="mb-4 text-base">Now that you have LogDeck running, here are some next steps:</p>
        <ul className="mb-8 space-y-2">
          <li>
            <a href="/docs/installation">Installation Guide</a> - Learn about different deployment options
          </li>
          <li>
            <a href="/docs/features">Features</a> - Discover all the features LogDeck offers
          </li>
          <li>
            <a href="/docs/log-history">Log History</a> - How stored logs work, and how to tune retention
          </li>
          <li>
            <a href="/docs/alerting">Alerting</a> - Alert on container deaths, OOM kills, and log patterns
          </li>
          <li>
            <a href="/docs/cli">CLI</a> - Install the logdeck command-line client and work from the terminal
          </li>
          <li>
            <a href="/docs/configuration">Configuration</a> - Configure authentication, environment variables, and more
          </li>
        </ul>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">System Requirements</h2>
        <ul className="mb-8 space-y-2">
          <li>Docker Engine 20.10 or later</li>
          <li>Access to the Docker socket (<code>/var/run/docker.sock</code>)</li>
          <li>Modern web browser (Chrome, Firefox, Safari, or Edge)</li>
          <li>Network access and credentials for any remote Docker hosts if using multi-host mode</li>
        </ul>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">Getting Help</h2>
        <p className="mb-4 text-base">
          If you encounter any issues or have questions:
        </p>
        <ul className="space-y-2">
          <li>
            Check the{" "}
            <a href="https://github.com/AmoabaKelvin/logdeck/issues" target="_blank" rel="noopener noreferrer">
              GitHub Issues
            </a>{" "}
            for known problems and solutions
          </li>
          <li>
            Open a new issue on{" "}
            <a href="https://github.com/AmoabaKelvin/logdeck/issues/new" target="_blank" rel="noopener noreferrer">
              GitHub
            </a>
          </li>
        </ul>
      </div>
    </div>
  )
}
