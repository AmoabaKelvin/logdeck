import type { Metadata } from "next";
import "./globals.css";

import { Geist, Geist_Mono } from "next/font/google";

import { Footer } from "@/components/footer";
import { Navbar } from "@/components/navbar";
import { ThemeProvider } from "@/components/theme-provider";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  metadataBase: new URL("https://logdeck.dev"),
  title: {
    default: "LogDeck – Logs, alerts, and control for every container you run",
    template: "%s | LogDeck",
  },
  description:
    "Free, self-hosted logs, alerts, and control for Docker and Podman. Stored history, alert rules with cooldowns, live limit edits, a CLI, and an MCP server.",
  keywords: [
    "docker",
    "podman",
    "container",
    "logs",
    "log viewer",
    "docker logs",
    "container management",
    "docker ui",
    "docker cli",
    "open source",
    "docker compose",
    "log monitoring",
    "multi-host",
    "dozzle alternative",
    "log history",
    "container alerts",
    "mcp server",
  ],
  authors: [{ name: "Amoaba Kelvin", url: "https://github.com/AmoabaKelvin" }],
  creator: "Amoaba Kelvin",
  applicationName: "LogDeck",
  category: "developer tools",
  alternates: {
    canonical: "./",
  },
  openGraph: {
    type: "website",
    locale: "en_US",
    url: "https://logdeck.dev",
    title: "LogDeck – Logs, alerts, and control for every container you run",
    description:
      "Free, self-hosted logs, alerts, and control for Docker and Podman. Stored history, alert rules with cooldowns, live limit edits, a CLI, and an MCP server.",
    siteName: "LogDeck",
    images: [
      {
        url: "/dashboard.png",
        width: 3850,
        height: 2188,
        alt: "LogDeck containers dashboard with health, ports, and live CPU and memory sparklines",
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: "LogDeck – Logs, alerts, and control for every container you run",
    description:
      "Free, self-hosted logs, alerts, and control for Docker and Podman. Stored history, alert rules with cooldowns, live limit edits, a CLI, and an MCP server.",
    creator: "@amoabakelvin",
    images: ["/dashboard.png"],
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

const structuredData = [
  {
    "@context": "https://schema.org",
    "@type": "WebSite",
    name: "LogDeck",
    url: "https://logdeck.dev",
  },
  {
    "@context": "https://schema.org",
    "@type": "SoftwareApplication",
    name: "LogDeck",
    description:
      "Free, self-hosted logs, alerts, and control for Docker and Podman. Stored history, alert rules with cooldowns, live limit edits, a CLI, and an MCP server.",
    url: "https://logdeck.dev",
    applicationCategory: "DeveloperApplication",
    operatingSystem: "Linux, macOS",
    license: "https://www.gnu.org/licenses/gpl-3.0.html",
    offers: {
      "@type": "Offer",
      price: "0",
      priceCurrency: "USD",
    },
    author: {
      "@type": "Person",
      name: "Amoaba Kelvin",
      url: "https://github.com/AmoabaKelvin",
    },
    sameAs: [
      "https://github.com/AmoabaKelvin/logdeck",
      "https://hub.docker.com/r/amoabakelvin/logdeck",
    ],
    screenshot: "https://logdeck.dev/dashboard.png",
  },
];

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        {/* Google Sans Flex is not in next/font yet. Used for the landing page. */}
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
        className={`${geistSans.className} ${geistSans.variable} ${geistMono.variable} antialiased`}
      >
        <script
          type="application/ld+json"
          // biome-ignore lint/security/noDangerouslySetInnerHtml: static JSON-LD literal defined above, no user input
          dangerouslySetInnerHTML={{ __html: JSON.stringify(structuredData) }}
        />
        <ThemeProvider
          attribute="class"
          defaultTheme="light"
          disableTransitionOnChange
        >
          <div className="relative flex min-h-screen flex-col">
            <Navbar />
            <main className="flex-1">{children}</main>
            <Footer />
          </div>
        </ThemeProvider>
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
