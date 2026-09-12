"use client";

import Image from "next/image";
import Link from "next/link";
import { useEffect, useState } from "react";

import { Github01Icon } from "@hugeicons/core-free-icons";

import { Icon, Wrapper, h1Class, pill } from "./ui";

export function Hero() {
  const [stars, setStars] = useState<number | null>(null);

  useEffect(() => {
    fetch("https://api.github.com/repos/AmoabaKelvin/logdeck")
      .then((res) => res.json())
      .then((data) => {
        if (data.stargazers_count) setStars(data.stargazers_count);
      })
      .catch(() => {
        // Silently fail - stars badge won't show
      });
  }, []);

  return (
    <section>
      <Wrapper className="pt-16 pb-4 sm:pt-24">
        <div className="text-center text-balance">
          <h1 className={`${h1Class} mx-auto max-w-[30ch]`}>
            Logs, alerts, and control for every container you run
          </h1>
          <p className="mx-auto mt-4 max-w-xl text-pretty text-base text-base-500">
            One free binary for Docker and Podman. Keeps your logs, tells you
            once when it breaks, lets you fix it.
          </p>
        </div>
        <div className="mt-8 flex w-full flex-wrap items-center justify-center gap-2">
          <Link href="/demo" className={pill.accent}>
            Try the live demo
          </Link>
          <a href="#install" className={pill.muted}>
            Install in a minute
          </a>
          <a
            href="https://github.com/AmoabaKelvin/logdeck"
            target="_blank"
            rel="noopener noreferrer"
            className={`${pill.muted} gap-2`}
          >
            <Icon icon={Github01Icon} size={16} className="shrink-0" />
            Star on GitHub
            {stars !== null && (
              <span className="font-mono tabular-nums text-base-500">
                {stars.toLocaleString()}
              </span>
            )}
          </a>
        </div>
        <p className="mt-4 text-center text-sm text-base-500">
          Open source, self-hosted, no cloud tier. Keep your deploy tool.
        </p>
        <Image
          src="/dashboard.png"
          alt="LogDeck dashboard listing containers with health badges, ports, and CPU and memory sparklines"
          width={3850}
          height={2188}
          className="mt-8 h-auto w-full rounded-xl outline-1 -outline-offset-1 outline-sand-500/10"
          priority
        />
      </Wrapper>
    </section>
  );
}
