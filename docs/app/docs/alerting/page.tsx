import { Info } from "lucide-react";
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
  title: "Alerting",
  description:
    "Alert on container deaths, OOM kills, and log patterns. Rate thresholds, per-rule cooldowns, a generic JSON webhook that works with Slack and Discord, and alert history.",
  alternates: { canonical: "/docs/alerting" },
};

export default function AlertingPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="scroll-m-20 text-4xl font-bold tracking-tight">
          Alerting
        </h1>
        <p className="text-lg text-muted-foreground mt-2">
          Get told when a container dies, gets OOM-killed, or starts logging
          errors.
        </p>
      </div>

      <Separator />

      <div className="prose prose-neutral dark:prose-invert max-w-none">
        <p className="text-base">
          LogDeck already watches every container&apos;s events and log stream.
          Alert rules put that to use: they match on what LogDeck sees and POST
          a JSON payload to one webhook URL. Rules are managed under{" "}
          <strong>Settings &rarr; Alerts</strong> in the UI, or with{" "}
          <code>logdeck alerts</code> from the terminal.
        </p>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">Rule types</h2>

        <h3 className="mb-3 mt-8 text-xl font-semibold">Event rules</h3>
        <p className="mb-4 text-base">
          Event rules watch container lifecycle events. Two are alertable:
        </p>
        <ul className="mb-4 space-y-2">
          <li>
            <code>die</code> — the container exited. Only a{" "}
            <strong>non-zero exit code</strong> fires the rule; a clean exit
            (code 0) is not an alert condition. When the engine event arrives
            without an exit code, LogDeck inspects the container to find it.
          </li>
          <li>
            <code>oom</code> — the container was killed by the OOM killer.
          </li>
        </ul>
        <p className="mb-6 text-base">
          An OOM kill usually emits <code>oom</code> immediately followed by a{" "}
          <code>die</code> with code 137. For a rule that watches both, LogDeck
          counts that pair as one incident rather than alerting twice.
        </p>

        <h3 className="mb-3 mt-8 text-xl font-semibold">Log rules</h3>
        <p className="mb-4 text-base">
          Log rules match lines as they stream. A rule can set a minimum level,
          a regex pattern, or both — both must match:
        </p>
        <ul className="mb-6 space-y-2">
          <li>
            <strong>Minimum level</strong> — matches any line at that level or
            more severe (<code>TRACE</code>, <code>DEBUG</code>,{" "}
            <code>INFO</code>, <code>WARN</code>, <code>ERROR</code>,{" "}
            <code>FATAL</code>, <code>PANIC</code>). Lines LogDeck cannot
            classify never pass a level filter.
          </li>
          <li>
            <strong>Pattern</strong> — an RE2 regular expression, tested against
            the parsed message and the raw line.
          </li>
        </ul>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">Targeting</h2>
        <p className="mb-4 text-base">
          Every rule can be narrowed by <strong>hosts</strong>,{" "}
          <strong>container names</strong> (exact), and{" "}
          <strong>Compose projects</strong>. The dimensions are combined with
          AND; a dimension you leave empty matches everything. A rule with no
          targeting at all watches every container on every host.
        </p>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">
          Rate thresholds and cooldowns
        </h2>
        <p className="mb-4 text-base">
          A single stray error is rarely worth a notification. Two controls turn
          matches into alerts:
        </p>
        <ul className="mb-6 space-y-2">
          <li>
            <strong>Threshold and window</strong> — fire only after N matches
            within W seconds (&quot;5 errors in 60 seconds&quot;). The window
            defaults to 60 seconds. A threshold of 0 or 1 fires on every match.
          </li>
          <li>
            <strong>Cooldown</strong> — the minimum time between deliveries for
            the same rule and container. It defaults to{" "}
            <strong>300 seconds</strong> when unset. Matches that occur during a
            cooldown are not thrown away: they are counted, and the next alert
            reports how many were <strong>suppressed</strong>.
          </li>
        </ul>
        <p className="mb-8 text-base">
          Threshold and cooldown state is kept in memory, so it resets when
          LogDeck restarts.
        </p>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">The webhook</h2>
        <p className="mb-4 text-base">
          There is one webhook URL for the whole instance, set under{" "}
          <strong>Settings &rarr; Alerts</strong> (or{" "}
          <code>logdeck alerts webhook set &lt;url&gt;</code>). Every fired
          alert is POSTed to it as JSON:
        </p>

        <div className="not-prose mb-6">
          <CodeBlock
            code={`{
  "source": "logdeck",
  "version": 1,
  "text": "LogDeck alert: error spike: 5 matches (level >= ERROR) within 60s (prod/api)",
  "content": "LogDeck alert: error spike: 5 matches (level >= ERROR) within 60s (prod/api)",
  "alert": {
    "id": "...",
    "ruleId": "...",
    "ruleName": "error spike",
    "type": "log",
    "host": "prod",
    "containerId": "...",
    "containerName": "api",
    "reason": "5 matches (level >= ERROR) within 60s",
    "sample": "level=error msg=\\"upstream timeout\\"",
    "count": 5,
    "suppressed": 0,
    "firedAt": "2026-07-14T09:31:04Z"
  }
}`}
            language="json"
          />
        </div>

        <div className="not-prose mb-8">
          <Card className="border-blue-200 bg-blue-50 dark:border-blue-900 dark:bg-blue-950/20">
            <CardHeader>
              <div className="flex items-start gap-2">
                <Info className="h-5 w-5 text-blue-600 dark:text-blue-500 shrink-0 mt-0.5" />
                <div>
                  <CardTitle className="text-blue-900 dark:text-blue-200">
                    Why it works with Slack and Discord unchanged
                  </CardTitle>
                </div>
              </div>
            </CardHeader>
            <CardContent className="text-sm text-blue-900 dark:text-blue-200 space-y-2">
              <p>
                The same human-readable summary is sent twice, as{" "}
                <code>text</code> and as <code>content</code>. Slack (and
                Mattermost) render <code>text</code>; Discord renders{" "}
                <code>content</code>. So a Slack or Discord incoming-webhook URL
                works as-is, with no proxy or template in between.
              </p>
              <p>
                Anything else that accepts a JSON POST works too — the full
                alert object is in the body for receivers that want to parse it.
              </p>
            </CardContent>
          </Card>
        </div>

        <p className="mb-4 text-base">
          Delivery has a 10-second timeout. Network errors and 5xx responses are
          retried once after 5 seconds; other statuses are treated as permanent.
          The outcome of each delivery (status, HTTP code, error) is recorded in
          the alert history.
        </p>
        <p className="mb-8 text-base">
          Use <strong>Send test</strong> in Settings (or{" "}
          <code>logdeck alerts test</code>) to verify the URL. With no webhook
          configured, rules still evaluate and fire — the alerts are simply
          recorded in history and delivered nowhere.
        </p>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">
          Alert history
        </h2>
        <p className="mb-8 text-base">
          LogDeck keeps the most recent 500 fired alerts, newest first, and
          mirrors them to <code>alerts-history.json</code> next to the config
          file — so history survives a restart, provided that directory is a
          mounted volume. Each entry records the rule, the container and host,
          the reason, a sample line for log rules, how many matches were
          suppressed, and the delivery result. Read it under{" "}
          <strong>Settings &rarr; Alerts</strong>, with{" "}
          <code>logdeck alerts history</code>, or from{" "}
          <code>GET /api/v1/alerts/history</code>. Clearing it is a single
          action in the UI, or <code>logdeck alerts history clear</code>.
        </p>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">
          Managing rules from the CLI
        </h2>
        <p className="mb-6 text-base">
          Everything in the Alerts settings card is available from the CLI,
          which makes rules reproducible across deployments.
        </p>
      </div>

      <CodeBlock
        code={`# Point alerts somewhere
logdeck alerts webhook set https://hooks.slack.com/services/...
logdeck alerts test

# Tell me when anything gets OOM-killed
logdeck alerts rules create --type event --name oom-watch --events oom

# Tell me when the api container crash-loops: 3 non-zero exits in 5 minutes
logdeck alerts rules create --type event --name api-crashloop \\
  --events die --container api --threshold 3 --window 5m

# Tell me when prod starts spewing errors, at most once every 10 minutes
logdeck alerts rules create --type log --name error-spike \\
  --min-level ERROR --host prod --threshold 5 --window 60s --cooldown 10m

# Match a specific failure, wherever it happens
logdeck alerts rules create --type log --name upstream-timeouts \\
  --pattern "upstream (timed out|timeout)" --project checkout

# Inspect and manage
logdeck alerts rules                  # list, with targets and triggers
logdeck alerts rules disable <id>     # or enable / delete
logdeck alerts history --limit 20`}
        language="bash"
      />

      <div className="prose prose-neutral dark:prose-invert max-w-none">
        <p className="my-4 text-base">
          <code>--host</code>, <code>--container</code>, and{" "}
          <code>--project</code> are repeatable and narrow the rule.{" "}
          <code>--window</code> and <code>--cooldown</code> accept durations (
          <code>60s</code>, <code>5m</code>) or bare seconds. See the{" "}
          <a href="/docs/cli#alerts">CLI reference</a> for every flag.
        </p>

        <Separator className="my-12" />

        <h2 className="mb-4 text-3xl font-bold tracking-tight">Storage</h2>
      </div>

      <div className="not-prose">
        <Card>
          <CardHeader>
            <CardTitle className="text-lg">
              Rules live in the config file
            </CardTitle>
            <CardDescription>
              Under <code>alerts</code>, alongside hosts and API tokens
            </CardDescription>
          </CardHeader>
          <CardContent className="text-sm text-muted-foreground space-y-2">
            <p>
              Alert rules and the webhook URL are persisted to{" "}
              <code>config.json</code> (<code>/data/config.json</code> by
              default), and fired alerts to <code>alerts-history.json</code>{" "}
              beside it. Mount <code>/data</code> as a volume, or both are lost
              when the LogDeck container is recreated.
            </p>
            <p>
              There are no alert-related environment variables: alerting is
              configured through the UI or the CLI only.
            </p>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
