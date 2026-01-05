"use client"

import { ScrollArea } from "@/components/ui/scroll-area"
import { SidebarNav } from "./sidebar-nav"

export function DocsSidebar() {
  return (
    <aside className="fixed top-16 z-30 hidden h-[calc(100vh-4rem)] w-full shrink-0 md:sticky md:block border-r">
      <ScrollArea className="h-full py-6 pr-6 lg:py-8">
        <SidebarNav />
      </ScrollArea>
    </aside>
  )
}
