import { Github } from "lucide-react";
import Link from "next/link";

import { Separator } from "@/components/ui/separator";

export function Footer() {
  return (
    <footer className="border-t bg-background">
      <div className="container py-12 md:py-16">
        <div className="grid grid-cols-1 md:grid-cols-4 gap-8">
          {/* Brand */}
          <div className="space-y-3">
            <div className="flex items-center gap-2">
              <span className="font-bold text-lg">LogDeck</span>
            </div>
            <p className="text-sm text-muted-foreground">
              The most intuitive tool for monitoring Docker container logs and
              management.
            </p>
          </div>

          {/* Documentation */}
          <div className="space-y-3">
            <h3 className="font-semibold text-sm">Documentation</h3>
            <ul className="space-y-2 text-sm text-muted-foreground">
              <li>
                <Link
                  href="/docs/getting-started"
                  className="hover:text-foreground transition-colors"
                >
                  Getting Started
                </Link>
              </li>
              <li>
                <Link
                  href="/docs/installation"
                  className="hover:text-foreground transition-colors"
                >
                  Installation
                </Link>
              </li>
              <li>
                <Link
                  href="/docs/features"
                  className="hover:text-foreground transition-colors"
                >
                  Features
                </Link>
              </li>
              <li>
                <Link
                  href="/docs/configuration"
                  className="hover:text-foreground transition-colors"
                >
                  Configuration
                </Link>
              </li>
            </ul>
          </div>

          {/* Resources */}
          <div className="space-y-3">
            <h3 className="font-semibold text-sm">Resources</h3>
            <ul className="space-y-2 text-sm text-muted-foreground">
              <li>
                <a
                  href="https://github.com/AmoabaKelvin/logdeck"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="hover:text-foreground transition-colors flex items-center gap-1"
                >
                  <Github className="h-3 w-3" />
                  GitHub Repository
                </a>
              </li>
              <li>
                <a
                  href="https://hub.docker.com/r/amoabakelvin/logdeck"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="hover:text-foreground transition-colors"
                >
                  Docker Hub
                </a>
              </li>
              <li>
                <a
                  href="https://github.com/AmoabaKelvin/logdeck/issues"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="hover:text-foreground transition-colors"
                >
                  Report an Issue
                </a>
              </li>
            </ul>
          </div>

          {/* Legal */}
          <div className="space-y-3">
            <h3 className="font-semibold text-sm">Project</h3>
            <ul className="space-y-2 text-sm text-muted-foreground">
              <li>
                <a
                  href="https://github.com/AmoabaKelvin/logdeck/blob/main/LICENSE"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="hover:text-foreground transition-colors"
                >
                  License (GPL-3.0)
                </a>
              </li>
              <li>
                <a
                  href="https://github.com/AmoabaKelvin/logdeck/releases"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="hover:text-foreground transition-colors"
                >
                  Releases
                </a>
              </li>
            </ul>
          </div>
        </div>

        <Separator className="my-8" />

        <div className="flex flex-col md:flex-row justify-between items-center gap-4 text-sm text-muted-foreground">
          <p>
            Built by{" "}
            <a
              href="https://github.com/AmoabaKelvin"
              target="_blank"
              rel="noopener noreferrer"
              className="font-medium hover:text-foreground transition-colors"
            >
              Amoaba Kelvin
            </a>
          </p>
          <p>Open source software licensed under GPL-3.0</p>
        </div>
      </div>
    </footer>
  );
}
