export const siteUrl = "https://logdeck.dev";

// The sidebar, sitemap, llms.txt, and page metadata all read this list. The
// body of each page lives in content/<href>.md. Bump `updated` only when a
// page's content changes: search engines stop trusting dates that always move.
export const docsNav = [
  {
    title: "Get started",
    items: [
      {
        title: "Getting started",
        href: "/docs/getting-started",
        updated: "2026-09-13",
        description:
          "What LogDeck is, what it is not, and the quickest way to run a self-hosted instance with Docker Compose.",
      },
      {
        title: "Installation",
        href: "/docs/installation",
        updated: "2026-09-13",
        description:
          "Install the LogDeck server with Docker Compose or docker run, mount the Docker or Podman socket, and update or troubleshoot it.",
      },
      {
        title: "Features",
        href: "/docs/features",
        updated: "2026-09-13",
        description:
          "Every LogDeck feature: live and stored container logs, alerting, stats, multi-host, Compose stack actions, a web terminal, the CLI, and MCP.",
      },
    ],
  },
  {
    title: "Guides",
    items: [
      {
        title: "Log history",
        href: "/docs/log-history",
        updated: "2026-09-13",
        description:
          "How LogDeck stores container logs in SQLite so history survives restarts, rebuilds, and removal, plus retention and the history API.",
      },
      {
        title: "Alerting",
        href: "/docs/alerting",
        updated: "2026-09-13",
        description:
          "Alert on container deaths, OOM kills, failing health checks, and log patterns, with thresholds and cooldowns, to webhooks, ntfy, Gotify, or Telegram.",
      },
      {
        title: "CLI reference",
        href: "/docs/cli",
        updated: "2026-09-13",
        description:
          "The logdeck CLI: install it, log in with contexts and API tokens, then read, follow, and grep logs and manage containers from the terminal.",
      },
      {
        title: "MCP server",
        href: "/docs/mcp",
        updated: "2026-09-13",
        description:
          "Run logdeck mcp so Claude, Cursor, or any MCP client can read your containers, logs, and stats, and act on them with an admin token.",
      },
    ],
  },
  {
    title: "Reference",
    items: [
      {
        title: "Configuration",
        href: "/docs/configuration",
        updated: "2026-09-13",
        description:
          "Every LogDeck setting: the /data directory, DOCKER_HOSTS, authentication, API token scopes, log retention, Coolify sync, read-only mode, and reverse proxies.",
      },
    ],
  },
  {
    title: "Compare",
    items: [
      {
        title: "LogDeck vs Dozzle",
        href: "/compare/dozzle",
        updated: "2026-09-13",
        description:
          "A feature-by-feature comparison of LogDeck and Dozzle, two self-hosted Docker log viewers, including where Dozzle is the better pick.",
      },
    ],
  },
];
