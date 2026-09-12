import { Comparison } from "@/components/landing/comparison";
import { Cta } from "@/components/landing/cta";
import { Features } from "@/components/landing/features";
import { ForceLight } from "@/components/landing/force-light";
import { Hero } from "@/components/landing/hero";
import { Install } from "@/components/landing/install";
import { Jobs } from "@/components/landing/jobs";
import { Pillars } from "@/components/landing/pillars";
import { WorksWith } from "@/components/landing/works-with";

export default function Home() {
  return (
    <div className="bg-white font-display text-base-900 selection:bg-sand-100 selection:text-accent-500">
      <ForceLight />
      <Hero />
      <WorksWith />
      <Jobs />
      <Pillars />
      <Install />
      <Features />
      <Comparison />
      <Cta />
    </div>
  );
}
