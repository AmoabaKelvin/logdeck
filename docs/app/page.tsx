import { Cli } from "@/components/landing/cli"
import { Features } from "@/components/landing/features"
import { Hero } from "@/components/landing/hero"
import { Installation } from "@/components/landing/installation"
import { Screenshots } from "@/components/landing/screenshots"

export default function Home() {
  return (
    <div className="flex flex-col">
      <Hero />
      <Features />
      <Screenshots />
      <Cli />
      <Installation />
    </div>
  )
}
