import { Wrapper } from "./ui";

const items = [
  "Docker",
  "Podman",
  "Docker Compose",
  "podman-compose",
  "SSH hosts",
  "Coolify",
  "ntfy",
  "Gotify",
  "Telegram",
  "Slack",
  "Discord",
  "Claude",
  "Cursor",
  "Any MCP client",
];

export function WorksWith() {
  return (
    <section>
      <Wrapper className="border-t border-dashed border-base-200 py-6">
        <p className="text-center text-sm text-base-500">
          Works with what you already run.
        </p>
        <div className="mt-4 flex overflow-hidden [mask-image:linear-gradient(to_right,transparent,black_10%,black_90%,transparent)]">
          {[0, 1].map((copy) => (
            <ul
              key={copy}
              aria-hidden={copy === 1}
              className="flex shrink-0 animate-marquee items-center gap-3 whitespace-nowrap pr-3"
            >
              {items.map((item) => (
                <li
                  key={item}
                  className="rounded-full bg-sand-100 px-4 py-1.5 text-sm text-sand-950"
                >
                  {item}
                </li>
              ))}
            </ul>
          ))}
        </div>
      </Wrapper>
    </section>
  );
}
