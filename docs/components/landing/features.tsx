const features = [
  {
    title: "Live log streaming",
    description:
      "Follow logs in real time with pause, auto-scroll, and timestamps. Thousands of lines stay smooth.",
  },
  {
    title: "Log history",
    description:
      "Logs persist to a local store, so history survives a restart, a rebuild, and even removal.",
  },
  {
    title: "Alerting",
    description:
      "Rules on container deaths, OOM kills, and log patterns, delivered to a webhook Slack and Discord accept as-is.",
  },
  {
    title: "Search and filtering",
    description:
      "Full-text search with match navigation, log level filters, and calendar-based time ranges.",
  },
  {
    title: "Multi-host",
    description:
      "Manage local, TCP, and SSH daemons from one dashboard. Every container, one list.",
  },
  {
    title: "Docker and Podman",
    description:
      "Works with both engines, rootless or rootful, side by side in the same setup.",
  },
  {
    title: "Compose stacks",
    description:
      "Start, stop, or restart whole stacks, and read stack logs merged by timestamp.",
  },
  {
    title: "Stats and trends",
    description:
      "Live CPU and memory per container, with sparklines covering the last five minutes.",
  },
  {
    title: "Resource limits",
    description:
      "Change memory limits, CPU limits, and restart policies live — no recreate, no downtime.",
  },
  {
    title: "Command-line client",
    description:
      "A scriptable logdeck CLI with JSON output, built for automation and AI agents.",
  },
  {
    title: "Web terminal",
    description:
      "Open a shell in any running container straight from the browser.",
  },
  {
    title: "Images, volumes, networks",
    description:
      "Read-only views of everything else on your hosts, aggregated and filterable.",
  },
  {
    title: "Environment variables",
    description:
      "View and edit container env vars, with bulk paste from .env files.",
  },
  {
    title: "Auth and API tokens",
    description:
      "Optional login, admin and read-only API tokens for external tools, and a read-only mode for production.",
  },
];

export function Features() {
  return (
    <section id="features" className="container py-20 sm:py-24">
      <div className="max-w-2xl">
        <h2 className="text-2xl font-semibold tracking-tight sm:text-3xl">
          Everything you need to run containers
        </h2>
        <p className="mt-3 text-muted-foreground">
          One binary with the frontend embedded. No external database, no agents
          on your hosts.
        </p>
      </div>

      <dl className="mt-12 grid gap-x-10 gap-y-9 sm:grid-cols-2 lg:grid-cols-3">
        {features.map((feature) => (
          <div key={feature.title}>
            <dt className="font-medium">{feature.title}</dt>
            <dd className="mt-1.5 text-sm leading-relaxed text-muted-foreground">
              {feature.description}
            </dd>
          </div>
        ))}
      </dl>
    </section>
  );
}
