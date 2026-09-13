import type { Metadata } from "next";
import Link from "next/link";

import { ForceLight } from "@/components/landing/force-light";
import { Wrapper, h1Class } from "@/components/landing/ui";

export const metadata: Metadata = {
  title: "Blog",
  description: "Engineering notes from building LogDeck.",
  alternates: { canonical: "/blog" },
};

const posts = [
  {
    href: "/blog/log-store",
    date: "13 September 2026",
    title: "Storing 10× more container logs in the same SQLite file",
    summary:
      "One row per line cost more disk than the logs themselves. Sealing every 1,000 lines into a columnar zstd block fixed that. Most of the work went into the invariants around it.",
  },
];

export default function BlogIndex() {
  return (
    <div className="bg-white font-display text-base-900 selection:bg-sand-100 selection:text-accent-500">
      <ForceLight />
      <Wrapper className="pt-16 pb-4 sm:pt-24">
        <h1 className={h1Class}>Engineering notes</h1>
        <p className="mt-4 max-w-xl text-pretty text-base text-base-500">
          How LogDeck is built, with the numbers.
        </p>
      </Wrapper>
      <Wrapper className="border-t border-dashed border-base-200 pt-8 pb-24">
        <ul className="divide-y divide-dashed divide-base-200">
          {posts.map((post) => (
            <li key={post.href} className="py-8">
              <p className="font-mono text-xs text-base-500">{post.date}</p>
              <Link
                href={post.href}
                className="mt-2 block font-display text-lg font-medium tracking-tight text-black hover:text-accent-500 md:text-xl"
              >
                {post.title}
              </Link>
              <p className="mt-2 max-w-2xl text-pretty text-base text-base-500">
                {post.summary}
              </p>
            </li>
          ))}
        </ul>
      </Wrapper>
    </div>
  );
}
