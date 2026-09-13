import { JsonLd } from "@/components/json-ld";
import { Comparison } from "@/components/landing/comparison";
import { Cta } from "@/components/landing/cta";
import { Faq } from "@/components/landing/faq";
import { Features } from "@/components/landing/features";
import { Hero } from "@/components/landing/hero";
import { Install } from "@/components/landing/install";
import { Jobs } from "@/components/landing/jobs";
import { Pillars } from "@/components/landing/pillars";
import { WorksWith } from "@/components/landing/works-with";
import { summary } from "@/lib/docs";
import { siteUrl } from "@/lib/docs-nav";

const sameAs = [
  "https://github.com/AmoabaKelvin/logdeck",
  "https://hub.docker.com/r/amoabakelvin/logdeck",
];

const structuredData = [
  {
    "@context": "https://schema.org",
    "@type": "WebSite",
    name: "LogDeck",
    url: siteUrl,
  },
  {
    "@context": "https://schema.org",
    "@type": "Organization",
    name: "LogDeck",
    url: siteUrl,
    sameAs,
  },
  {
    "@context": "https://schema.org",
    "@type": "SoftwareApplication",
    name: "LogDeck",
    description: summary,
    url: siteUrl,
    applicationCategory: "DeveloperApplication",
    operatingSystem: "Linux, macOS",
    license: "https://www.gnu.org/licenses/gpl-3.0.html",
    isAccessibleForFree: true,
    offers: { "@type": "Offer", price: "0", priceCurrency: "USD" },
    downloadUrl: "https://hub.docker.com/r/amoabakelvin/logdeck",
    softwareHelp: {
      "@type": "CreativeWork",
      url: `${siteUrl}/docs/getting-started`,
    },
    screenshot: `${siteUrl}/dashboard.png`,
    author: {
      "@type": "Person",
      name: "Amoaba Kelvin",
      url: "https://github.com/AmoabaKelvin",
    },
    sameAs,
  },
];

export default function Home() {
  return (
    <div className="selection:bg-sand-100 selection:text-accent-500">
      <JsonLd data={structuredData} />
      <Hero />
      <WorksWith />
      <Jobs />
      <Pillars />
      <Install />
      <Features />
      <Comparison />
      <Faq />
      <Cta />
    </div>
  );
}
