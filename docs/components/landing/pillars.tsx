import {
  CpuSettingsIcon,
  DatabaseRestoreIcon,
  McpServerIcon,
  RadioTowerIcon,
} from "@hugeicons/core-free-icons";
import Link from "next/link";

import { Icon, Wrapper, h2Class, pill } from "./ui";

const level: Record<string, string> = {
  ERROR: "bg-[#ff421e]",
  WARN: "bg-[#e3962d]",
  INFO: "bg-[#60beff]",
};

const historyLines = [
  {
    ts: "23:58:41",
    level: "ERROR",
    msg: "upstream timeout after 30s (attempt 3/3)",
  },
  { ts: "23:59:07", level: "WARN", msg: "circuit open for payments-svc" },
  {
    ts: "00:02:19",
    level: "ERROR",
    msg: "upstream timeout after 30s (attempt 1/3)",
  },
  { ts: "00:02:20", level: "INFO", msg: "listening on :8080" },
];

function LogLine({ ts, level: lvl, msg }: (typeof historyLines)[number]) {
  return (
    <li className="flex items-center gap-3 px-4 py-1.5 font-mono text-xs">
      <span className="shrink-0 text-base-400 tabular-nums">{ts}</span>
      <span className={`size-2 shrink-0 rounded-full ${level[lvl]}`} />
      <span className="min-w-0 truncate text-base-800">{msg}</span>
    </li>
  );
}

function HistoryCard() {
  return (
    <div className="-mr-1 -mb-1 ml-auto max-w-md overflow-hidden rounded-tl-xl bg-white shadow-sm outline outline-sand-500/10">
      <div className="flex items-center justify-between px-4 py-2 text-xs text-base-600">
        <span className="font-medium text-base-900">stack-api</span>
        <span className="inline-flex rounded-full bg-sand-100 p-0.5">
          <span className="rounded-full px-3 py-1 text-base-500">Live</span>
          <span className="rounded-full bg-white px-3 py-1 text-base-900 shadow-sm">
            History
          </span>
        </span>
      </div>
      <div className="flex items-center gap-2 border-t border-sand-500/10 px-4 py-2 font-mono text-xs text-base-500">
        <span className="rounded-full bg-sand-100 px-2 py-0.5">
          timeout|circuit
        </span>
        <span className="rounded-full bg-sand-100 px-2 py-0.5">
          level ≥ WARN
        </span>
        <span className="rounded-full bg-sand-100 px-2 py-0.5">7d</span>
      </div>
      <ul className="border-t border-sand-500/10 py-1">
        {historyLines.slice(0, 2).map((l) => (
          <LogLine key={l.ts} {...l} />
        ))}
        <li className="my-1 flex items-center gap-2 bg-sand-50 px-4 py-1.5 text-xs text-base-500">
          <span className="h-px flex-1 border-t border-dashed border-base-300" />
          container recreated, history continues
          <span className="h-px flex-1 border-t border-dashed border-base-300" />
        </li>
        {historyLines.slice(2).map((l) => (
          <LogLine key={l.ts} {...l} />
        ))}
      </ul>
      <p className="border-t border-sand-500/10 px-4 py-2 text-xs text-base-500">
        4 matches across 2 container lifetimes
      </p>
    </div>
  );
}

const alerts = [
  { time: "03:12", what: "stack-worker died (exit 137)", via: "Telegram" },
  {
    time: "02:58",
    what: "api-errors: 7 ERROR in 60s",
    via: "ntfy · 212 repeats suppressed",
  },
  { time: "01:31", what: "stack-db unhealthy", via: "webhook" },
];

function AlertsCard() {
  return (
    <div className="flex flex-col gap-4">
      <div className="rounded-r-xl bg-white p-6 shadow outline outline-sand-500/10">
        <p className="text-sm text-base-500">Rule</p>
        <p className="mt-1 font-medium text-base-900">api-errors</p>
        <dl className="mt-4 grid grid-cols-3 gap-4 text-sm">
          <div>
            <dt className="text-base-500">Trigger</dt>
            <dd className="mt-0.5 font-mono text-xs text-base-900">
              ERROR ≥ 5 / 60s
            </dd>
          </div>
          <div>
            <dt className="text-base-500">Target</dt>
            <dd className="mt-0.5 font-mono text-xs text-base-900">
              prod / stack-api
            </dd>
          </div>
          <div>
            <dt className="text-base-500">Cooldown</dt>
            <dd className="mt-0.5 font-mono text-xs text-base-900">10m</dd>
          </div>
        </dl>
      </div>
      <div className="-mb-1 rounded-tr-xl bg-white p-6 shadow outline outline-sand-500/10">
        <p className="text-sm text-base-500">Alert history</p>
        <ul className="mt-3 divide-y divide-sand-500/10 text-sm">
          {alerts.map((a) => (
            <li key={a.time} className="flex items-baseline gap-3 py-2">
              <span className="shrink-0 font-mono text-xs text-base-400 tabular-nums">
                {a.time}
              </span>
              <span className="min-w-0 flex-1 truncate text-base-900">
                {a.what}
              </span>
              <span className="flex shrink-0 items-center gap-1.5 text-xs text-base-500">
                <span className="size-2 rounded-full bg-[#5aab69]" />
                {a.via}
              </span>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}

function ControlCard() {
  return (
    <div className="-mr-1 -mb-1 ml-auto max-w-md rounded-tl-xl bg-white p-4 shadow-sm outline outline-sand-500/10">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm text-base-500">Compose stack</p>
          <p className="font-medium text-base-900">demostack · 6 containers</p>
        </div>
        <div className="flex gap-1">
          {["Start", "Stop", "Restart"].map((a) => (
            <span
              key={a}
              className="rounded-full border border-base-300 px-3 py-1 text-xs text-base-700"
            >
              {a}
            </span>
          ))}
        </div>
      </div>
      <div className="mt-4 grid grid-cols-1 gap-x-8 gap-y-4 border-t border-base-100 pt-4 sm:grid-cols-2">
        <div>
          <p className="text-sm text-base-500">Memory limit</p>
          <p className="font-mono text-sm text-base-900">
            <span className="text-base-400 line-through">512M</span> 1G
          </p>
        </div>
        <div>
          <p className="text-sm text-base-500">CPU limit</p>
          <p className="font-mono text-sm text-base-900">
            <span className="text-base-400 line-through">1.0</span> 2.0
          </p>
        </div>
        <div>
          <p className="text-sm text-base-500">LOG_LEVEL</p>
          <p className="font-mono text-sm text-base-900">
            <span className="text-base-400 line-through">info</span> debug
          </p>
        </div>
        <div>
          <p className="text-sm text-base-500">Restart policy</p>
          <p className="font-mono text-sm text-base-900">unless-stopped</p>
        </div>
      </div>
      <p className="mt-4 flex items-center gap-2 text-xs text-base-500">
        <span className="size-2 rounded-full bg-[#5aab69]" />
        Limits applied live, no restart. Env change recreates the container.
      </p>
    </div>
  );
}

const readTools = [
  "list_containers",
  "get_logs",
  "search_logs",
  "history_search",
  "container_stats",
];
const adminTools = [
  "restart_container",
  "run_command",
  "set_env",
  "set_read_only",
];

function AgentCard() {
  return (
    <div className="flex flex-col gap-4">
      <div className="rounded-r-xl bg-white p-6 shadow outline outline-sand-500/10">
        <p className="text-sm text-base-500">claude_desktop_config.json</p>
        <pre className="mt-2 overflow-x-auto font-mono text-xs/5 text-base-800">{`"logdeck": {
  "command": "logdeck",
  "args": ["mcp"],
  "env": { "LOGDECK_TOKEN": "ldk_read_…" }
}`}</pre>
      </div>
      <div className="-mb-1 rounded-tr-xl bg-white p-6 shadow outline outline-sand-500/10">
        <p className="text-sm text-base-500">Read token</p>
        <div className="mt-2 flex flex-wrap gap-1.5">
          {readTools.map((t) => (
            <span
              key={t}
              className="rounded-full bg-sand-100 px-2.5 py-1 font-mono text-xs text-base-800"
            >
              {t}
            </span>
          ))}
        </div>
        <p className="mt-4 text-sm text-base-500">Admin token adds</p>
        <div className="mt-2 flex flex-wrap gap-1.5">
          {adminTools.map((t) => (
            <span
              key={t}
              className="rounded-full bg-accent-50 px-2.5 py-1 font-mono text-xs text-accent-800"
            >
              {t}
            </span>
          ))}
        </div>
      </div>
    </div>
  );
}

export function Pillars() {
  return (
    <section id="features">
      <Wrapper className="flex flex-col gap-24 border-t border-dashed border-base-200 py-12">
        <div className="grid grid-cols-1 items-start gap-12 md:grid-cols-2">
          <div className="flex flex-col gap-4 text-balance">
            <div className="inline-flex w-fit rounded-lg bg-sand-100 p-4">
              <Icon icon={DatabaseRestoreIcon} className="text-accent-600" />
            </div>
            <div>
              <h2 className={h2Class}>Logs that outlive the container</h2>
              <p className="mt-4 max-w-xl text-pretty text-base text-base-500">
                Every container’s logs are stored on LogDeck’s own disk.
                Restart, rebuild, or remove it and the history is still there,
                searchable by regex, level, and time.
              </p>
              <div className="mt-4 flex">
                <Link href="/docs/log-history" className={pill.accent}>
                  How history works
                </Link>
              </div>
            </div>
          </div>
          <div
            aria-hidden="true"
            className="overflow-hidden rounded-xl bg-sand-100 p-8 pr-0 pb-0"
          >
            <HistoryCard />
          </div>
        </div>

        <div className="grid grid-cols-1 items-start gap-12 md:grid-cols-2">
          <div className="flex flex-col gap-4 text-balance md:order-last">
            <div className="inline-flex w-fit rounded-lg bg-sand-100 p-4">
              <Icon icon={RadioTowerIcon} className="text-accent-600" />
            </div>
            <div>
              <h2 className={h2Class}>One alert, not two hundred</h2>
              <p className="mt-4 max-w-xl text-pretty text-base text-base-500">
                Rules for deaths, OOM kills, health checks, and log patterns.
                Rate windows and cooldowns turn a crash loop into one message to
                ntfy, Gotify, Telegram, or a webhook.
              </p>
              <div className="mt-4 flex">
                <Link href="/docs/alerting" className={pill.accent}>
                  Set up alerts
                </Link>
              </div>
            </div>
          </div>
          <div
            aria-hidden="true"
            className="overflow-hidden rounded-xl bg-sand-100 p-8 pb-0 pl-0"
          >
            <AlertsCard />
          </div>
        </div>

        <div className="grid grid-cols-1 items-start gap-12 md:grid-cols-2">
          <div className="flex flex-col gap-4 text-balance">
            <div className="inline-flex w-fit rounded-lg bg-sand-100 p-4">
              <Icon icon={CpuSettingsIcon} className="text-accent-600" />
            </div>
            <div>
              <h2 className={h2Class}>Fix it from the same screen</h2>
              <p className="mt-4 max-w-xl text-pretty text-base text-base-500">
                Raise a memory limit live, edit env vars, restart a Compose
                stack, or open a shell in the container. No SSH into the host.
              </p>
              <div className="mt-4 flex">
                <Link href="/docs/features" className={pill.accent}>
                  See every feature
                </Link>
              </div>
            </div>
          </div>
          <div
            aria-hidden="true"
            className="overflow-hidden rounded-xl bg-sand-100 p-8 pr-0 pb-0"
          >
            <ControlCard />
          </div>
        </div>

        <div className="grid grid-cols-1 items-start gap-12 md:grid-cols-2">
          <div className="flex flex-col gap-4 text-balance md:order-last">
            <div className="inline-flex w-fit rounded-lg bg-sand-100 p-4">
              <Icon icon={McpServerIcon} className="text-accent-600" />
            </div>
            <div>
              <h2 className={h2Class}>Ask your agent what broke at 3am</h2>
              <p className="mt-4 max-w-xl text-pretty text-base text-base-500">
                Point Claude, Cursor, or any MCP client at LogDeck and ask what
                happened overnight. It can search history and pull stats, or act
                too with an admin token.
              </p>
              <div className="mt-4 flex">
                <Link href="/docs/mcp" className={pill.accent}>
                  Set up the MCP server
                </Link>
              </div>
            </div>
          </div>
          <div
            aria-hidden="true"
            className="overflow-hidden rounded-xl bg-sand-100 p-8 pb-0 pl-0"
          >
            <AgentCard />
          </div>
        </div>
      </Wrapper>
    </section>
  );
}
