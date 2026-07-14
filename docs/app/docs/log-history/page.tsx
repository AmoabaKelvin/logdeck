import { AlertTriangle, Info } from "lucide-react";
import type { Metadata } from "next";
import { CodeBlock } from "@/components/landing/code-block";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";

export const metadata: Metadata = {
  title: "Log history",
  description:
    "LogDeck persists container logs to a local SQLite store so history survives restarts, rebuilds, and removal. Configure retention, search history, and read logs of containers that no longer exist.",
  alternates: { canonical: "/docs/log-history" },
};

export default function LogHistoryPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="scroll-m-20 text-4xl font-bold tracking-tight">
          Log History
        </h1>
        <p className="text-lg text-muted-foreground mt-2">
          LogDeck stores container logs locally, so history outlives the
          containers that produced it.
        </p>
      </div>

      <Separator />

      <div className="prose prose-neutral dark:prose-invert max-w-none">
        <p className="text-base">
          By default, LogDeck tails every container on every configured host and
          writes those lines to a SQLite database on its own disk. That store is
          what powers <strong>History</strong> mode in the log viewer: logs you
          can still read after a container restarts, after it is rebuilt with a
          new image, and even after it is removed entirely.
        </p>
        <p className="text-base">
          This is a local convenience store, not a log aggregation platform. It
          is bounded by retention caps, it lives on the machine running LogDeck,
          and it is not replicated anywhere.
        </p>
      </div>

      <div className="not-prose">
        <Card className="border-amber-200 bg-amber-50 dark:border-amber-900 dark:bg-amber-950/20">
          <CardHeader>
            <div className="flex items-start gap-2">
              <AlertTriangle className="h-5 w-5 text-amber-600 dark:text-amber-500 mt-0.5" />
              <div>
                <CardTitle className="text-amber-900 dark:text-amber-200">
                  Mount a volume, or history is not history
                </CardTitle>
                <CardDescription className="text-amber-800 dark:text-amber-300">
                  The database lives next to the config file
                </CardDescription>
              </div>
            </div>
          </CardHeader>
          <CardContent className="text-sm text-amber-900 dark:text-amber-200 space-y-2">
            <p>
              LogDeck writes <code>logs.db</code> into the same directory as its
              config file — <code>/data/logs.db</code> with the default{" "}
              <code>CONFIG_PATH</code>. If <code>/data</code> is not a mounted
              volume, the database is stored inside the container&apos;s
              filesystem and is destroyed the next time you recreate LogDeck.
            </p>
            <p>
              The same volume also holds <code>config.json</code> (hosts, API
              tokens, alert rules) and <code>alerts-history.json</code>.
            </p>
          </CardContent>
        </Card>
      </div>

      <div className="prose prose-neutral dark:prose-invert max-w-none">
        <div className="not-prose my-6">
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
      # Config file, stored logs, and alert history live here
      - logdeck-data:/data
    restart: unless-stopped

volumes:
  logdeck-data:`}
            language="yaml"
          />
        </div>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">
          What survives what
        </h2>
        <p className="mb-4 text-base">
          A container&apos;s stored history is keyed by its host and its{" "}
          <strong>name</strong>, not by its engine ID. Every engine container ID
          is recorded as a separate generation of that name, and queries stitch
          the generations back together in timestamp order. That is what makes
          history survive operations that give a container a brand-new ID:
        </p>
        <ul className="mb-6 space-y-2">
          <li>
            <strong>Restart</strong> (<code>docker restart</code>, a crash loop,
            a restart policy) — one continuous timeline.
          </li>
          <li>
            <strong>Rebuild or recreate</strong> (
            <code>docker compose up -d --build</code>, an image bump, an
            environment-variable edit in LogDeck) — the new container appends to
            the same timeline under the same name.
          </li>
          <li>
            <strong>Removal</strong> — the lines already stored stay readable
            (see below), even though the container is gone from the engine.
          </li>
          <li>
            <strong>Restarting LogDeck itself</strong> — on startup, LogDeck
            re-reads each container&apos;s engine logs from where it left off,
            so lines emitted while it was down are backfilled rather than lost.
          </li>
        </ul>

        <div className="not-prose mb-8">
          <Card className="border-blue-200 bg-blue-50 dark:border-blue-900 dark:bg-blue-950/20">
            <CardHeader>
              <div className="flex items-start gap-2">
                <Info className="h-5 w-5 text-blue-600 dark:text-blue-500 shrink-0 mt-0.5" />
                <div>
                  <CardTitle className="text-blue-900 dark:text-blue-200">
                    Limits worth knowing
                  </CardTitle>
                </div>
              </div>
            </CardHeader>
            <CardContent className="text-sm text-blue-900 dark:text-blue-200 space-y-2">
              <p>
                • Backfill reads the logs the <em>engine</em> still holds. If a
                container is removed while LogDeck is down, the engine discards
                its logs with it, and whatever LogDeck had not already stored is
                unrecoverable.
              </p>
              <p>
                • Containers whose logging driver has no read API (
                <code>awslogs</code>, <code>syslog</code>, <code>none</code>, …)
                cannot be read by LogDeck at all. They are excluded from the
                store, and the reason is written to the server log.
              </p>
              <p>
                • Renaming a container starts a new timeline, because the name
                is the identity.
              </p>
            </CardContent>
          </Card>
        </div>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">
          Using History mode
        </h2>
        <p className="mb-4 text-base">
          On a container&apos;s log page, the toolbar shows a{" "}
          <strong>Live | History</strong> toggle whenever the store is enabled.
          Live streams from the engine as always; History queries the database.
        </p>
        <p className="mb-4 text-base">In History mode:</p>
        <ul className="mb-6 space-y-2">
          <li>
            <strong>
              Search, level filter, and time range are applied server-side
            </strong>{" "}
            across everything stored for that container — not just the lines
            currently loaded in the browser. Search accepts plain text or a
            regex.
          </li>
          <li>
            Results page backwards from the newest line. A{" "}
            <strong>Load older</strong> button fetches the previous page (500
            lines) until you reach the beginning of stored history.
          </li>
          <li>
            Timestamps, wrapping, line selection, pinning, copying, and
            downloading (JSON or TXT) all work exactly as they do in Live mode.
          </li>
          <li>
            Streaming controls (Stream, Pause, tail size, auto-scroll) are
            hidden — there is nothing to stream.
          </li>
        </ul>
        <p className="mb-6 text-base">
          History is available for single containers on the container log page.{" "}
          <strong>Aggregated Compose stack logs are live-only</strong>: the
          stack view merges live streams and has no History toggle. The
          quick-look log sheet on the dashboard is live-only too — open the
          container&apos;s full log page for history.
        </p>

        <h3 className="mb-4 mt-8 text-xl font-semibold">Removed containers</h3>
        <p className="mb-4 text-base">
          When LogDeck holds stored logs for a container that no longer exists
          on any host, the dashboard&apos;s state summary grows a{" "}
          <strong>Removed</strong> chip. Removed containers are hidden under
          &quot;All states&quot; — click the chip (or pick <em>Removed</em> in
          the state filter) to list them.
        </p>
        <p className="mb-8 text-base">
          A removed container shows how much log data is stored for it instead
          of CPU and memory, and offers a single action:{" "}
          <strong>View stored logs</strong>. Its log page opens locked to
          History; there is no live stream, no terminal, and no environment or
          resources tab, because there is no container left to inspect.
        </p>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">
          Retention and disk use
        </h2>
        <p className="mb-4 text-base">
          Two caps bound the store, and a sweep runs every minute to enforce
          them by evicting the <strong>oldest lines first</strong>:
        </p>
        <ul className="mb-6 space-y-2">
          <li>
            <strong>Per container</strong> (default <code>50</code> MB) —
            applied to a logical container, meaning all generations of the same
            name together. A rebuilt container does not get a fresh budget.
          </li>
          <li>
            <strong>Total</strong> (default <code>1024</code> MB) — applied to
            the whole store, across every host and container.
          </li>
        </ul>
        <p className="mb-8 text-base">
          The database file is never vacuumed: SQLite reuses freed pages, so
          after eviction the file plateaus at its high-water mark rather than
          shrinking. Size <code>/data</code> for roughly the total cap plus
          headroom.
        </p>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">
          Configuration
        </h2>
        <p className="mb-4 text-base">
          Persistence is <strong>enabled by default</strong>. It is configured
          in the config file under <code>logStore</code>, and every field can be
          overridden by an environment variable, which wins over the file. There
          is no Settings page for it.
        </p>

        <div className="not-prose mb-6">
          <CodeBlock
            code={`{
  "logStore": {
    "enabled": true,
    "perContainerMB": 50,
    "totalMB": 1024
  }
}`}
            language="json"
          />
        </div>

        <div className="not-prose">
          <Card>
            <CardHeader>
              <CardTitle className="text-lg">Environment overrides</CardTitle>
              <CardDescription>
                Each one overrides the matching config-file field
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div>
                <code className="text-sm bg-muted px-2 py-1 rounded">
                  LOG_STORE_ENABLED
                </code>
                <p className="text-sm text-muted-foreground mt-1">
                  <code>false</code> turns persistence off entirely: no database
                  file is created, History mode disappears from the UI, and the
                  history endpoints report the store as disabled. Existing data
                  is left on disk untouched. Default: <code>true</code>.
                </p>
              </div>
              <div>
                <code className="text-sm bg-muted px-2 py-1 rounded">
                  LOG_STORE_PER_CONTAINER_MB
                </code>
                <p className="text-sm text-muted-foreground mt-1">
                  Per-container retention cap in MB. Must be a positive integer;
                  anything else is ignored with a warning. Default:{" "}
                  <code>50</code>.
                </p>
              </div>
              <div>
                <code className="text-sm bg-muted px-2 py-1 rounded">
                  LOG_STORE_TOTAL_MB
                </code>
                <p className="text-sm text-muted-foreground mt-1">
                  Total retention cap in MB across the whole store. Must be a
                  positive integer. Default: <code>1024</code>.
                </p>
              </div>
            </CardContent>
          </Card>
        </div>

        <p className="mt-6 mb-8 text-base">
          If the database cannot be opened — a read-only volume, a missing mount
          — LogDeck logs a warning and keeps running <em>without</em> stored
          logs. Persistence never blocks startup. Check the server log for{" "}
          <code>Log persistence is ENABLED</code> (with the path and the caps)
          to confirm it came up.
        </p>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">API</h2>
        <p className="mb-4 text-base">
          The store is queryable over the HTTP API. These are read endpoints, so
          a <code>read</code>-scoped API token can call them.
        </p>
        <ul className="mb-6 space-y-2">
          <li>
            <code>GET /api/v1/history/status</code> — whether persistence is
            available (<code>{`{"enabled": true}`}</code>).
          </li>
          <li>
            <code>GET /api/v1/history/containers</code> — every logical
            container the store knows about, including removed ones, with their
            stored size.
          </li>
          <li>
            <code>GET /api/v1/history/logs</code> — one page of stored logs.
            Returns <code>503</code> when persistence is disabled.
          </li>
        </ul>
        <p className="mb-4 text-base">
          <code>/history/logs</code> takes <code>container</code> (required),{" "}
          <code>host</code>, <code>search</code>, <code>regex</code> (boolean),{" "}
          <code>levels</code> (comma-separated, including <code>UNKNOWN</code>),{" "}
          <code>since</code> and <code>until</code> (RFC3339),{" "}
          <code>limit</code> (default <code>500</code>, max <code>1000</code>),
          and <code>cursor</code>. Pages walk backwards through history: follow
          the returned <code>nextCursor</code> for older lines.
        </p>

        <div className="not-prose mb-8">
          <CodeBlock
            code={`curl -H "Authorization: Bearer ldk_..." \\
  "http://localhost:8123/api/v1/history/logs?container=api&host=local&levels=ERROR,FATAL&limit=200"`}
            language="bash"
          />
        </div>
      </div>
    </div>
  );
}
