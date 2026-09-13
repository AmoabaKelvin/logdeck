import dozzle from "@/content/compare/dozzle.md";
import alerting from "@/content/docs/alerting.md";
import cli from "@/content/docs/cli.md";
import configuration from "@/content/docs/configuration.md";
import features from "@/content/docs/features.md";
import gettingStarted from "@/content/docs/getting-started.md";
import installation from "@/content/docs/installation.md";
import logHistory from "@/content/docs/log-history.md";
import mcp from "@/content/docs/mcp.md";
import { docsNav, siteUrl } from "@/lib/docs-nav";

// The one-sentence definition. Reuse it wherever LogDeck gets described.
export const summary =
  "LogDeck is a free, open-source (GPL-3.0), self-hosted log viewer, alerting tool, and control panel for Docker and Podman containers.";

const bodies = new Map([
  ["/docs/getting-started", gettingStarted],
  ["/docs/installation", installation],
  ["/docs/features", features],
  ["/docs/log-history", logHistory],
  ["/docs/alerting", alerting],
  ["/docs/cli", cli],
  ["/docs/mcp", mcp],
  ["/docs/configuration", configuration],
  ["/compare/dozzle", dozzle],
]);

export const docs = docsNav.flatMap((section) =>
  section.items.map((item) => {
    const body = bodies.get(item.href);
    if (body === undefined) {
      throw new Error(`No markdown imported for ${item.href} in lib/docs.ts`);
    }
    return { ...item, section: section.title, body };
  }),
);

export type Doc = (typeof docs)[number];

export function getDoc(href: string) {
  return docs.find((doc) => doc.href === href);
}

// A page as agents get it: title and summary first, links made absolute.
export function toMarkdown(doc: Doc) {
  const body = doc.body.replaceAll("](/", `](${siteUrl}/`);
  return `# ${doc.title}\n\n> ${doc.description}\n\n${body}`;
}

// Heading anchors. The markdown renderer and the table of contents both use
// this, so links like /docs/cli#alerts keep working.
export function slugify(text: string) {
  return text
    .toLowerCase()
    .replace(/[^a-z0-9_ -]/g, "")
    .trim()
    .replace(/[\s-]+/g, "-");
}
