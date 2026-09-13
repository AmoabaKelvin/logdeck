import { Wrapper, h2Class } from "./ui";

// Plain text only: llms-full.txt reuses these answers as markdown.
export const faqs = [
  {
    question: "Is LogDeck free?",
    answer:
      "Yes. LogDeck is open source under GPL-3.0 and runs on your own server. There is no paid tier, no cloud version, and no account to create.",
  },
  {
    question: "Does LogDeck keep logs after a container is removed?",
    answer:
      "Yes. LogDeck stores every container's logs in a SQLite database on its own disk, so history survives restarts, rebuilds, and removal, up to the retention caps you set (50 MB per container and 1024 MB in total by default). Mount /data as a volume, or the store is lost when LogDeck itself is recreated.",
  },
  {
    question: "Does it work with Podman?",
    answer:
      "Yes. LogDeck talks to Podman through its Docker-compatible API socket, rootless or rootful, and one setup can mix Docker and Podman hosts. Health badges are the one gap: they only show for Docker containers.",
  },
  {
    question: "Do I need to install anything on remote hosts?",
    answer:
      "No. The one LogDeck container connects to remote engines over TCP or SSH. Nothing extra runs on the other hosts.",
  },
  {
    question: "How is LogDeck different from Dozzle?",
    answer:
      "Dozzle is a live log viewer. LogDeck also stores log history, sends alerts, edits environment variables and resource limits, runs Compose stack actions, and has a CLI and an MCP server that can act. Dozzle has multiple users, SSO, split-screen logs, and Swarm and Kubernetes support, which LogDeck does not.",
  },
  {
    question: "Can an AI agent use LogDeck?",
    answer:
      "Yes. Run logdeck mcp and point Claude, Cursor, or any MCP client at it. With a read-scoped API token the agent can search logs, history, and stats. An admin token also lets it restart containers, run commands, and edit environment variables.",
  },
  {
    question: "Where do alerts go?",
    answer:
      "To any webhook (Slack and Discord incoming webhooks work as-is), ntfy, Gotify, or Telegram. Rules cover container deaths with a non-zero exit code, OOM kills, failing health checks, and log patterns, with rate windows and cooldowns.",
  },
  {
    question: "Does LogDeck support multiple users or SSO?",
    answer:
      "No. LogDeck has one admin login. Scripts, agents, and teammates get their own read or admin API tokens, and read-only mode locks down an instance everyone can see.",
  },
];

export function Faq() {
  return (
    <section id="faq">
      <Wrapper className="border-t border-dashed border-base-200 pt-12 pb-12">
        <h2 className={`${h2Class} max-w-[40ch] text-balance`}>
          Questions before you install
        </h2>
        <dl className="mt-8 grid grid-cols-1 gap-x-12 md:grid-cols-2">
          {faqs.map((faq) => (
            <div
              key={faq.question}
              className="border-t border-dashed border-base-200 py-6"
            >
              <dt className="font-medium text-base-900">{faq.question}</dt>
              <dd className="mt-2 text-base/7 text-pretty text-base-500 sm:text-sm/6">
                {faq.answer}
              </dd>
            </div>
          ))}
        </dl>
      </Wrapper>
    </section>
  );
}
