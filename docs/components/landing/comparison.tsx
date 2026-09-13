import Link from "next/link";

import dozzle from "@/content/compare/dozzle.md";

import { Wrapper, h2Class, pill } from "./ui";

// The table lives in content/compare/dozzle.md, so this section, the compare
// page, and its markdown copy never disagree. Keep its cells plain text.
const rows = dozzle
  .split("\n")
  .filter((line) => line.startsWith("|"))
  .slice(2)
  .map((line) =>
    line
      .split("|")
      .slice(1, -1)
      .map((cell) => cell.trim()),
  );

export function Comparison() {
  return (
    <section id="compare">
      <Wrapper className="border-t border-dashed border-base-200 pt-12 pb-4">
        <div className="max-w-xl text-balance">
          <h2 className={h2Class}>How it compares to Dozzle</h2>
          <p className="mt-4 text-pretty text-base text-base-500">
            Dozzle is the live log viewer most people try first. Here’s where
            LogDeck differs, and where it doesn’t.
          </p>
        </div>
        <div className="-mx-4 mt-8 overflow-x-auto 2xl:-mx-12">
          <table className="w-full min-w-[44rem] text-left text-sm">
            <thead>
              <tr className="border-y border-dashed border-base-200 text-base-500">
                <th className="w-1/4 px-4 py-3 font-medium 2xl:px-12">
                  <span className="sr-only">Feature</span>
                </th>
                <th className="whitespace-nowrap py-3 pr-4 font-medium text-base-900">
                  LogDeck
                </th>
                <th className="whitespace-nowrap py-3 pr-4 font-medium 2xl:pr-12">
                  Dozzle
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-dashed divide-base-200">
              {rows.map(([feature, ours, theirs]) => (
                <tr key={feature}>
                  <th
                    scope="row"
                    className="px-4 py-3 align-top font-medium text-base-900 2xl:px-12"
                  >
                    {feature}
                  </th>
                  <td className="py-3 pr-4 align-top text-base-800">{ours}</td>
                  <td className="py-3 pr-4 align-top text-base-500 2xl:pr-12">
                    {theirs}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <div className="mt-6 flex flex-wrap items-center gap-x-6 gap-y-4">
          <Link href="/compare/dozzle" className={pill.accent}>
            Read the full comparison
          </Link>
          <p className="max-w-xl text-pretty text-sm text-base-500">
            Checked against the Dozzle v11 docs, September 2026. Spot something
            stale?{" "}
            <a
              href="https://github.com/AmoabaKelvin/logdeck/issues"
              target="_blank"
              rel="noopener noreferrer"
              className="text-base-900 hover:text-accent-500"
            >
              Open an issue
            </a>
            .
          </p>
        </div>
      </Wrapper>
    </section>
  );
}
