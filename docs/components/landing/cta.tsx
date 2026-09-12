import Link from "next/link";

import { Wrapper, pill } from "./ui";

export function Cta() {
  return (
    <section className="border-t border-dashed border-base-200">
      <Wrapper className="py-4">
        <div className="rounded-xl bg-accent-500 p-8 py-24 text-center">
          <div className="text-balance">
            <h2 className="font-display text-3xl font-medium text-white md:text-4xl lg:text-5xl">
              Your logs, your disk, your call.
            </h2>
            <p className="mx-auto mt-2 max-w-xl text-pretty text-base text-white/80 sm:text-lg">
              Free, open source, one binary. Runs next to your containers on
              Docker or Podman.
            </p>
          </div>
          <div className="mt-8 flex flex-wrap justify-center gap-2">
            <Link href="/demo" className={pill.muted}>
              Try the live demo
            </Link>
            <a href="#install" className={`${pill.black}`}>
              Install in a minute
            </a>
          </div>
        </div>
      </Wrapper>
    </section>
  );
}
