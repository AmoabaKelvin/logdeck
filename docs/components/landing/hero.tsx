"use client";

import { Github } from "lucide-react";
import Image from "next/image";
import { useEffect, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

export function Hero() {
  const [stars, setStars] = useState<number | null>(null);

  useEffect(() => {
    fetch("https://api.github.com/repos/AmoabaKelvin/logdeck")
      .then((res) => res.json())
      .then((data) => {
        if (data.stargazers_count) {
          setStars(data.stargazers_count);
        }
      })
      .catch(() => {
        // Silently fail - stars badge won't show
      });
  }, []);

  return (
    <section className="border-b">
      <div className="container flex flex-col items-center py-20 text-center sm:py-24">
        <h1 className="max-w-3xl text-balance text-4xl font-semibold tracking-tight sm:text-5xl md:text-6xl">
          Logs, stats, and control for all your containers
        </h1>
        <p className="mt-5 max-w-xl text-pretty text-base text-muted-foreground sm:text-lg">
          LogDeck is an open-source dashboard and CLI for Docker and Podman.
          Stream logs, watch resource usage, and manage containers across every
          host you run.
        </p>

        <div className="mt-8 flex flex-wrap items-center justify-center gap-3">
          <Button size="lg" asChild>
            <a href="/demo">Try the demo</a>
          </Button>
          <Button size="lg" variant="outline" asChild>
            <a href="#installation">Install</a>
          </Button>
        </div>

        <a
          href="https://github.com/AmoabaKelvin/logdeck"
          target="_blank"
          rel="noopener noreferrer"
          className="mt-6 inline-flex items-center gap-2 text-sm text-muted-foreground transition-colors hover:text-foreground"
        >
          <Github className="h-4 w-4" />
          Star on GitHub
          {stars !== null && (
            <Badge variant="secondary" className="font-mono text-xs">
              {stars.toLocaleString()}
            </Badge>
          )}
        </a>

        <div className="mt-14 w-full max-w-5xl overflow-hidden rounded-xl border shadow-sm">
          <Image
            src="/new-landing.png"
            alt="LogDeck dashboard showing containers across hosts with live CPU and memory sparklines, compose stack groups, and quick actions"
            width={4396}
            height={2894}
            className="h-auto w-full"
            priority
          />
        </div>
      </div>
    </section>
  );
}
