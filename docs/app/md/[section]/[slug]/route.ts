import { docs, getDoc, toMarkdown } from "@/lib/docs";
import { siteUrl } from "@/lib/docs-nav";

type Context = { params: Promise<{ section: string; slug: string }> };

export const dynamicParams = false;

export function generateStaticParams() {
  return docs.map((doc) => {
    const [, section, slug] = doc.href.split("/");
    return { section, slug };
  });
}

export async function GET(_request: Request, { params }: Context) {
  const { section, slug } = await params;
  const doc = getDoc(`/${section}/${slug}`);
  if (!doc) return new Response("Not found", { status: 404 });

  return new Response(toMarkdown(doc), {
    headers: {
      "Content-Type": "text/markdown; charset=utf-8",
      Link: `<${siteUrl}${doc.href}>; rel="canonical"`,
      Vary: "Accept",
    },
  });
}
