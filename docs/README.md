# logdeck.dev

The LogDeck landing page and docs. Next.js 16 and Tailwind CSS v4, deployed to Cloudflare Workers with OpenNext.

```bash
bun install
bun run dev      # http://localhost:3000
bun run build
bun run preview  # the Workers build, locally
```

## Layout

```
app/
├── page.tsx                   # Landing page
├── (docs)/[section]/[slug]/   # Every docs page and /compare/dozzle
├── md/[section]/[slug]/       # The same pages as markdown (/docs/cli.md)
├── llms.txt/, llms-full.txt/  # Agent indexes, built from the docs
├── sitemap.ts, robots.ts
└── opengraph-image.png        # Share image for every page
content/
├── docs/*.md                  # Docs page bodies
└── compare/dozzle.md          # Also feeds the landing page comparison table
lib/
├── docs-nav.ts                # Page list: titles, descriptions, dates, order
└── docs.ts                    # Loads the markdown
components/
├── landing/                   # Landing page sections
└── docs/                      # Docs shell, markdown renderer, code blocks
```

## Adding a docs page

1. Write `content/docs/<slug>.md`. No front matter and no H1; start with an intro paragraph and use `##` and `###` headings.
2. Add an entry to `lib/docs-nav.ts`.
3. Import the file in `lib/docs.ts`.

The sidebar, sitemap, llms.txt, llms-full.txt, and the `.md` URL all pick it up from there. When you edit a page, bump its `updated` date in `lib/docs-nav.ts`.
