import type { Metadata } from "next";
import { notFound } from "next/navigation";

import { DocPage } from "@/components/docs/doc-page";
import { docs, getDoc } from "@/lib/docs";

type Props = { params: Promise<{ section: string; slug: string }> };

export function generateStaticParams() {
  return docs.map((doc) => {
    const [, section, slug] = doc.href.split("/");
    return { section, slug };
  });
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { section, slug } = await params;
  const doc = getDoc(`/${section}/${slug}`);
  if (!doc) return {};

  return {
    title: doc.title,
    description: doc.description,
    alternates: {
      canonical: doc.href,
      types: { "text/markdown": `${doc.href}.md` },
    },
    // Replaces the layout's openGraph object whole, including the share
    // image from app/opengraph-image.png, so repeat both.
    openGraph: {
      type: "article",
      siteName: "LogDeck",
      images: [{ url: "/opengraph-image.png", width: 1200, height: 630 }],
      url: doc.href,
      title: doc.title,
      description: doc.description,
      modifiedTime: doc.updated,
    },
  };
}

export default async function Page({ params }: Props) {
  const { section, slug } = await params;
  const doc = getDoc(`/${section}/${slug}`);
  if (!doc) notFound();

  return <DocPage doc={doc} />;
}
