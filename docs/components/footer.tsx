"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

import { cn } from "@/lib/utils";

const footerLinks = [
  {
    heading: "Documentation",
    links: [
      { label: "Getting started", href: "/docs/getting-started" },
      { label: "Installation", href: "/docs/installation" },
      { label: "Features", href: "/docs/features" },
      { label: "Log history", href: "/docs/log-history" },
      { label: "Alerting", href: "/docs/alerting" },
      { label: "CLI", href: "/docs/cli" },
      { label: "MCP server", href: "/docs/mcp" },
      { label: "Configuration", href: "/docs/configuration" },
    ],
  },
  {
    heading: "Project",
    links: [
      { label: "GitHub", href: "https://github.com/AmoabaKelvin/logdeck" },
      {
        label: "Docker Hub",
        href: "https://hub.docker.com/r/amoabakelvin/logdeck",
      },
      {
        label: "Releases",
        href: "https://github.com/AmoabaKelvin/logdeck/releases",
      },
      {
        label: "Report an issue",
        href: "https://github.com/AmoabaKelvin/logdeck/issues",
      },
      {
        label: "License (GPL-3.0)",
        href: "https://github.com/AmoabaKelvin/logdeck/blob/main/LICENSE",
      },
    ],
  },
  {
    heading: "Try it",
    links: [
      { label: "Live demo", href: "/demo" },
      { label: "Compare with Dozzle", href: "/#compare" },
    ],
  },
];

export function Footer() {
  const isLanding = usePathname() === "/";

  return (
    <footer className="border-t border-dashed border-base-200 bg-background">
      <div
        className={cn(
          "py-12",
          isLanding && "font-display",
          isLanding
            ? "mx-auto w-full max-w-5xl border-x border-dashed border-base-200 px-4 2xl:max-w-6xl 2xl:px-12"
            : "container",
        )}
      >
        <div className="xl:grid xl:grid-cols-3 xl:gap-8">
          <div>
            <Link href="/" className="font-display text-lg font-medium">
              LogDeck
            </Link>
            <p className="mt-1 max-w-xs text-pretty text-sm text-muted-foreground">
              Self-hosted logs, alerts, and control for Docker and Podman. Built
              by{" "}
              <a
                href="https://github.com/AmoabaKelvin"
                target="_blank"
                rel="noopener noreferrer"
                className="text-foreground hover:text-accent-500"
              >
                Amoaba Kelvin
              </a>
              .
            </p>
          </div>
          <div className="md:grid md:grid-cols-3 md:gap-8 xl:col-span-2">
            {footerLinks.map((section) => (
              <div key={section.heading} className="mt-12 md:mt-0">
                <h3 className="text-sm font-medium">{section.heading}</h3>
                <ul className="mt-4 space-y-2 text-sm text-muted-foreground">
                  {section.links.map((link) => (
                    <li key={link.href}>
                      <a
                        href={link.href}
                        className="hover:text-foreground"
                        {...(link.href.startsWith("http")
                          ? { target: "_blank", rel: "noopener noreferrer" }
                          : {})}
                      >
                        {link.label}
                      </a>
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </div>
        </div>
      </div>
    </footer>
  );
}
