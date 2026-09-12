"use client";

import { DemoApp } from "@/components/logdeck-demo/demo-app";
import { Badge } from "@/components/logdeck-demo/ui/badge";

export function DemoPageClient() {
  return (
    <DemoApp
      className="min-h-dvh"
      banner={
        <div className="mb-4 flex items-center gap-2">
          <Badge variant="outline">Interactive Demo</Badge>
          <span className="text-xs text-muted-foreground">
            Running on simulated data — every feature is live.
          </span>
        </div>
      }
    />
  );
}
