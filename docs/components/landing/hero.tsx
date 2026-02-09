"use client";

import { motion } from "framer-motion";
import { Github, Star } from "lucide-react";
import { useEffect, useState } from "react";

import HeroBadge from "@/components/hero-badge";
import { CodeBlock } from "@/components/landing/code-block";
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
    <section className="relative overflow-hidden border-b bg-background">
      {/* Grid pattern background */}
      <div className="absolute inset-0 -z-10 h-full w-full bg-background bg-[linear-gradient(to_right,#8080800a_1px,transparent_1px),linear-gradient(to_bottom,#8080800a_1px,transparent_1px)] bg-[size:14px_24px]" />

      {/* Gradient orbs */}
      <div className="absolute top-0 right-0 -z-10 h-96 w-96 rounded-full bg-blue-500/10 blur-3xl" />
      <div className="absolute bottom-0 left-0 -z-10 h-96 w-96 rounded-full bg-purple-500/10 blur-3xl" />

      <div className="container relative">
        <div className="flex min-h-[calc(100vh-4rem)] flex-col items-center justify-center py-12 sm:py-20 text-center px-4">
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.5 }}
            className="flex flex-col items-center gap-6 w-full max-w-4xl"
          >
            {/* New badge */}
            <HeroBadge
              text="Open Source Docker Container Management"
              icon={<Star className="h-3.5 w-3.5 fill-current" />}
              variant="outline"
              size="md"
            />

            {/* Main headline */}
            <motion.h1
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: 0.2, duration: 0.5 }}
              className="text-3xl font-bold tracking-tight sm:text-4xl md:text-5xl lg:text-6xl xl:text-7xl px-4"
            >
              The better way to{" "}
              <span className="text-primary">manage your containers</span>
            </motion.h1>

            {/* Subheading */}
            <motion.p
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: 0.3, duration: 0.5 }}
              className="text-base text-muted-foreground max-w-2xl sm:text-lg md:text-xl lg:text-2xl px-4"
            >
              The most intuitive and visually appealing tool for monitoring
              Docker container logs and managing containers.
            </motion.p>

            {/* CTA Buttons */}
            <motion.div
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: 0.4, duration: 0.5 }}
              className="flex flex-col items-center gap-4 mt-4"
            >
              <div className="flex flex-col sm:flex-row gap-3">
                <Button size="lg" className="text-base h-12 px-8" asChild>
                  <a href="/demo">Try Live Demo</a>
                </Button>
                <Button
                  size="lg"
                  variant="outline"
                  className="gap-2 text-base h-12 px-8"
                  asChild
                >
                  <a href="#installation">Get Started</a>
                </Button>
              </div>
              <a
                href="https://github.com/AmoabaKelvin/logdeck"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
              >
                <Github className="h-4 w-4" />
                {stars !== null ? (
                  <>
                    Star on GitHub
                    <Badge
                      variant="secondary"
                      className="font-mono text-xs"
                    >
                      {stars.toLocaleString()}
                    </Badge>
                  </>
                ) : (
                  "View on GitHub"
                )}
              </a>
            </motion.div>

            {/* Quick install command */}
            <motion.div
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: 0.5, duration: 0.5 }}
              className="mt-8 w-full max-w-2xl px-4"
            >
              <div className="w-full overflow-hidden">
                <CodeBlock
                  code="docker run -d -p 8123:8080 -v /var/run/docker.sock:/var/run/docker.sock amoabakelvin/logdeck"
                  language="bash"
                />
              </div>
              <p className="mt-3 text-xs text-muted-foreground">
                Logs viewing and container management shouldn&apos;t be that
                hard.
              </p>
            </motion.div>
          </motion.div>
        </div>
      </div>
    </section>
  );
}
