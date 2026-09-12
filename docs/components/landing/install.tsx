"use client";

import {
  CommandLineIcon,
  ContainerIcon,
  FileScriptIcon,
  McpServerIcon,
} from "@hugeicons/core-free-icons";
import Link from "next/link";
import { useState } from "react";

import { Icon, Wrapper, h2Class } from "./ui";

const tabs = [
  {
    id: "compose",
    label: "Docker Compose",
    icon: FileScriptIcon,
    code: `services:
  logdeck:
    image: amoabakelvin/logdeck:latest
    ports:
      - "8123:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /proc:/host/proc:ro
      - logdeck-data:/data   # config, log history, alert history
    environment:
      # DOCKER_HOSTS: local=unix:///var/run/docker.sock,prod=ssh://deploy@prod.example.com
      # ADMIN_USERNAME: admin
      # ADMIN_PASSWORD: your-bcrypt-hash
      # JWT_SECRET: your-super-secret-key-min-32-chars
    restart: unless-stopped

volumes:
  logdeck-data:

# then: docker compose up -d  →  http://localhost:8123`,
  },
  {
    id: "docker",
    label: "Docker run",
    icon: ContainerIcon,
    code: `docker run -d \\
  --name logdeck \\
  -p 8123:8080 \\
  -v /var/run/docker.sock:/var/run/docker.sock \\
  -v /proc:/host/proc:ro \\
  -v logdeck-data:/data \\
  amoabakelvin/logdeck:latest

# Podman: point the socket at /run/podman/podman.sock instead.
# Your containers are already listed at http://localhost:8123`,
  },
  {
    id: "cli",
    label: "CLI",
    icon: CommandLineIcon,
    code: `curl -fsSL https://raw.githubusercontent.com/AmoabaKelvin/logdeck/main/install.sh | sh

logdeck login --url https://logdeck.example.com --token ldk_... --name vps

# search the last hour across every host
logdeck grep "connection refused" --since 1h

# follow one service, errors only
logdeck logs api --follow --level ERROR

# raise a limit, then confirm it took
logdeck resources set api --memory 1g
logdeck resources api -o json`,
  },
  {
    id: "mcp",
    label: "MCP",
    icon: McpServerIcon,
    code: `{
  "mcpServers": {
    "logdeck": {
      "command": "logdeck",
      "args": ["mcp"],
      "env": {
        "LOGDECK_URL": "https://logdeck.example.com",
        "LOGDECK_TOKEN": "ldk_your_read_token"
      }
    }
  }
}

// A read token can list, read logs, search history, and read stats.
// Hand it an admin token and it can restart, exec, and edit env too.`,
  },
];

export function Install() {
  const [active, setActive] = useState(tabs[0].id);
  const current = tabs.find((t) => t.id === active) ?? tabs[0];

  return (
    <section id="install" className="overflow-hidden">
      <Wrapper className="border-t border-dashed border-base-200 pt-12 pb-4">
        <div className="text-center text-balance">
          <h2 className={h2Class}>Up and running in a minute</h2>
          <p className="mx-auto mt-4 max-w-xl text-pretty text-base text-base-500">
            One container for the server, one command for the CLI, one JSON
            block for your agent. Mount a volume or lose the history on
            recreate.
          </p>
        </div>

        <div className="mt-12 flex w-full items-center justify-center gap-6 py-2 sm:gap-12">
          {tabs.map((tab) => {
            const isActive = tab.id === active;
            return (
              <button
                key={tab.id}
                type="button"
                onClick={() => setActive(tab.id)}
                aria-pressed={isActive}
                className="flex cursor-pointer flex-col items-center gap-2"
              >
                <span
                  className={`rounded-xl bg-sand-100 p-3 ${isActive ? "opacity-100" : "opacity-50 hover:opacity-100"}`}
                >
                  <Icon icon={tab.icon} className="text-accent-600" />
                </span>
                <span className="text-xs text-base-500">{tab.label}</span>
              </button>
            );
          })}
        </div>

        <div className="-mx-4 mt-8 border-t border-dashed border-base-200 2xl:-mx-12">
          <div className="flex items-center gap-1 bg-sand-100 p-4">
            <span className="size-2 rounded-full bg-[#ff421e]" />
            <span className="size-2 rounded-full bg-[#60beff]" />
            <span className="size-2 rounded-full bg-[#e3962d]" />
          </div>
          <pre className="overflow-x-auto p-4 font-mono text-xs/5 text-base-600 sm:text-sm/6">
            {current.code}
          </pre>
        </div>
        <dl className="mx-auto mt-10 grid max-w-3xl gap-6 border-t border-dashed border-base-200 pt-8 text-sm sm:grid-cols-3">
          <div>
            <dt className="font-medium text-base-900">Not a deploy tool</dt>
            <dd className="mt-1 text-pretty text-base-500">
              Keep Dockge, Komodo, or plain Compose for shipping. LogDeck picks
              up after up -d.
            </dd>
          </div>
          <div>
            <dt className="font-medium text-base-900">One admin login</dt>
            <dd className="mt-1 text-pretty text-base-500">
              No multi-user or SSO. Hand out read or admin tokens to scripts,
              agents, and teammates.
            </dd>
          </div>
          <div>
            <dt className="font-medium text-base-900">
              Logs live on this disk
            </dt>
            <dd className="mt-1 text-pretty text-base-500">
              Stored on the LogDeck host, capped by the retention you set.
              Health badges are Docker-only.
            </dd>
          </div>
        </dl>
        <p className="mt-6 text-center text-sm text-base-500">
          Auth, more hosts, retention caps? See the{" "}
          <Link
            href="/docs/configuration"
            className="text-base-900 hover:text-accent-500"
          >
            configuration guide
          </Link>
          .
        </p>
      </Wrapper>
    </section>
  );
}
