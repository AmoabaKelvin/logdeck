import { Wrapper, h2Class } from "./ui";

const rows = [
  ["Stored log history", "On disk, searchable, survives rebuild and removal", "None: live only, history via Dozzle Cloud"],
  ["Search", "Regex, level, and time range on the server, across history", "Regex on the live view, SQL over JSON logs in the browser"],
  ["Alerts", "Event and log rules, rate windows, cooldowns, delivery history", "Event, log, and metric rules with an expression language"],
  ["Alert channels", "Webhook (Slack and Discord as-is), ntfy, Gotify, Telegram", "Webhook, Slack, Discord, ntfy; Telegram and email via Cloud"],
  ["Edit env vars and resource limits", "Yes, with .env import and Coolify sync", "No"],
  ["Compose stacks", "Start, stop, restart a whole stack; merged logs", "Merged group logs"],
  ["Multi-host", "Local, TCP, or SSH; no agents", "TCP, or an agent container on each host"],
  ["Web terminal", "Yes", "Yes"],
  ["Split-screen log view", "No", "Yes"],
  ["Swarm and Kubernetes", "No", "Yes"],
  ["CLI and MCP", "logdeck CLI on the server API; MCP with read and action tools", "No API CLI; read-only MCP"],
  ["Users and auth", "Single admin, scoped API tokens, read-only mode", "Multiple users, OIDC, forward proxy, per-user roles"],
  ["Hosted tier", "None", "Dozzle Cloud (paid tiers)"],
  ["License", "GPL-3.0", "MIT"],
];

export function Comparison() {
  return (
    <section id="compare">
      <Wrapper className="border-t border-dashed border-base-200 pt-12 pb-4">
        <div className="max-w-xl text-balance">
          <h2 className={h2Class}>How it compares to Dozzle</h2>
          <p className="mt-4 text-pretty text-base text-base-500">
            Dozzle is the live log viewer most people try first. Here’s where
            LogDeck differs, and where it doesn’t.
          </p>
        </div>
        <div className="-mx-4 mt-8 overflow-x-auto 2xl:-mx-12">
          <table className="w-full min-w-[44rem] text-left text-sm">
            <thead>
              <tr className="border-y border-dashed border-base-200 text-base-500">
                <th className="w-1/4 px-4 py-3 font-medium 2xl:px-12">
                  <span className="sr-only">Feature</span>
                </th>
                <th className="whitespace-nowrap py-3 pr-4 font-medium text-base-900">
                  LogDeck
                </th>
                <th className="whitespace-nowrap py-3 pr-4 font-medium 2xl:pr-12">
                  Dozzle
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-dashed divide-base-200">
              {rows.map(([feature, ours, theirs]) => (
                <tr key={feature}>
                  <th scope="row" className="px-4 py-3 align-top font-medium text-base-900 2xl:px-12">
                    {feature}
                  </th>
                  <td className="py-3 pr-4 align-top text-base-800">{ours}</td>
                  <td className="py-3 pr-4 align-top text-base-500 2xl:pr-12">{theirs}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <p className="mt-4 max-w-xl text-pretty text-sm text-base-500">
          Checked against the Dozzle v11 docs, September 2026. Spot something
          stale?{" "}
          <a
            href="https://github.com/AmoabaKelvin/logdeck/issues"
            target="_blank"
            rel="noopener noreferrer"
            className="text-base-900 hover:text-accent-500"
          >
            Open an issue
          </a>
          .
        </p>
      </Wrapper>
    </section>
  );
}
