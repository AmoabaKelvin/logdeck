"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

import { docsNav } from "@/lib/docs-nav";
import { cn } from "@/lib/utils";

export function SidebarNav({ onNavigate }: { onNavigate?: () => void }) {
  const pathname = usePathname();

  return (
    <nav aria-label="Documentation" className="flex flex-col gap-8">
      {docsNav.map((section) => (
        <div key={section.title}>
          <p className="px-3 text-sm font-medium text-base-900">
            {section.title}
          </p>
          <ul className="mt-2 flex flex-col gap-0.5">
            {section.items.map((item) => {
              const active = pathname === item.href;
              return (
                <li key={item.href}>
                  <Link
                    href={item.href}
                    onClick={onNavigate}
                    aria-current={active ? "page" : undefined}
                    className={cn(
                      "flex rounded-full px-3 py-2 text-base/6 sm:py-1.5 sm:text-sm/6",
                      active
                        ? "bg-sand-100 text-accent-600"
                        : "text-base-500 hover:text-base-900",
                    )}
                  >
                    {item.title}
                  </Link>
                </li>
              );
            })}
          </ul>
        </div>
      ))}
    </nav>
  );
}
