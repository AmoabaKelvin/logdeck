import type { Metadata } from "next";
import "./globals.css";

import { Vend_Sans } from "next/font/google";

import { Footer } from "@/components/footer";
import { Navbar } from "@/components/navbar";
import { ThemeProvider } from "@/components/theme-provider";

const vendSans = Vend_Sans({
  variable: "--font-vend-sans",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  metadataBase: new URL("https://logdeck.dev"),
  title: {
    default: "LogDeck - Beautiful Docker Container Logs & Management",
    template: "%s | LogDeck",
  },
  description:
    "The most intuitive and visually appealing tool for monitoring Docker container logs and managing containers. Open-source, lightweight, and easy to use.",
  keywords: [
    "docker",
    "container",
    "logs",
    "log viewer",
    "docker logs",
    "container management",
    "docker ui",
    "open source",
    "docker compose",
    "log monitoring",
  ],
  authors: [{ name: "Amoaba Kelvin", url: "https://github.com/AmoabaKelvin" }],
  creator: "Amoaba Kelvin",
  openGraph: {
    type: "website",
    locale: "en_US",
    url: "https://logdeck.dev",
    title: "LogDeck - Beautiful Docker Container Logs & Management",
    description:
      "The most intuitive and visually appealing tool for monitoring Docker container logs and managing containers.",
    siteName: "LogDeck",
    images: [
      {
        url: "/landing.png",
        width: 1920,
        height: 1080,
        alt: "LogDeck - Beautiful Docker Container Dashboard",
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: "LogDeck - Beautiful Docker Container Logs & Management",
    description:
      "The most intuitive and visually appealing tool for monitoring Docker container logs and managing containers.",
    creator: "@amoabakelvin",
    images: ["/landing.png"],
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
    <html lang="en" suppressHydrationWarning>
      <body className={`${vendSans.className} antialiased`}>
        <ThemeProvider
          attribute="class"
          defaultTheme="system"
          enableSystem
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
