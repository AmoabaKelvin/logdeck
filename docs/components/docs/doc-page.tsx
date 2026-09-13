import Link from "next/link";

import { CopyButton } from "@/components/docs/copy-button";
import { Markdown } from "@/components/docs/markdown";
import { Toc } from "@/components/docs/toc";
import { JsonLd } from "@/components/json-ld";
import { h1Class, pill } from "@/components/landing/ui";
import { type Doc, docs, slugify, toMarkdown } from "@/lib/docs";
import { siteUrl } from "@/lib/docs-nav";
import { cn } from "@/lib/utils";

const dateFormat = new Intl.DateTimeFormat("en-US", {
  dateStyle: "medium",
  timeZone: "UTC",
});

export function DocPage({ doc }: { doc: Doc }) {
  const index = docs.indexOf(doc);
  const previous = docs[index - 1];
  const next = docs[index + 1];
  const headings = [...doc.body.matchAll(/^## (.+)$/gm)].map((match) => {
    const text = match[1].replace(/[`*]/g, "");
    return { id: slugify(text), text };
  });
  const url = `${siteUrl}${doc.href}`;

  return (
    <div className="xl:grid xl:grid-cols-[minmax(0,1fr)_13rem] xl:gap-12">
      <JsonLd
        data={[
          {
            "@context": "https://schema.org",
            "@type": "TechArticle",
            headline: doc.title,
            description: doc.description,
            url,
            dateModified: doc.updated,
            author: {
              "@type": "Person",
              name: "Amoaba Kelvin",
              url: "https://github.com/AmoabaKelvin",
            },
            about: { "@type": "SoftwareApplication", name: "LogDeck" },
          },
          {
            "@context": "https://schema.org",
            "@type": "BreadcrumbList",
            itemListElement: [
              { "@type": "ListItem", position: 1, name: "LogDeck", item: siteUrl },
              { "@type": "ListItem", position: 2, name: doc.title, item: url },
            ],
          },
        ]}
      />

      <article className="min-w-0 py-10 lg:py-14">
        <header>
          <p className="text-sm text-accent-600">{doc.section}</p>
          <h1 className={cn(h1Class, "mt-2 text-pretty")}>{doc.title}</h1>
          <p className="mt-4 max-w-[60ch] text-lg text-pretty text-base-500">
            {doc.description}
          </p>
          <div className="mt-6 flex flex-wrap items-center gap-2">
            <CopyButton
              text={toMarkdown(doc)}
              className={cn(pill.muted, "h-8 gap-2 pr-3 pl-2.5")}
            >
              Copy page
            </CopyButton>
            <a href={`${doc.href}.md`} className={cn(pill.muted, "h-8 px-3")}>
              View as Markdown
            </a>
            <p className="text-sm text-base-500 sm:ml-2">
              Updated{" "}
              <time dateTime={doc.updated}>
                {dateFormat.format(new Date(doc.updated))}
              </time>
            </p>
          </div>
        </header>

        <div className="mt-10 max-w-[72ch] border-t border-dashed border-base-200 pt-10">
          <Markdown>{doc.body}</Markdown>
        </div>

        <nav
          aria-label="Pagination"
          className="mt-16 grid max-w-[72ch] gap-2 sm:grid-cols-2"
        >
          {previous && (
            <Link
              href={previous.href}
              className="rounded-xl bg-sand-100 p-4 hover:bg-sand-200/60"
            >
              <p className="text-sm text-base-500">Previous</p>
              <p className="mt-1 text-base-900">{previous.title}</p>
            </Link>
          )}
          {next && (
            <Link
              href={next.href}
              className="rounded-xl bg-sand-100 p-4 text-right hover:bg-sand-200/60 sm:col-start-2"
            >
              <p className="text-sm text-base-500">Next</p>
              <p className="mt-1 text-base-900">{next.title}</p>
            </Link>
          )}
        </nav>

        <p className="mt-8 text-sm text-base-500">
          Spot a mistake?{" "}
          <a
            href={`https://github.com/AmoabaKelvin/logdeck/blob/main/docs/content${doc.href}.md`}
            target="_blank"
            rel="noopener noreferrer"
            className="text-base-900 hover:text-accent-500"
          >
            Edit this page on GitHub
          </a>
          .
        </p>
      </article>

      {headings.length > 0 && (
        <aside className="max-xl:hidden">
          <Toc headings={headings} />
        </aside>
      )}
    </div>
  );
}
