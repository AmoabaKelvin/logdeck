import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";

import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Features",
  description:
    "Every LogDeck feature: real-time Docker and Podman log streaming, persistent log history, alerting, container stats, multi-host management, Compose stack tools, web terminal, and scoped API tokens.",
  alternates: { canonical: "/docs/features" },
};

type Feature = {
  title: string;
  description: string;
  items: string[];
  href?: string;
};

const features: Feature[] = [
  {
    title: "Real-time Log Streaming",
    description:
      "Watch your container logs update in real-time with WebSocket-based streaming. Auto-scroll keeps you at the latest logs, or pause to review specific entries.",
    items: [
      "Live log streaming with automatic updates",
      "Auto-scroll toggle for following new logs",
      "Configurable tail size, and pause with a buffered-line count",
      "Timestamps with toggle visibility",
      "Support for both stdout and stderr streams",
    ],
  },
  {
    title: "Log Persistence & History",
    description:
      "Logs are stored locally, so history outlives the container that produced it.",
    items: [
      "Every container on every host is tailed into a local SQLite store, enabled by default",
      "A Live | History toggle in the log viewer searches everything stored, server-side",
      "History survives restarts, and rebuilds that give a container a new ID (docker compose up --build)",
      "Containers that no longer exist appear under a Removed filter with their stored logs still readable",
      "Retention caps (50 MB per container, 1024 MB total by default) evict oldest-first",
      "Aggregated stack logs remain live-only",
    ],
    href: "/docs/log-history",
  },
  {
    title: "Alerting",
    description:
      "Get told when a container dies, gets OOM-killed, or starts logging errors.",
    items: [
      "Event rules on container death (non-zero exit) and OOM kills",
      "Log rules on a minimum level, a regex pattern, or both",
      "Rate thresholds ('5 matches in 60 seconds') and per-rule cooldowns that report suppressed counts",
      "Target rules by host, container name, or Compose project",
      "One JSON webhook that Slack and Discord incoming webhooks accept unchanged",
      "Alert history with the delivery result of every notification",
    ],
    href: "/docs/alerting",
  },
  {
    title: "Advanced Filtering",
    description:
      "Find exactly what you're looking for with powerful filtering and search capabilities.",
    items: [
      "Full-text search with match navigation, plus highlight or exclude modes",
      "Filter by log level (TRACE, DEBUG, INFO, WARN, ERROR, FATAL, PANIC, and unclassified lines)",
      "Regex pattern matching support",
      "Time range presets and a custom calendar range",
      "Color-coded log levels, collapsible JSON lines, and line pinning",
    ],
  },
  {
    title: "Log Export",
    description:
      "Download container logs for offline analysis or archival purposes.",
    items: [
      "Download the filtered view as JSON or TXT",
      "Works in both Live and History mode",
      "Preserves timestamps and log levels",
      "Copy individual lines or a multi-line selection to the clipboard",
    ],
  },
  {
    title: "Container Discovery",
    description:
      "Automatically discover and monitor all containers running on your Docker host.",
    items: [
      "Real-time container status updates driven by the engine's event stream",
      "View container details (name, image, status, uptime)",
      "Health status badges (healthy, unhealthy, starting) for containers with a healthcheck — Docker only, see the note below",
      "Host information display (engine version, container count)",
      "System resource usage (CPU, memory)",
      "Group containers by Compose project",
    ],
  },
  {
    title: "Multi-Host Management",
    description:
      "Operate across multiple Docker hosts from a single LogDeck dashboard while keeping actions scoped to the right daemon.",
    items: [
      "Connect to local sockets, remote TCP endpoints, or SSH hosts via DOCKER_HOSTS",
      "Unified container list with host badges so you always know where a container lives",
      "Host-aware lifecycle actions, environment variable edits, and log streaming",
      "Automatic fallback to the local Docker socket when no hosts are provided",
    ],
  },
  {
    title: "Container Management",
    description: "Manage your containers with simple, intuitive controls.",
    items: [
      "Start, stop, and restart containers",
      "Remove containers (with confirmation)",
      "View detailed container information",
      "Inspect environment variables",
      "View mounted volumes and exposed ports",
      "Access container labels and metadata",
    ],
  },
  {
    title: "Environment Variable Management",
    description:
      "View and update container environment variables through the UI.",
    items: [
      "Display all environment variables",
      "Add, edit, and delete variables with live container recreation",
      "Coolify integration: sync changes to Coolify so they persist across redeployments",
      "Coolify-managed containers are automatically detected and labeled in the UI",
      "Works with Docker Compose setups",
    ],
  },
  {
    title: "Compose Stack Tools",
    description:
      "Operate on whole Compose stacks instead of one container at a time.",
    items: [
      "Start, stop, or restart every container in a stack from its group header",
      "Aggregated stack logs: all containers merged by timestamp in one stream",
      "Color-coded container badges identify each log line's source",
      "Works with Docker Compose and podman-compose projects",
    ],
  },
  {
    title: "Stats & Trends",
    description:
      "Live resource usage with short-term history, across all your hosts.",
    items: [
      "Live CPU and memory readings per container",
      "Sparkline trend lines covering the last five minutes",
      "Per-host engine stats (CPUs, memory, container counts, version) in multi-host setups",
      "System stats for the machine running LogDeck",
    ],
  },
  {
    title: "Resource Limits & Restart Policies",
    description:
      "Tune container resources without recreating or restarting anything.",
    items: [
      "Edit memory limits, CPU limits, and restart policies from the container page",
      "Applied live via the engine's update API — no downtime",
      "Human-friendly inputs (512m, 1g) with validation",
      "Respects read-only mode",
    ],
  },
  {
    title: "Images, Volumes & Networks",
    description:
      "See what else lives on your hosts beyond containers.",
    items: [
      "Read-only listings of images, volumes, and networks",
      "Aggregated across all configured hosts",
      "Text filtering and per-host error reporting",
    ],
  },
  {
    title: "Command-Line Interface",
    description:
      "A scriptable logdeck CLI that talks to the server's HTTP API — built for automation and AI agents.",
    items: [
      "List containers and stacks, inspect, read/follow/search logs, and check stats from the terminal",
      "logdeck grep searches the recent logs of every running container across all hosts",
      "Lifecycle actions, resource limits, and compose stack controls",
      "Manage alert rules, the webhook, and alert history with logdeck alerts",
      "Persistent named contexts with logdeck login, kubectl-style",
      "Table output for humans, JSON/NDJSON output (-o json) for machines",
    ],
    href: "/docs/cli",
  },
  {
    title: "Interactive Terminal",
    description:
      "Open a real shell in any running container without leaving the browser.",
    items: [
      "WebSocket-based container terminal access",
      "Full terminal emulation with XTerm.js",
      "10,000 line scrollback history",
      "Copy-to-clipboard support",
    ],
  },
  {
    title: "Scoped API Access Tokens",
    description:
      "Give the CLI and external tools their own credentials instead of sharing your login.",
    items: [
      "Create and revoke tokens from Settings → API Access",
      "Tokens are prefixed ldk_ and shown only once at creation",
      "Two scopes: admin (full access) and read (read-only)",
      "A read token cannot mutate anything, open the web terminal, read container environment variables, or read settings",
      "Sent as an Authorization: Bearer header on the HTTP API",
      "Work alongside the JWT sessions used by the web UI",
    ],
    href: "/docs/configuration",
  },
  {
    title: "Optional Authentication",
    description:
      "Secure your LogDeck instance or run it completely open based on your needs.",
    items: [
      "JWT-based login, with a 7-day session token",
      "Enable it from the Settings page, or pin it with environment variables",
      "Environment-configured passwords are bcrypt hashes",
      "Rate-limited login endpoint",
      "Can be completely disabled if not needed",
    ],
  },
  {
    title: "Read-Only Mode",
    description:
      "Enable read-only mode to prevent accidental modifications to your containers.",
    items: [
      "View logs without container management capabilities",
      "Prevents start, stop, restart, remove, and environment or resource edits",
      "Perfect for production environments",
      "Toggle from the Settings page, or pin it with the READONLY_MODE environment variable",
    ],
  },
];

export default function FeaturesPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="scroll-m-20 text-4xl font-bold tracking-tight">
          Features
        </h1>
        <p className="text-lg text-muted-foreground mt-2">
          Discover everything LogDeck has to offer for Docker container
          management.
        </p>
      </div>

      <Separator />

      <div className="prose prose-neutral dark:prose-invert max-w-none">
        <p>
          LogDeck is packed with features designed to make Docker container log
          viewing and management as intuitive and efficient as possible.
          Here&apos;s a comprehensive look at what LogDeck can do.
        </p>
      </div>

      <div className="grid gap-6 mt-8">
        {features.map((feature) => {
          return (
            <Card key={feature.title}>
              <CardHeader>
                <CardTitle className="text-2xl font-bold">
                  {feature.title}
                </CardTitle>
                <CardDescription className="mt-2 text-base">
                  {feature.description}
                </CardDescription>
              </CardHeader>
              <CardContent>
                <ul className="space-y-2 text-sm">
                  {feature.items.map((item) => (
                    <li key={item} className="flex items-start gap-2">
                      <div className="h-1.5 w-1.5 rounded-full bg-primary mt-2 shrink-0" />
                      <span>{item}</span>
                    </li>
                  ))}
                </ul>
                {feature.href && (
                  <a
                    href={feature.href}
                    className="mt-4 inline-block text-sm font-medium underline underline-offset-4"
                  >
                    Read the guide
                  </a>
                )}
              </CardContent>
            </Card>
          );
        })}
      </div>

      <Separator className="my-8" />

      <div className="prose prose-neutral dark:prose-invert max-w-none">
        <h2>User Interface Features</h2>
        <p>
          Beyond the core functionality, LogDeck offers a polished user
          experience:
        </p>
        <ul>
          <li>
            <strong>Dark and Light Mode</strong> - Automatic theme switching
            based on system preferences with manual toggle
          </li>
          <li>
            <strong>Responsive Design</strong> - Works seamlessly on desktop,
            tablet, and mobile devices
          </li>
          <li>
            <strong>Modern UI</strong> - Built with React, Tailwind CSS, and
            Radix UI for a clean, professional look
          </li>
          <li>
            <strong>Toast Notifications</strong> - Get immediate feedback on
            actions with non-intrusive notifications
          </li>
          <li>
            <strong>Keyboard Shortcuts</strong> - Navigate and control LogDeck
            efficiently with keyboard shortcuts, with a built-in cheat sheet
            overlay (press <code>?</code>)
          </li>
          <li>
            <strong>Virtualized Lists</strong> - Handle thousands of log lines
            without performance degradation
          </li>
        </ul>

        <h2>Engine Support and Caveats</h2>
        <ul>
          <li>
            <strong>Docker and Podman</strong> - LogDeck talks to Podman through
            its Docker-compatible API socket, rootless or rootful, and can mix
            Docker and Podman hosts in one multi-host setup. Local sockets are
            auto-detected: Docker first, then rootless Podman, then rootful
            Podman.
          </li>
          <li>
            <strong>Health badges are Docker-only</strong> - LogDeck reads a
            container&apos;s health from the engine&apos;s container list.
            Docker embeds it there; Podman&apos;s Docker-compatible list API
            does not, so containers on a Podman host show no health badge even
            when they define a healthcheck. Everything else works the same on
            both engines.
          </li>
        </ul>

        <h2>Technical Features</h2>
        <ul>
          <li>
            <strong>Single Binary Deployment</strong> - Frontend embedded in Go
            binary for easy deployment
          </li>
          <li>
            <strong>One Directory of State</strong> - No external database. The
            config file, the SQLite log store, and the alert history all live in
            one directory (<code>/data</code> by default) — mount it as a volume
            and there is nothing else to manage
          </li>
          <li>
            <strong>Lightweight</strong> - Small resource footprint, suitable
            for resource-constrained environments
          </li>
          <li>
            <strong>Fast</strong> - Built with Go for high performance and low
            latency
          </li>
          <li>
            <strong>Open Source</strong> - GPL-3.0 licensed, community-driven
            development
          </li>
        </ul>

        <p className="mt-8">
          Have a feature request?{" "}
          <a
            href="https://github.com/AmoabaKelvin/logdeck/issues/new"
            target="_blank"
            rel="noopener noreferrer"
          >
            Open an issue on GitHub
          </a>{" "}
          and let us know what you&apos;d like to see!
        </p>
      </div>
    </div>
  );
}
