"use client"

import { ChevronRight } from "lucide-react"
import Link from "next/link"
import { usePathname } from "next/navigation"

import { docsNav } from "@/lib/docs-nav"
import { cn } from "@/lib/utils"

interface SidebarNavProps {
  onNavigate?: () => void
}

export function SidebarNav({ onNavigate }: SidebarNavProps) {
  const pathname = usePathname()

  return (
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
                  onClick={onNavigate}
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
  )
}
