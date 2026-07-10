import type { Metadata } from "next";
import { redirect } from "next/navigation";

export const metadata: Metadata = {
  title: "Demo",
  description:
    "Interactive LogDeck demo in your browser: explore container log streaming, stats, and management for Docker and Podman without installing anything.",
  alternates: { canonical: "/demo" },
};

export default function DemoPage() {
  redirect("/demo");
}
