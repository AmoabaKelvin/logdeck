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
    default: "LogDeck - Logs, stats, and control for your containers",
    template: "%s | LogDeck",
  },
  description:
    "Open-source dashboard and CLI for Docker and Podman. Stream logs, watch resource usage, and manage containers across every host you run.",
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
    title: "LogDeck - Logs, stats, and control for your containers",
    description:
      "Open-source dashboard and CLI for Docker and Podman. Stream logs, watch resource usage, and manage containers across every host you run.",
    siteName: "LogDeck",
    images: [
      {
        url: "/new-landing.png",
        width: 4396,
        height: 2894,
        alt: "LogDeck dashboard showing container logs, stats, and controls",
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: "LogDeck - Logs, stats, and control for your containers",
    description:
      "Open-source dashboard and CLI for Docker and Podman. Stream logs, watch resource usage, and manage containers across every host you run.",
    creator: "@amoabakelvin",
    images: ["/new-landing.png"],
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
      "Open-source dashboard and CLI for Docker and Podman. Stream logs, watch resource usage, and manage containers across every host you run.",
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
    screenshot: "https://logdeck.dev/new-landing.png",
  },
];

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body className={`${geistSans.className} ${geistMono.variable} antialiased`}>
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
      </body>
    </html>
  );
}
