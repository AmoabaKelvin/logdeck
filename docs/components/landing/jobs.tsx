import {
  SatelliteDishIcon,
  Telescope01Icon,
  ToolboxIcon,
} from "@hugeicons/core-free-icons";

import { Icon, Wrapper, h2Class } from "./ui";

const jobs = [
  {
    title: "Watch",
    icon: Telescope01Icon,
    text: "Live tail and stored history for every container on every host, stats beside it.",
  },
  {
    title: "Get told",
    icon: SatelliteDishIcon,
    text: "Died, OOM-killed, unhealthy, or logging errors. One alert to ntfy, Telegram, or a webhook.",
  },
  {
    title: "Fix",
    icon: ToolboxIcon,
    text: "Raise a limit, edit env, restart the stack, open a shell. Browser, CLI, or your AI agent.",
  },
];

export function Jobs() {
  return (
    <section>
      <Wrapper className="border-t border-dashed border-base-200 pt-12 pb-12">
        <div className="max-w-xl text-balance">
          <h2 className={h2Class}>
            For everything after{" "}
            <code className="font-mono font-normal">docker compose up -d</code>
          </h2>
          <p className="mt-4 text-pretty text-base text-base-500">
            Dozzle for logs, Uptime Kuma for pings, ntfy glued on, a Grafana
            stack nobody finished. LogDeck does the three jobs that matter in
            one binary. Keep your deploy tool.
          </p>
        </div>
        <dl className="mt-10 grid grid-cols-1 sm:grid-cols-3">
          {jobs.map((job) => (
            <div
              key={job.title}
              className="border-dashed border-base-200 max-sm:not-first:mt-8 max-sm:not-first:border-t max-sm:not-first:pt-8 sm:not-first:border-l sm:not-first:pl-8 sm:not-last:pr-8"
            >
              <Icon icon={job.icon} className="text-accent-500" />
              <dt className="mt-4 font-medium text-base-900">{job.title}</dt>
              <dd className="mt-2 max-w-[40ch] text-pretty text-base/7 text-base-500 sm:text-sm/6">
                {job.text}
              </dd>
            </div>
          ))}
        </dl>
      </Wrapper>
    </section>
  );
}
