import { BookOpen, Layers, Rocket, Settings } from "lucide-react"

export const docsNav = [
  {
    title: "Getting Started",
    items: [
      {
        title: "Introduction",
        href: "/docs/getting-started",
        icon: Rocket,
      },
    ],
  },
  {
    title: "Guides",
    items: [
      {
        title: "Installation",
        href: "/docs/installation",
        icon: BookOpen,
      },
      {
        title: "Features",
        href: "/docs/features",
        icon: Layers,
      },
      {
        title: "Configuration",
        href: "/docs/configuration",
        icon: Settings,
      },
    ],
  },
]
