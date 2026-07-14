import {
  BellRing,
  BookOpen,
  Database,
  Layers,
  Rocket,
  Settings,
  Terminal,
} from "lucide-react";

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
        title: "Log History",
        href: "/docs/log-history",
        icon: Database,
      },
      {
        title: "Alerting",
        href: "/docs/alerting",
        icon: BellRing,
      },
      {
        title: "CLI",
        href: "/docs/cli",
        icon: Terminal,
      },
      {
        title: "Configuration",
        href: "/docs/configuration",
        icon: Settings,
      },
    ],
  },
];
