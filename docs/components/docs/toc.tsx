"use client";

import { useEffect, useState } from "react";

import { cn } from "@/lib/utils";

// A heading counts as current once it scrolls up under the sticky header.
const headerOffset = 96;

export function Toc({
  headings,
}: {
  headings: { id: string; text: string }[];
}) {
  const [active, setActive] = useState(headings[0]?.id);

  useEffect(() => {
    function update() {
      let current = headings[0]?.id;
      for (const heading of headings) {
        const top = document
          .getElementById(heading.id)
          ?.getBoundingClientRect().top;
        if (top !== undefined && top <= headerOffset) current = heading.id;
      }
      // Short last sections never reach the top; light them up at the bottom.
      const atBottom =
        window.scrollY > 0 &&
        window.innerHeight + window.scrollY >=
          document.documentElement.scrollHeight - 2;
      setActive(atBottom ? headings[headings.length - 1]?.id : current);
    }

    update();
    window.addEventListener("scroll", update, { passive: true });
    return () => window.removeEventListener("scroll", update);
  }, [headings]);

  return (
    <nav aria-label="On this page" className="sticky top-16 py-14">
      <p className="font-mono text-xs tracking-wide text-base-500 uppercase">
        On this page
      </p>
      <ul className="mt-4 border-l border-dashed border-base-300 text-sm">
        {headings.map((heading) => {
          const current = heading.id === active;
          return (
            <li key={heading.id}>
              <a
                href={`#${heading.id}`}
                aria-current={current ? "location" : undefined}
                className={cn(
                  "relative block py-1.5 pl-4",
                  current
                    ? "text-base-900 before:absolute before:inset-y-0 before:-left-px before:w-px before:bg-accent-500"
                    : "text-base-500 hover:text-base-900",
                )}
              >
                {heading.text}
              </a>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
