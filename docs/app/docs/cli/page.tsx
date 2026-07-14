import type { Metadata } from "next";
import { CodeBlock } from "@/components/landing/code-block";
import { Separator } from "@/components/ui/separator";

export const metadata: Metadata = {
  title: "CLI reference",
  description:
    "Reference for the logdeck CLI: install it, log in with contexts and API tokens, then read logs, grep across containers, and manage Docker and Podman hosts.",
  alternates: { canonical: "/docs/cli" },
};

const commands = [
  {
    name: "login",
    summary:
      "Verify a server connection and save it as the current context. Fails with a hint pointing at Settings if the server requires authentication and no working token is given.",
    example:
      "logdeck login --url https://logdeck.example.com --token ldk_... --name prod",
  },
  {
    name: "context",
    summary: "Manage saved contexts: list, use <name>, rm <name>.",
    example: "logdeck context list",
  },
  {
    name: "logout",
    summary:
      "Remove the saved token from the current (or a named) context, keeping its URL.",
    example: "logdeck logout prod",
  },
  {
    name: "status",
    summary:
      "Server health, version, and a per-host summary, plus where the connection settings came from (flag, env, or context). The natural first call to discover what a server manages. Exits nonzero if the server is unreachable.",
    example: "logdeck status",
  },
  {
    name: "containers",
    summary: "List containers across all hosts, with optional filters.",
    example: "logdeck containers --state running --host prod",
  },
  {
    name: "stacks",
    summary:
      "List compose projects (grouped by the com.docker.compose.project / io.podman.compose.project labels) with container counts and hosts.",
    example: "logdeck stacks",
  },
  {
    name: "inspect",
    summary:
      "Full inspect data for one container. Table mode shows key facts; -o json prints the complete inspect document.",
    example: "logdeck inspect web -o json",
  },
  {
    name: "logs",
    summary:
      "Read or follow the parsed logs of a container, or a whole compose stack with --stack. --since/--until accept RFC3339 timestamps or relative durations (30s, 15m, 2h, 1d). Stack logs are merged by timestamp with the container name shown per line.",
    example: `logdeck logs web --tail 200 --level ERROR --since 1h
logdeck logs web --follow
logdeck logs --stack myapp --search "timeout" --since 30m`,
  },
  {
    name: "grep",
    summary:
      "Search the recent logs of every running container across all hosts, merged by timestamp. Bounded to the last 15 minutes by default so it stays fast.",
    example: `logdeck grep "connection refused" --since 1h --level ERROR`,
  },
  {
    name: "stats",
    summary: "CPU and memory usage for all running containers, or one.",
    example: `logdeck stats
logdeck stats web`,
  },
  {
    name: "events",
    summary:
      "Stream container lifecycle events (start, stop, die, ...). Streams until interrupted, or use --for to read for a fixed duration and exit.",
    example: "logdeck events --for 30s",
  },
  {
    name: "start / stop / restart / rm",
    summary:
      "Container lifecycle actions. Containers are matched by exact name first, then ID prefix; ambiguous matches list the candidates and --host disambiguates.",
    example: `logdeck restart web
logdeck stop web --host staging`,
  },
  {
    name: "stack",
    summary:
      "Start, stop, or restart every container of a compose project. Applies to every host that has the project unless --host narrows it.",
    example: "logdeck stack restart myapp",
  },
  {
    name: "env",
    summary: "Print a container's environment variables as KEY=value lines.",
    example: "logdeck env web",
  },
  {
    name: "resources",
    summary:
      "Show or update a container's resource limits and restart policy. Memory accepts human units (512m, 1.5g), CPUs accept fractions.",
    example: `logdeck resources web
logdeck resources set web --memory 512m --cpus 1.5 --restart on-failure --max-retries 3`,
  },
  {
    name: "images / volumes / networks",
    summary:
      "Read-only listings across all hosts, with an optional --host filter.",
    example: `logdeck images --host prod
logdeck volumes
logdeck networks`,
  },
  {
    name: "alerts",
    summary:
      "Manage alerting: rules, the webhook destination, and fired-alert history. Rules match container events (die, oom) or log lines (minimum level and/or regex), and can require a threshold of matches within a window. --host, --container, and --project (repeatable) narrow which containers a rule watches. --window and --cooldown accept durations (60s, 5m) or bare seconds; an omitted cooldown means the server default of 300s.",
    example: `logdeck alerts rules
logdeck alerts rules create --type event --name oom-watch --events oom
logdeck alerts rules create --type log --name errors --min-level ERROR --threshold 5 --window 60s
logdeck alerts rules disable <id>
logdeck alerts webhook set https://hooks.example.com/logdeck
logdeck alerts test
logdeck alerts history --limit 20`,
  },
];

export default function CliPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="scroll-m-20 text-4xl font-bold tracking-tight">CLI</h1>
        <p className="text-lg text-muted-foreground mt-2">
          Everything the web UI can see, from your terminal — built for
          scripting and AI agents.
        </p>
      </div>

      <Separator />

      <div className="prose prose-neutral dark:prose-invert max-w-none">
        <p className="mb-4 text-base">
          <code>logdeck</code> is a command-line client for a running LogDeck
          server. It talks to the same HTTP API as the web interface, so
          everything you can see in the UI — containers, logs, stats, events,
          compose stacks — is available from the terminal.
        </p>
        <p className="mb-4 text-base">
          It is fully non-interactive: every command supports machine-readable
          JSON output (<code>-o json</code>), errors always go to stderr, and
          exit codes are consistent (0 success, 1 runtime error, 2 usage error).
        </p>

        <h2 className="mb-4 mt-10 text-3xl font-bold tracking-tight">
          Install
        </h2>
        <p className="mb-4 text-base">
          Install the latest release binary (macOS and Linux, amd64/arm64):
        </p>
      </div>

      <CodeBlock
        code="curl -fsSL https://raw.githubusercontent.com/AmoabaKelvin/logdeck/main/install.sh | sh"
        language="bash"
      />

      <div className="prose prose-neutral dark:prose-invert max-w-none">
        <p className="my-4 text-base">
          Binaries are published on{" "}
          <a
            href="https://github.com/AmoabaKelvin/logdeck/releases"
            target="_blank"
            rel="noopener noreferrer"
          >
            GitHub Releases
          </a>{" "}
          with checksums; the installer picks the right one for your
          OS/architecture and installs to <code>/usr/local/bin</code> or{" "}
          <code>~/.local/bin</code>. Check your version with{" "}
          <code>logdeck --version</code>.
        </p>
        <p className="mb-4 text-base">
          Or build from source — the CLI lives in the same Go module as the
          server and builds to a single static binary:
        </p>
      </div>

      <CodeBlock
        code={`cd server
go build ./cmd/logdeck
./logdeck --help`}
        language="bash"
      />

      <div className="prose prose-neutral dark:prose-invert max-w-none">
        <h2 className="mb-4 mt-10 text-3xl font-bold tracking-tight">
          Connection and authentication
        </h2>
        <p className="mb-4 text-base">
          Connect once with <code>logdeck login</code> — it verifies the
          connection and saves it as a named context, kubectl-style. From then
          on every command uses that context:
        </p>
      </div>

      <CodeBlock
        code={`logdeck login --url https://logdeck.example.com --token ldk_... --name prod
logdeck status        # now talks to prod`}
        language="bash"
      />

      <div className="prose prose-neutral dark:prose-invert max-w-none">
        <p className="my-4 text-base">
          Contexts persist in <code>~/.config/logdeck/config.json</code>{" "}
          (respecting <code>XDG_CONFIG_HOME</code>). Because the file stores API
          tokens, it is created with <code>0600</code> permissions inside a{" "}
          <code>0700</code> directory, and the CLI never prints a saved token —
          only its <code>ldk_</code> prefix.
        </p>
        <p className="mb-4 text-base">
          API tokens are created in the LogDeck web UI under{" "}
          <strong>Settings → API Tokens</strong> and sent as{" "}
          <code>Authorization: Bearer &lt;token&gt;</code>. Tokens have a scope:{" "}
          <strong>admin</strong> tokens have full access, while{" "}
          <strong>read-only</strong> tokens can read logs, stats, container
          details, and events but cannot mutate anything, use the web terminal,
          or read container environment variables or settings — useful for CI
          jobs or AI agents that only need to read. When the server has
          authentication disabled, no token is needed.
        </p>

        <h3 className="mb-3 mt-8 text-xl font-semibold">Resolution order</h3>
        <p className="mb-4 text-base">
          Flags and environment variables override the saved context — useful
          for CI or one-off calls. URL and token resolve independently, each
          from the first source that provides it:
        </p>
        <ol className="mb-4 list-decimal space-y-1.5 pl-6 text-base">
          <li>
            Explicit <code>--url</code> / <code>--token</code> flags
          </li>
          <li>
            <code>LOGDECK_URL</code> / <code>LOGDECK_TOKEN</code> environment
            variables
          </li>
          <li>
            The active context from the config file (<code>--context</code>{" "}
            selects another saved context for one invocation)
          </li>
          <li>
            Default <code>http://localhost:8080</code>
          </li>
        </ol>
        <p className="mb-4 text-base">
          <code>logdeck status</code> shows which source supplied the URL and
          token.
        </p>

        <h2 className="mb-4 mt-10 text-3xl font-bold tracking-tight">
          Output formats
        </h2>
        <p className="mb-4 text-base">
          Every command accepts <code>-o/--output</code>:
        </p>
        <ul className="mb-4 list-disc space-y-1.5 pl-6 text-base">
          <li>
            <code>table</code> (default): compact aligned columns for humans.
          </li>
          <li>
            <code>json</code>: a single JSON document for one-shot commands;
            NDJSON (one JSON object per line) for streaming commands (
            <code>logs --follow</code>, <code>events</code>).
          </li>
        </ul>
        <p className="mb-4 text-base">
          Timestamps are RFC3339. There are no colors, spinners, prompts, or
          pagination.
        </p>

        <h2 className="mb-4 mt-10 text-3xl font-bold tracking-tight">
          Commands
        </h2>
      </div>

      <div className="space-y-8">
        {commands.map((cmd) => (
          <div key={cmd.name} className="space-y-3">
            <h3
              className="font-mono text-lg font-semibold"
              id={cmd.name.replaceAll(" ", "")}
            >
              {cmd.name}
            </h3>
            <p className="text-sm text-muted-foreground">{cmd.summary}</p>
            <CodeBlock code={cmd.example} language="bash" />
          </div>
        ))}
      </div>

      <Separator className="my-8" />

      <div className="prose prose-neutral dark:prose-invert max-w-none">
        <h2 className="mb-4 text-3xl font-bold tracking-tight">
          Using with AI agents
        </h2>
        <p className="mb-4 text-base">
          The CLI is designed so an agent can debug containerized services
          without a browser. A typical investigation:
        </p>
      </div>

      <CodeBlock
        code={`# One-time setup on this machine (or use LOGDECK_URL/LOGDECK_TOKEN in CI)
logdeck login --url https://logdeck.example.com --token ldk_...

# What is running, and is the server healthy?
logdeck status -o json
logdeck containers -o json

# Anything failing right now?
logdeck grep "error|exception|panic" --since 15m -o json

# Zoom into the suspect service
logdeck logs api --tail 500 --level ERROR --since 1h -o json
logdeck inspect api -o json
logdeck stats api -o json

# Act, then confirm
logdeck restart api
logdeck logs api --follow`}
        language="bash"
      />

      <div className="prose prose-neutral dark:prose-invert max-w-none">
        <p className="my-4 text-base">Notes for agents:</p>
        <ul className="mb-4 list-disc space-y-1.5 pl-6 text-base">
          <li>
            <code>-o json</code> always emits a single JSON document on stdout
            for one-shot commands; streaming commands emit NDJSON, one object
            per line.
          </li>
          <li>
            Errors go to stderr as <code>{`{"error": "..."}`}</code> in JSON
            mode; stdout stays clean for parsing.
          </li>
          <li>Exit codes: 0 success, 1 runtime/server error, 2 usage error.</li>
          <li>
            <code>--since</code>/<code>--until</code> accept relative durations
            (<code>15m</code>, <code>2h</code>, <code>1d</code>), so no date
            math is needed.
          </li>
          <li>
            <code>logdeck grep</code> is the fastest way to find which container
            is emitting an error across an entire deployment.
          </li>
        </ul>
      </div>
    </div>
  );
}
