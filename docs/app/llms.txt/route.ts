import { summary } from "@/lib/docs";
import { docsNav, siteUrl } from "@/lib/docs-nav";

export const dynamic = "force-static";

// Format: https://llmstxt.org
export function GET() {
  const sections = docsNav.map(
    (section) =>
      `## ${section.title}\n\n${section.items
        .map(
          (item) =>
            `- [${item.title}](${siteUrl}${item.href}.md): ${item.description}`,
        )
        .join("\n")}`,
  );

  const body = `# LogDeck

> ${summary} It keeps container logs after restarts and removal, alerts when containers break, and lets you fix them from a browser, a CLI, or an AI agent.

- Server: one Docker image, \`amoabakelvin/logdeck\`, listening on port 8080 inside the container. Mount the Docker or Podman socket, and mount \`/data\` as a volume: it holds the config, the SQLite log store, and alert history.
- Log history: every container's logs are stored on the LogDeck host and stay searchable after the container restarts, is rebuilt, or is removed.
- Alerts: rules on container deaths, OOM kills, failing health checks, and log patterns, sent to a webhook (Slack and Discord work as-is), ntfy, Gotify, or Telegram.
- CLI and MCP: the \`logdeck\` CLI and \`logdeck mcp\` use the server's HTTP API with scoped \`ldk_\` API tokens. A read token can only read; an admin token can act.
- Not included: deploying apps, multiple users or SSO, Docker Swarm, and Kubernetes.

Every page below is also served as HTML without the \`.md\` suffix.

${sections.join("\n\n")}

## Optional

- [Full documentation](${siteUrl}/llms-full.txt): Every page above in one markdown file.
- [Live demo](${siteUrl}/demo): The LogDeck UI running in the browser on simulated data.
- [GitHub repository](https://github.com/AmoabaKelvin/logdeck): Source code, issues, and releases.
- [Docker Hub image](https://hub.docker.com/r/amoabakelvin/logdeck): The amoabakelvin/logdeck server image.
`;

  return new Response(body, {
    headers: { "Content-Type": "text/plain; charset=utf-8" },
  });
}
