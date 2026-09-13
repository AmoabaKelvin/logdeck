"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

import { frameClass } from "@/components/navbar";
import { docsNav } from "@/lib/docs-nav";
import { cn } from "@/lib/utils";

const footerLinks = [
  {
    heading: "Docs",
    links: docsNav
      .flatMap((section) => section.items)
      .filter((item) => item.href.startsWith("/docs"))
      .map((item) => ({ label: item.title, href: item.href })),
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
      { label: "LogDeck vs Dozzle", href: "/compare/dozzle" },
    ],
  },
  {
    heading: "For agents",
    links: [
      { label: "llms.txt", href: "/llms.txt" },
      { label: "llms-full.txt", href: "/llms-full.txt" },
      { label: "Docs as Markdown", href: "/docs/getting-started.md" },
      { label: "MCP server", href: "/docs/mcp" },
    ],
  },
];

export function Footer() {
  const pathname = usePathname();

  return (
    <footer className="border-t border-dashed border-base-200">
      <div
        className={cn(
          "mx-auto grid w-full gap-12 border-x border-dashed border-base-200 px-4 py-12 xl:grid-cols-3",
          frameClass(pathname),
        )}
      >
        <div>
          <Link href="/" className="text-lg font-medium text-black">
            LogDeck
          </Link>
          <p className="mt-2 max-w-xs text-base/7 text-pretty text-base-500 sm:text-sm/6">
            Self-hosted logs, alerts, and control for Docker and Podman. Built
            by{" "}
            <a
              href="https://github.com/AmoabaKelvin"
              target="_blank"
              rel="noopener noreferrer"
              className="text-base-900 hover:text-accent-500"
            >
              Amoaba Kelvin
            </a>
            .
          </p>
        </div>
        <div className="grid grid-cols-2 gap-8 sm:grid-cols-4 xl:col-span-2">
          {footerLinks.map((section) => (
            <div key={section.heading}>
              <h3 className="text-sm font-medium text-base-900">
                {section.heading}
              </h3>
              <ul className="mt-4 flex flex-col gap-2">
                {section.links.map((link) => (
                  <li key={link.href} className="text-base/7 sm:text-sm/6">
                    <a
                      href={link.href}
                      className="text-base-500 hover:text-base-900"
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
    </footer>
  );
}
