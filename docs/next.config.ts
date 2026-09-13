import path from "node:path";

import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  turbopack: {
    root: __dirname,
    rules: {
      "*.md": {
        loaders: [path.join(__dirname, "lib/md-loader.js")],
        as: "*.js",
      },
    },
  },
  async redirects() {
    return [
      {
        source: "/:path*",
        has: [{ type: "host", value: "www.logdeck.dev" }],
        destination: "https://logdeck.dev/:path*",
        permanent: true,
      },
      { source: "/docs", destination: "/docs/getting-started", permanent: true },
      { source: "/docs/demo", destination: "/demo", permanent: true },
    ];
  },
  async rewrites() {
    // Agents get a page as markdown from /docs/cli.md, or from /docs/cli with
    // `Accept: text/markdown`. Both land on app/md.
    return {
      beforeFiles: [
        {
          source: "/:section(docs|compare)/:slug.md",
          destination: "/md/:section/:slug",
        },
        {
          source: "/:section(docs|compare)/:slug",
          has: [
            { type: "header", key: "accept", value: "(.*)text/markdown(.*)" },
          ],
          destination: "/md/:section/:slug",
        },
      ],
      afterFiles: [],
      fallback: [],
    };
  },
  async headers() {
    return [
      {
        source: "/:section(docs|compare)/:slug",
        headers: [{ key: "Vary", value: "Accept" }],
      },
    ];
  },
};

export default nextConfig;

import { initOpenNextCloudflareForDev } from "@opennextjs/cloudflare";
initOpenNextCloudflareForDev();
