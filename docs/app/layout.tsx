import type { Metadata } from "next";
import "./globals.css";

import { Geist, Geist_Mono } from "next/font/google";

import { Footer } from "@/components/footer";
import { Navbar } from "@/components/navbar";
import { siteUrl } from "@/lib/docs-nav";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

const title =
  "LogDeck: self-hosted logs, alerts, and control for Docker and Podman";
const description =
  "Free, open-source log viewer, alerting, and container control for Docker and Podman. Stored log history, alert rules with cooldowns, live limit edits, a CLI, and an MCP server.";

export const metadata: Metadata = {
  metadataBase: new URL(siteUrl),
  title: {
    default: title,
    template: "%s | LogDeck",
  },
  description,
  authors: [{ name: "Amoaba Kelvin", url: "https://github.com/AmoabaKelvin" }],
  creator: "Amoaba Kelvin",
  applicationName: "LogDeck",
  category: "developer tools",
  alternates: {
    canonical: "./",
  },
  // The share image comes from app/opengraph-image.png.
  openGraph: {
    type: "website",
    locale: "en_US",
    url: "/",
    siteName: "LogDeck",
    title,
    description,
  },
  twitter: {
    card: "summary_large_image",
    creator: "@amoabakelvin",
  },
  robots: {
    index: true,
    follow: true,
    googleBot: {
      index: true,
      follow: true,
      "max-video-preview": -1,
      "max-image-preview": "large",
      "max-snippet": -1,
    },
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <head>
        {/* Google Sans Flex is not in next/font yet. */}
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link
          rel="preconnect"
          href="https://fonts.gstatic.com"
          crossOrigin="anonymous"
        />
        <link
          rel="stylesheet"
          href="https://fonts.googleapis.com/css2?family=Google+Sans+Flex:opsz,wght@6..144,100..1000&display=swap"
        />
      </head>
      <body
        className={`${geistSans.variable} ${geistMono.variable} bg-white font-display text-base-900 antialiased`}
      >
        <div className="relative isolate flex min-h-dvh flex-col">
          <Navbar />
          <main className="flex-1">{children}</main>
          <Footer />
        </div>
        {process.env.NODE_ENV === "production" && (
          <script
            defer
            src="https://static.cloudflareinsights.com/beacon.min.js"
            data-cf-beacon='{"token": "64cf8b0762624db2ae501d86591a04b9"}'
          />
        )}
      </body>
    </html>
  );
}
