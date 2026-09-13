import type { MetadataRoute } from "next";

import { siteUrl } from "@/lib/docs-nav";

// Every crawler is welcome, AI ones included. /md is the internal target of
// the .md rewrites; crawlers should use the public /docs/*.md URLs.
export default function robots(): MetadataRoute.Robots {
  return {
    rules: {
      userAgent: "*",
      allow: "/",
      disallow: "/md/",
    },
    sitemap: `${siteUrl}/sitemap.xml`,
  };
}
