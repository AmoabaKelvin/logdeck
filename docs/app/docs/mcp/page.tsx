import type { Metadata } from "next";
import { CodeBlock } from "@/components/landing/code-block";
import { Separator } from "@/components/ui/separator";

export const metadata: Metadata = {
  title: "MCP server",
  description:
    "Run logdeck as a Model Context Protocol server over stdio so an AI assistant can read your containers, logs, events, and stats — and, when you opt in, restart containers or run commands. Capability follows the API token.",
  alternates: { canonical: "/docs/mcp" },
};

const readTools = [
  {
    name: "list_containers",
    summary:
      "Containers across every host, including removed ones and their health state.",
  },
  {
    name: "get_logs",
    summary:
      "Recent parsed logs for one container, with tail, level, regex, and time-range filters. Never follows.",
  },
  {
    name: "search_logs",
    summary:
      "Regex search across many running containers, merged by timestamp.",
  },
  {
    name: "inspect_container",
    summary: "The full inspect document for one container.",
  },
  {
    name: "list_events",
    summary: "Docker/Podman events collected over a short bounded window.",
  },
  {
    name: "container_stats / host_stats",
    summary: "Live CPU and memory per container, and per-host system stats.",
  },
  {
    name: "list_images / list_volumes / list_networks",
    summary: "Images, volumes, and networks across hosts.",
  },
  {
    name: "history_search / history_status / history_containers",
    summary:
      "Query the persisted log store: fast, indexed, cursor-paginated, and readable even for containers that no longer exist.",
  },
];

const actionTools = [
  {
    name: "start_container / stop_container / restart_container",
    summary: "Reversible lifecycle actions.",
  },
  {
    name: "remove_container",
    summary:
      "Remove a container. Irreversible, and marked destructive so clients prompt harder.",
  },
  {
    name: "run_command",
    summary:
      "Run one non-interactive command in a container and return separate stdout, stderr, and the exit code.",
  },
  {
    name: "get_env / set_env",
    summary:
      "Read and replace a container's environment variables. Values commonly hold secrets, and a write recreates the container, so it restarts with a new ID.",
  },
  {
    name: "get_settings / set_read_only / set_log_storage",
    summary:
      "Read settings, toggle server-wide read-only mode, and change log persistence and its retention caps.",
  },
  {
    name: "set_docker_hosts / set_coolify_hosts",
    summary:
      "Replace the configured hosts. Each takes the complete list rather than merging, so read get_settings first.",
  },
  {
    name: "set_auth / list_api_tokens / create_api_token / delete_api_token",
    summary:
      "Change authentication and manage API tokens. Disabling auth leaves the server open to anyone who can reach it.",
  },
];

export default function McpPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="scroll-m-20 text-4xl font-bold tracking-tight">
          MCP server
        </h1>
        <p className="text-lg text-muted-foreground mt-2">
          Let an AI assistant read and manage your containers — safely, on your
          terms.
        </p>
      </div>

      <Separator />

      <div className="prose prose-neutral dark:prose-invert max-w-none">
        <p className="mb-4 text-base">
          <code>logdeck mcp</code> runs a{" "}
          <a
            href="https://modelcontextprotocol.io"
            target="_blank"
            rel="noopener noreferrer"
          >
            Model Context Protocol
          </a>{" "}
          server over stdio, so an assistant like Claude Desktop, Cursor, or
          Claude Code can query your containers, logs, events, and stats
          directly — and, when you opt in, restart a container or run a command.
          It is a thin layer over the same HTTP API the web UI and CLI use, so
          it adds no new server and no new way in: your existing API token
          decides what it can do.
        </p>

        <h2 className="mb-4 mt-10 text-3xl font-bold tracking-tight">Setup</h2>
        <p className="mb-4 text-base">
          You need the <code>logdeck</code> CLI installed (see the{" "}
          <a href="/docs/cli">CLI reference</a>) and a running LogDeck server.
          Add one entry to your MCP client&apos;s configuration:
        </p>
      </div>

      <CodeBlock
        language="json"
        code={`{
  "mcpServers": {
    "logdeck": {
      "command": "logdeck",
      "args": ["mcp"],
      "env": {
        "LOGDECK_URL": "https://logdeck.example.com",
        "LOGDECK_TOKEN": "ldk_your_read_token"
      }
    }
  }
}`}
      />

      <div className="prose prose-neutral dark:prose-invert max-w-none">
        <p className="my-4 text-base">
          The server reads its connection from <code>LOGDECK_URL</code> and{" "}
          <code>LOGDECK_TOKEN</code> (or a saved CLI context). On startup it
          prints the server it is serving to stderr, and warns if the token is
          not a scoped <code>ldk_</code> API token.
        </p>

        <h2 className="mb-4 mt-10 text-3xl font-bold tracking-tight">
          Capability follows the token
        </h2>
        <p className="mb-4 text-base">
          The MCP server never widens what your token can do. Mutations are
          enforced by the LogDeck server, not the MCP layer, so the safe path is
          the default:
        </p>
        <ul className="mb-4 list-disc space-y-1 pl-6 text-base">
          <li>
            A <strong>read-scoped token</strong> (<code>ldk_</code>, created in
            Settings → API access) can read logs, stats, events, and container
            details, but every action tool returns a permission error. This is
            what you hand to an assistant by default — safe by construction. See{" "}
            <a href="/docs/configuration">scoped API tokens</a>.
          </li>
          <li>
            An <strong>admin token</strong> can do everything an admin can do,
            including exec, environment variables, and settings — the same reach
            it has in the CLI and the web UI. You opt into that simply by
            choosing which token to configure.
          </li>
        </ul>

        <h2 className="mb-4 mt-10 text-3xl font-bold tracking-tight">
          Read tools
        </h2>
        <p className="mb-4 text-base">
          These are always available and are read-only.
        </p>
      </div>

      <div className="space-y-6">
        {readTools.map((tool) => (
          <div key={tool.name} className="space-y-1">
            <h3 className="font-mono text-base font-semibold">{tool.name}</h3>
            <p className="text-sm text-muted-foreground">{tool.summary}</p>
          </div>
        ))}
      </div>

      <div className="prose prose-neutral dark:prose-invert max-w-none">
        <h2 className="mb-4 mt-10 text-3xl font-bold tracking-tight">
          Action tools
        </h2>
        <p className="mb-4 text-base">
          These need an admin token. There are no flags to set: hand the
          assistant the token you want it to have, exactly as you would for the
          CLI.
        </p>
      </div>

      <div className="space-y-6">
        {actionTools.map((tool) => (
          <div key={tool.name} className="space-y-1">
            <h3 className="font-mono text-base font-semibold">{tool.name}</h3>
            <p className="text-sm text-muted-foreground">{tool.summary}</p>
          </div>
        ))}
      </div>

      <div className="prose prose-neutral dark:prose-invert max-w-none">
        <p className="my-4 text-base">
          Client confirmation prompts are a convenience, not a security boundary
          — the token scope is. A read-scoped token cannot use any action tool:
          the server rejects every mutation, and denies the env and settings
          surfaces outright. Destructive tools carry the protocol&apos;s
          destructive hint, so a well-behaved client prompts harder before
          running them.
        </p>

        <h2 className="mb-4 mt-10 text-3xl font-bold tracking-tight">Notes</h2>
        <ul className="mb-4 list-disc space-y-1 pl-6 text-base">
          <li>
            Log tools default to a small tail and cap the number of lines
            returned, to stay within an assistant&apos;s context.{" "}
            <code>history_search</code> is cursor-paginated for walking further
            back.
          </li>
          <li>
            <code>run_command</code> is non-interactive: it runs one command,
            returns its output and exit code, and does not attach a terminal.
            For an interactive shell, use the web terminal.
          </li>
        </ul>
      </div>
    </div>
  );
}
