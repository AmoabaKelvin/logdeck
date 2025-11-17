import type { Metadata } from "next"
import { CodeBlock } from "@/components/landing/code-block"
import { Separator } from "@/components/ui/separator"

export const metadata: Metadata = {
  title: "Getting Started",
  description: "Learn how to get started with LogDeck, the intuitive Docker container log viewer and management tool.",
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
          <li><strong>Advanced filtering</strong> - Search, filter by log level, and use regex to find exactly what you need</li>
          <li><strong>Container management</strong> - Start, stop, restart, and remove containers with a single click</li>
          <li><strong>Beautiful UI</strong> - A modern, responsive interface that works on desktop and mobile</li>
          <li><strong>Zero configuration</strong> - Works out of the box with sensible defaults</li>
          <li><strong>Optional authentication</strong> - Secure your instance or run it completely open</li>
        </ul>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">Quick Start</h2>
        <p className="mb-6 text-base">
          The fastest way to get LogDeck running is with Docker Compose:
        </p>

        <div className="mb-6">
          <CodeBlock
          code={`version: '3.8'

services:
  logdeck:
    image: logdeck/logdeck:latest
    container_name: logdeck
    ports:
      - "8123:8123"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    restart: unless-stopped`}
          language="yaml"
          />
        </div>

        <p className="mb-4 text-sm">Save this as <code>docker-compose.yml</code> and run:</p>

        <div className="mb-6">
          <CodeBlock code="docker-compose up -d" language="bash" />
        </div>

        <p className="mb-8 text-sm">
          Then open your browser to{" "}
          <a href="http://localhost:8123" target="_blank" rel="noopener noreferrer">
            http://localhost:8123
          </a>
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
            <a href="/docs/configuration">Configuration</a> - Configure authentication, environment variables, and more
          </li>
        </ul>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">System Requirements</h2>
        <ul className="mb-8 space-y-2">
          <li>Docker Engine 20.10 or later</li>
          <li>Access to the Docker socket (<code>/var/run/docker.sock</code>)</li>
          <li>Modern web browser (Chrome, Firefox, Safari, or Edge)</li>
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
