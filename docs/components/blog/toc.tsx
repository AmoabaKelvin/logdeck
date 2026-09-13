"use client";

import { useEffect, useState } from "react";

import { cn } from "@/lib/utils";

export type TocItem = { id: string; title: string };

// Sticky table of contents for the right column. The active item is the last
// heading whose top has scrolled past the navbar.
export function Toc({ items }: { items: TocItem[] }) {
  const [active, setActive] = useState(items[0]?.id);

  useEffect(() => {
    const headings = items
      .map((item) => document.getElementById(item.id))
      .filter((el): el is HTMLElement => el !== null);
    let frame = 0;
    const update = () => {
      frame = 0;
      let current = headings[0]?.id;
      for (const el of headings) {
        if (el.getBoundingClientRect().top <= 120) current = el.id;
      }
      setActive(current);
    };
    const onScroll = () => {
      if (!frame) frame = requestAnimationFrame(update);
    };
    update();
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => {
      window.removeEventListener("scroll", onScroll);
      if (frame) cancelAnimationFrame(frame);
    };
  }, [items]);

  return (
    <nav aria-label="On this page" className="sticky top-24">
      <p className="font-mono text-[11px] tracking-[0.1em] text-base-500">
        ON THIS PAGE
      </p>
      <ol className="mt-3 border-l border-dashed border-base-200">
        {items.map((item) => {
          const isActive = item.id === active;
          return (
            <li key={item.id}>
              <a
                href={`#${item.id}`}
                className={cn(
                  "-ml-px block border-l py-1 pl-4 text-sm transition-colors",
                  isActive
                    ? "border-accent-500 text-base-900"
                    : "border-transparent text-base-500 hover:text-base-900",
                )}
              >
                {item.title}
              </a>
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
