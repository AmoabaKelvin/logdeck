import type { Metadata } from "next"
import { Separator } from "@/components/ui/separator"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

export const metadata: Metadata = {
  title: "Features",
  description: "Explore all the features LogDeck offers for Docker container log viewing and management.",
}

const features = [
  {
    title: "Real-time Log Streaming",
    description:
      "Watch your container logs update in real-time with WebSocket-based streaming. Auto-scroll keeps you at the latest logs, or pause to review specific entries.",
    items: [
      "Live log streaming with automatic updates",
      "Auto-scroll toggle for following new logs",
      "Configurable line limits (50, 100, 500, 1000+ lines)",
      "Timestamps with toggle visibility",
      "Support for both stdout and stderr streams",
    ],
  },
  {
    title: "Advanced Filtering",
    description:
      "Find exactly what you're looking for with powerful filtering and search capabilities.",
    items: [
      "Full-text search across all logs",
      "Filter by log level (ERROR, WARN, INFO, DEBUG)",
      "Regex pattern matching support",
      "Date range filtering",
      "Color-coded log levels for easy identification",
    ],
  },
  {
    title: "Log Export",
    description:
      "Download container logs for offline analysis or archival purposes.",
    items: [
      "Download parsed logs as text files",
      "Export raw logs without processing",
      "Preserves timestamps and log formatting",
      "Useful for sharing with team members or support",
    ],
  },
  {
    title: "Container Discovery",
    description:
      "Automatically discover and monitor all containers running on your Docker host.",
    items: [
      "Real-time container status updates",
      "View container details (name, image, status, uptime)",
      "Host information display (Docker version, container count)",
      "System resource usage (CPU, memory)",
      "Group containers by project, network, or label",
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
    description:
      "Manage your containers with simple, intuitive controls.",
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
      "Update variables without container recreation",
      "Useful for configuration changes",
      "Works with Docker Compose setups",
    ],
  },
  {
    title: "Optional Authentication",
    description:
      "Secure your LogDeck instance or run it completely open based on your needs.",
    items: [
      "JWT-based authentication",
      "SHA256 password hashing with salt",
      "Configurable admin credentials",
      "Can be completely disabled if not needed",
      "Session management with token expiration",
    ],
  },
  {
    title: "Read-Only Mode",
    description:
      "Enable read-only mode to prevent accidental modifications to your containers.",
    items: [
      "View logs without container management capabilities",
      "Prevents start, stop, restart, and remove operations",
      "Perfect for production environments",
      "Can be toggled via environment variable",
    ],
  },
]

export default function FeaturesPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="scroll-m-20 text-4xl font-bold tracking-tight">
          Features
        </h1>
        <p className="text-lg text-muted-foreground mt-2">
          Discover everything LogDeck has to offer for Docker container management.
        </p>
      </div>

      <Separator />

      <div className="prose prose-neutral dark:prose-invert max-w-none">
        <p>
          LogDeck is packed with features designed to make Docker container log viewing and management
          as intuitive and efficient as possible. Here&apos;s a comprehensive look at what LogDeck can do.
        </p>
      </div>

      <div className="grid gap-6 mt-8">
        {features.map((feature) => {
          return (
            <Card key={feature.title}>
              <CardHeader>
                <CardTitle className="text-2xl font-bold">{feature.title}</CardTitle>
                <CardDescription className="mt-2 text-base">
                  {feature.description}
                </CardDescription>
              </CardHeader>
              <CardContent>
                <ul className="space-y-2 text-sm">
                  {feature.items.map((item, i) => (
                    <li key={i} className="flex items-start gap-2">
                      <div className="h-1.5 w-1.5 rounded-full bg-primary mt-2 shrink-0" />
                      <span>{item}</span>
                    </li>
                  ))}
                </ul>
              </CardContent>
            </Card>
          )
        })}
      </div>

      <Separator className="my-8" />

      <div className="prose prose-neutral dark:prose-invert max-w-none">
        <h2>User Interface Features</h2>
        <p>
          Beyond the core functionality, LogDeck offers a polished user experience:
        </p>
        <ul>
          <li><strong>Dark and Light Mode</strong> - Automatic theme switching based on system preferences with manual toggle</li>
          <li><strong>Responsive Design</strong> - Works seamlessly on desktop, tablet, and mobile devices</li>
          <li><strong>Modern UI</strong> - Built with React, Tailwind CSS, and Radix UI for a clean, professional look</li>
          <li><strong>Toast Notifications</strong> - Get immediate feedback on actions with non-intrusive notifications</li>
          <li><strong>Keyboard Shortcuts</strong> - Navigate and control LogDeck efficiently with keyboard shortcuts</li>
          <li><strong>Virtualized Lists</strong> - Handle thousands of log lines without performance degradation</li>
        </ul>

        <h2>Technical Features</h2>
        <ul>
          <li><strong>Single Binary Deployment</strong> - Frontend embedded in Go binary for easy deployment</li>
          <li><strong>No Database Required</strong> - All state managed through Docker API</li>
          <li><strong>Lightweight</strong> - Small resource footprint, suitable for resource-constrained environments</li>
          <li><strong>Fast</strong> - Built with Go for high performance and low latency</li>
          <li><strong>Open Source</strong> - GPL-3.0 licensed, community-driven development</li>
        </ul>

        <h2>Coming Soon</h2>
        <p>LogDeck is actively developed. Here are some features planned for future releases:</p>
        <ul>
          <li>Multi-container log viewing (view logs from multiple containers simultaneously)</li>
          <li>Log persistence and history</li>
          <li>Alert system for specific log patterns</li>
          <li>Container stats and metrics visualization</li>
          <li>Docker Compose integration</li>
          <li>Custom log parsing rules</li>
        </ul>

        <p className="mt-8">
          Have a feature request? {" "}
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
  )
}
