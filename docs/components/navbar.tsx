"use client";

import { Github } from "lucide-react";
import Link from "next/link";
import * as React from "react";

import { MobileSidebar } from "@/components/docs/mobile-sidebar";
import { ThemeToggle } from "@/components/theme-toggle";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export function Navbar() {
  const [scrolled, setScrolled] = React.useState(false);

  React.useEffect(() => {
    const handleScroll = () => {
      setScrolled(window.scrollY > 10);
    };
    window.addEventListener("scroll", handleScroll);
    return () => window.removeEventListener("scroll", handleScroll);
  }, []);

  return (
    <header
      className={cn(
        "sticky top-0 z-50 w-full border-b border-border/40 bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60 transition-all",
        scrolled && "shadow-sm"
      )}
    >
      <div className="container flex h-16 items-center justify-between">
        <div className="flex items-center gap-8">
          <Link href="/" className="flex items-center space-x-2">
            <div className="flex items-center gap-2">
              <span className="font-bold text-xl">LogDeck</span>
            </div>
          </Link>

          <nav className="hidden md:flex items-center gap-6">
            <Link
              href="/docs/getting-started"
              className="text-sm font-medium text-muted-foreground hover:text-foreground transition-colors"
            >
              Documentation
            </Link>
            <Link
              href="/#features"
              className="text-sm font-medium text-muted-foreground hover:text-foreground transition-colors"
            >
              Features
            </Link>
            <Link
              href="/#installation"
              className="text-sm font-medium text-muted-foreground hover:text-foreground transition-colors"
            >
              Installation
            </Link>
            <Link
              href="/demo"
              className="text-sm font-medium text-muted-foreground hover:text-foreground transition-colors"
            >
              Demo
            </Link>
          </nav>
        </div>

        <div className="flex items-center gap-2">
          <Button variant="ghost" size="sm" asChild className="md:hidden">
            <Link href="/demo">Demo</Link>
          </Button>
          <Button variant="ghost" size="sm" asChild>
            <a
              href="https://github.com/AmoabaKelvin/logdeck"
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-2"
            >
              <Github className="h-4 w-4" />
              <span className="hidden sm:inline">GitHub</span>
            </a>
          </Button>
          <ThemeToggle />
          <MobileSidebar />
        </div>
      </div>
    </header>
  );
}
