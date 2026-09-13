import type { MetadataRoute } from "next";

import { docsNav, siteUrl } from "@/lib/docs-nav";

export default function sitemap(): MetadataRoute.Sitemap {
  return [
    { url: siteUrl },
    { url: `${siteUrl}/demo` },
    { url: `${siteUrl}/blog` },
    { url: `${siteUrl}/blog/log-store`, lastModified: new Date("2026-09-13") },
    ...docsNav.flatMap((section) =>
      section.items.map((item) => ({
        url: `${siteUrl}${item.href}`,
        lastModified: item.updated,
      })),
    ),
  ];
}
