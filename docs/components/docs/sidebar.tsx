"use client";

import { BookOpen, ChevronRight, Layers, Rocket, Settings } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";

import { ScrollArea } from "@/components/ui/scroll-area";
import { cn } from "@/lib/utils";

const docsNav = [
  {
    title: "Getting Started",
    items: [
      {
        title: "Introduction",
        href: "/docs/getting-started",
        icon: Rocket,
      },
    ],
  },
  {
    title: "Guides",
    items: [
      {
        title: "Installation",
        href: "/docs/installation",
        icon: BookOpen,
      },
      {
        title: "Features",
        href: "/docs/features",
        icon: Layers,
      },
      {
        title: "Configuration",
        href: "/docs/configuration",
        icon: Settings,
      },
    ],
  },
];

export function DocsSidebar() {
  const pathname = usePathname();

  return (
    <aside className="fixed top-16 z-30 hidden h-[calc(100vh-4rem)] w-full shrink-0 md:sticky md:block border-r">
      <ScrollArea className="h-full py-6 pr-6 lg:py-8">
        <div className="w-full">
          {docsNav.map((section) => (
            <div key={section.title} className="pb-8">
              <h4 className="mb-1 rounded-md px-2 py-1 text-sm font-semibold">
                {section.title}
              </h4>
              <div className="grid grid-flow-row auto-rows-max text-sm">
                {section.items.map((item) => {
                  const Icon = item.icon;
                  const isActive = pathname === item.href;
                  return (
                    <Link
                      key={item.href}
                      href={item.href}
                      className={cn(
                        "group flex w-full items-center gap-2 rounded-md border border-transparent px-2 py-1.5 hover:bg-muted",
                        isActive && "bg-muted font-medium"
                      )}
                    >
                      <Icon className="h-4 w-4 text-muted-foreground" />
                      <span>{item.title}</span>
                      {isActive && <ChevronRight className="ml-auto h-4 w-4" />}
                    </Link>
                  );
                })}
              </div>
            </div>
          ))}
        </div>
      </ScrollArea>
    </aside>
  );
}
