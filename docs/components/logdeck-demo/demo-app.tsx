"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { type ReactNode, useState } from "react";

import { AppHeader } from "@/components/logdeck-demo/app-header";
import { ContainersDashboard } from "@/components/logdeck-demo/containers/containers-dashboard";
import {
  DemoThemeContext,
  type DemoView,
  DemoViewContext,
  PortalContainerContext,
} from "@/components/logdeck-demo/demo-view";
import { ImagesPage } from "@/components/logdeck-demo/resources/images-page";
import { NetworksPage } from "@/components/logdeck-demo/resources/networks-page";
import { VolumesPage } from "@/components/logdeck-demo/resources/volumes-page";
import { Toaster } from "@/components/logdeck-demo/ui/sonner";
import { cn } from "@/lib/utils";

import "@/app/logdeck-demo.css";

// The whole in-browser app: header, page switch, theme, and overlays, all
// scoped to the root element so it can be a full page (/demo) or a framed
// embed (landing hero) without leaking into the site around it.
export function DemoApp({
  className,
  banner,
}: {
  className?: string;
  banner?: ReactNode;
}) {
  const [queryClient] = useState(() => new QueryClient());
  const [view, setView] = useState<DemoView>("containers");
  const [theme, setTheme] = useState("light");
  const [root, setRoot] = useState<HTMLElement | null>(null);

  return (
    <QueryClientProvider client={queryClient}>
      <DemoViewContext.Provider value={{ view, setView }}>
        <DemoThemeContext.Provider value={{ theme, setTheme }}>
          <PortalContainerContext.Provider value={root}>
            <div
              ref={setRoot}
              className={cn(
                "logdeck-demo-root isolate bg-background",
                theme === "dark" && "dark",
                className,
              )}
            >
              <AppHeader />
              <main className="app-width px-4 py-8 sm:px-6 lg:px-8">
                {banner}
                {view === "containers" && <ContainersDashboard />}
                {view === "images" && <ImagesPage />}
                {view === "volumes" && <VolumesPage />}
                {view === "networks" && <NetworksPage />}
              </main>
              <Toaster />
            </div>
          </PortalContainerContext.Provider>
        </DemoThemeContext.Provider>
      </DemoViewContext.Provider>
    </QueryClientProvider>
  );
}
