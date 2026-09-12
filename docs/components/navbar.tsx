"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

import { MobileSidebar } from "@/components/docs/mobile-sidebar";
import { Github01Icon } from "@hugeicons/core-free-icons";

import { Icon, pill } from "@/components/landing/ui";
import { ThemeToggle } from "@/components/theme-toggle";
import { cn } from "@/lib/utils";

const links = [
  { title: "Features", href: "/#features" },
  { title: "Install", href: "/#install" },
  { title: "Compare", href: "/#compare" },
  { title: "Docs", href: "/docs/getting-started" },
];

export function Navbar() {
  const isLanding = usePathname() === "/";

  return (
    <header className="sticky top-0 z-50 w-full border-b border-dashed border-base-200 bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div
        className={cn(
          "flex h-16 items-center justify-between",
          isLanding && "font-display",
          isLanding
            ? "mx-auto w-full max-w-5xl border-x border-dashed border-base-200 px-4 2xl:max-w-6xl 2xl:px-12"
            : "container",
        )}
      >
        <div className="flex items-center gap-8">
          <Link href="/" className="font-display text-lg font-medium">
            LogDeck
          </Link>
          <nav className="hidden items-center gap-4 md:flex">
            {links.map((link) => (
              <Link
                key={link.href}
                href={link.href}
                className="text-sm text-muted-foreground hover:text-foreground"
              >
                {link.title}
              </Link>
            ))}
          </nav>
        </div>

        <div className="flex items-center gap-2">
          <a
            href="https://github.com/AmoabaKelvin/logdeck"
            target="_blank"
            rel="noopener noreferrer"
            className={`${pill.muted} h-8 gap-2 max-sm:px-2.5`}
          >
            <Icon icon={Github01Icon} size={16} className="shrink-0" />
            <span className="max-sm:sr-only">GitHub</span>
          </a>
          <Link href="/demo" className={`${pill.accent} h-8`}>
            Demo
          </Link>
          {!isLanding && <ThemeToggle />}
          <MobileSidebar />
        </div>
      </div>
    </header>
  );
}
