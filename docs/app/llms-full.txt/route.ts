import { faqs } from "@/components/landing/faq";
import { docs, summary, toMarkdown } from "@/lib/docs";
import { siteUrl } from "@/lib/docs-nav";

export const dynamic = "force-static";

export function GET() {
  const faq = `# Frequently asked questions\n\n${faqs
    .map((item) => `## ${item.question}\n\n${item.answer}`)
    .join("\n\n")}`;

  const body = [
    `# LogDeck documentation\n\n> ${summary}\n\nEvery page of the docs at ${siteUrl}, in order. The index is ${siteUrl}/llms.txt.`,
    ...docs.map(toMarkdown),
    faq,
  ].join("\n\n---\n\n");

  return new Response(`${body}\n`, {
    headers: { "Content-Type": "text/plain; charset=utf-8" },
  });
}
