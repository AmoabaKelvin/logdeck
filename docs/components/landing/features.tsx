import {
  ChartSplineIcon,
  ComputerTerminal02Icon,
  ConsoleIcon,
  KeyGeneratorFobIcon,
  PuzzleIcon,
  ServerStack02Icon,
  Structure01Icon,
  WaveSquareIcon,
} from "@hugeicons/core-free-icons";

import { Icon, Wrapper, h2Class } from "./ui";

const features = [
  {
    title: "Multi-host, no agents",
    description:
      "Local sockets, TCP, or SSH with key auth. One list, nothing to install remotely.",
    icon: ServerStack02Icon,
  },
  {
    title: "Docker and Podman, mixed",
    description:
      "Auto-detects Docker and rootless or rootful Podman. Mix both in one setup.",
    icon: PuzzleIcon,
  },
  {
    title: "Live tail that keeps up",
    description:
      "WebSocket tail with pause, timestamps, collapsible JSON, and line pinning.",
    icon: WaveSquareIcon,
  },
  {
    title: "Stats without a metrics stack",
    description:
      "CPU and memory per container with sparklines, plus engine and host stats.",
    icon: ChartSplineIcon,
  },
  {
    title: "A CLI on the same API",
    description:
      "grep every host, follow a service, restart a stack. JSON on every command.",
    icon: ComputerTerminal02Icon,
  },
  {
    title: "A shell in the browser",
    description:
      "Open a terminal in any running container, or run one command and get the exit code.",
    icon: ConsoleIcon,
  },
  {
    title: "Scoped tokens, read-only mode",
    description:
      "Admin and read tokens, plus a read-only switch for the instance everyone can see.",
    icon: KeyGeneratorFobIcon,
  },
  {
    title: "Images, volumes, networks",
    description:
      "Read-only views of everything else on every host, in one filterable list.",
    icon: Structure01Icon,
  },
];

export function Features() {
  return (
    <section>
      <Wrapper className="border-t border-dashed border-base-200 pt-12 pb-4">
        <div className="max-w-xl text-balance">
          <h2 className={h2Class}>Everything else, no extra containers</h2>
          <p className="mt-4 text-pretty text-base text-base-500">
            One Go binary with the frontend embedded. No database to run, no
            agents on your hosts, no cloud tier.
          </p>
        </div>
        <div className="mt-8 grid grid-cols-1 items-center gap-2 md:grid-cols-2 lg:grid-cols-4">
          {features.map((feature) => {
            return (
              <div
                key={feature.title}
                className="group flex h-full flex-col rounded-xl bg-sand-100 p-4 outline-transparent duration-300 hover:bg-white hover:shadow-2xl hover:outline hover:outline-sand-100"
              >
                <div className="inline-flex w-fit rounded-lg bg-white p-4 duration-300 group-hover:-translate-y-2 group-hover:-rotate-12 group-hover:bg-sand-100">
                  <Icon icon={feature.icon} className="text-accent-500" />
                </div>
                <h3 className="mt-12 text-sm font-medium text-base-900">
                  {feature.title}
                </h3>
                <p className="mt-2 text-pretty text-sm text-base-500">
                  {feature.description}
                </p>
              </div>
            );
          })}
        </div>
      </Wrapper>
    </section>
  );
}
