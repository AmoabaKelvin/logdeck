"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import { ContainersDashboard } from "@/components/logdeck-demo/containers/containers-dashboard";
import { Badge } from "@/components/logdeck-demo/ui/badge";
import { Toaster } from "@/components/logdeck-demo/ui/sonner";

import "@/app/logdeck-demo.css";

export function DemoPageClient() {
  const [queryClient] = useState(() => new QueryClient());

  useEffect(() => {
    document.body.classList.add("logdeck-demo-page");
    return () => {
      document.body.classList.remove("logdeck-demo-page");
    };
  }, []);

  return (
    <QueryClientProvider client={queryClient}>
      <section className="logdeck-demo-root container mx-auto px-4 py-8">
        <div className="mb-4 flex items-center gap-2">
          <Badge variant="outline">Interactive Demo</Badge>
        </div>
        <ContainersDashboard />
      </section>
      <Toaster />
    </QueryClientProvider>
  );
}
