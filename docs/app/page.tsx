import { Hero } from "@/components/landing/hero"
import { Features } from "@/components/landing/features"
import { Screenshots } from "@/components/landing/screenshots"
import { Installation } from "@/components/landing/installation"

export default function Home() {
  return (
    <div className="flex flex-col">
      <Hero />
      <Features />
      <Screenshots />
      <Installation />
    </div>
  )
}
