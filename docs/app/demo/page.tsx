import type { Metadata } from "next";
import { DemoPageClient } from "@/components/logdeck-demo/demo-page-client";

const description =
  "Interactive LogDeck demo in your browser: explore container log streaming, stats, and management for Docker and Podman without installing anything.";

export const metadata: Metadata = {
  title: "Demo",
  description,
  alternates: { canonical: "/demo" },
  openGraph: {
    siteName: "LogDeck",
    images: [{ url: "/opengraph-image.png", width: 1200, height: 630 }],
    url: "/demo",
    title: "LogDeck demo",
    description,
  },
};

export default function DemoPage() {
  return <DemoPageClient />;
}
