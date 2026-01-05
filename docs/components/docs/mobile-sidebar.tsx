"use client"

import { BookOpen, ChevronRight, Layers, Menu, Rocket, Settings } from "lucide-react"
import Link from "next/link"
import { usePathname } from "next/navigation"
import { useState } from "react"

import { Button } from "@/components/ui/button"
import { ScrollArea } from "@/components/ui/scroll-area"
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet"
import { cn } from "@/lib/utils"

const docsNav = [
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

export function MobileSidebar() {
  const pathname = usePathname()
  const [open, setOpen] = useState(false)

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger asChild>
        <Button variant="ghost" size="icon" className="md:hidden">
          <Menu className="h-5 w-5" />
          <span className="sr-only">Toggle menu</span>
        </Button>
      </SheetTrigger>
      <SheetContent side="right" className="w-72 p-0">
        <SheetHeader className="border-b px-6 py-4">
          <SheetTitle>Documentation</SheetTitle>
        </SheetHeader>
        <ScrollArea className="h-[calc(100vh-5rem)] py-6 px-4">
          <div className="w-full">
            {docsNav.map((section) => (
              <div key={section.title} className="pb-8">
                <h4 className="mb-1 rounded-md px-2 py-1 text-sm font-semibold">
                  {section.title}
                </h4>
                <div className="grid grid-flow-row auto-rows-max text-sm">
                  {section.items.map((item) => {
                    const Icon = item.icon
                    const isActive = pathname === item.href
                    return (
                      <Link
                        key={item.href}
                        href={item.href}
                        onClick={() => setOpen(false)}
                        className={cn(
                          "group flex w-full items-center gap-2 rounded-md border border-transparent px-2 py-1.5 hover:bg-muted",
                          isActive && "bg-muted font-medium"
                        )}
                      >
                        <Icon className="h-4 w-4 text-muted-foreground" />
                        <span>{item.title}</span>
                        {isActive && <ChevronRight className="ml-auto h-4 w-4" />}
                      </Link>
                    )
                  })}
                </div>
              </div>
            ))}
          </div>
        </ScrollArea>
      </SheetContent>
    </Sheet>
  )
}
