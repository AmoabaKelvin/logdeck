import { ArrowRight } from "lucide-react";
import Link from "next/link";

import { CodeBlock } from "./code-block";

const session = `# search the last hour of logs across every container
logdeck grep "connection refused" --since 1h

# follow one service, errors only
logdeck logs api --follow --level ERROR

# act, then confirm
logdeck restart redis --host prod
logdeck stats redis`;

export function Cli() {
  return (
    <section className="container py-20 sm:py-24">
      <div className="grid items-center gap-10 lg:grid-cols-2">
        <div className="max-w-xl">
          <h2 className="text-2xl font-semibold tracking-tight sm:text-3xl">
            Also a command-line client
          </h2>
          <p className="mt-3 text-muted-foreground">
            The <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-sm">logdeck</code>{" "}
            CLI talks to the same API as the web UI. Search logs across every
            container, follow a service, or restart a stack — with JSON output
            for scripts and AI agents.
          </p>
          <Link
            href="/docs/cli"
            className="mt-5 inline-flex items-center gap-1.5 text-sm font-medium hover:underline"
          >
            Read the CLI guide
            <ArrowRight className="h-4 w-4" />
          </Link>
        </div>
        <CodeBlock code={session} language="bash" />
      </div>
    </section>
  );
}
