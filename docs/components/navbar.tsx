"use client";

import { Github01Icon } from "@hugeicons/core-free-icons";
import Link from "next/link";
import { usePathname } from "next/navigation";

import { MobileSidebar } from "@/components/docs/mobile-sidebar";
import { Icon, pill } from "@/components/landing/ui";
import { cn } from "@/lib/utils";

const links = [
  { title: "Features", href: "/#features" },
  { title: "Install", href: "/#install" },
  { title: "Compare", href: "/compare/dozzle" },
  { title: "Docs", href: "/docs/getting-started" },
  { title: "Blog", href: "/blog" },
];

// The landing page sits in a narrower dashed frame than the docs.
export function frameClass(pathname: string) {
  return pathname === "/" || pathname.startsWith("/blog")
    ? "max-w-5xl 2xl:max-w-6xl 2xl:px-12"
    : "max-w-7xl";
}

export function Navbar() {
  const pathname = usePathname();

  return (
    <header className="sticky top-0 z-50 border-b border-dashed border-base-200 bg-white/90 backdrop-blur">
      <div
        className={cn(
          "mx-auto flex h-16 w-full items-center justify-between gap-4 border-x border-dashed border-base-200 px-4",
          frameClass(pathname),
        )}
      >
        <div className="flex items-center gap-8">
          <Link
            href="/"
            aria-label="Homepage"
            className="text-lg font-medium text-black"
          >
            LogDeck
          </Link>
          <nav className="flex items-center gap-6 max-md:hidden">
            {links.map((link) => {
              const section = link.href.split("/")[1];
              const active =
                !link.href.startsWith("/#") &&
                pathname.startsWith(`/${section}`);
              return (
                <Link
                  key={link.href}
                  href={link.href}
                  className={cn(
                    "text-sm",
                    active
                      ? "text-base-900"
                      : "text-base-500 hover:text-base-900",
                  )}
                >
                  {link.title}
                </Link>
              );
            })}
          </nav>
        </div>

        <div className="flex items-center gap-2">
          <a
            href="https://github.com/AmoabaKelvin/logdeck"
            target="_blank"
            rel="noopener noreferrer"
            className={cn(pill.muted, "h-8 gap-2 px-3 max-sm:px-2")}
          >
            <Icon icon={Github01Icon} size={16} className="shrink-0" />
            <span className="max-sm:sr-only">GitHub</span>
          </a>
          <Link href="/demo" className={cn(pill.accent, "h-8 px-3")}>
            Demo
          </Link>
          <MobileSidebar links={links} />
        </div>
      </div>
    </header>
  );
}
