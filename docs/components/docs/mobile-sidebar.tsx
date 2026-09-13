"use client";

import { Menu01Icon } from "@hugeicons/core-free-icons";
import Link from "next/link";
import { useState } from "react";

import { SidebarNav } from "@/components/docs/sidebar-nav";
import { Icon } from "@/components/landing/ui";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";

export function MobileSidebar({
  links,
}: {
  links: { title: string; href: string }[];
}) {
  const [open, setOpen] = useState(false);
  const close = () => setOpen(false);

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger className="inline-flex size-9 items-center justify-center rounded-full text-base-700 hover:bg-sand-100 lg:hidden">
        <Icon icon={Menu01Icon} size={20} className="shrink-0" />
        <span className="sr-only">Open menu</span>
      </SheetTrigger>
      <SheetContent side="right" className="w-80 gap-0 bg-white font-display">
        <SheetHeader className="border-b border-dashed border-base-200 p-4">
          <SheetTitle className="font-display font-medium">Menu</SheetTitle>
        </SheetHeader>
        <div className="flex flex-col gap-8 overflow-y-auto p-4">
          <ul className="flex flex-col gap-0.5">
            {links.map((link) => (
              <li key={link.href}>
                <Link
                  href={link.href}
                  onClick={close}
                  className="flex rounded-full px-3 py-2 text-base/6 text-base-900 hover:bg-sand-100"
                >
                  {link.title}
                </Link>
              </li>
            ))}
          </ul>
          <SidebarNav onNavigate={close} />
        </div>
      </SheetContent>
    </Sheet>
  );
}
