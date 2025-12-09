import { createRootRouteWithContext, Outlet } from "@tanstack/react-router";
import { lazy, Suspense } from "react";
import { NuqsAdapter } from "nuqs/adapters/tanstack-router";

import { Toaster } from "@/components/ui/sonner";
import { AuthProvider } from "@/contexts/auth-context";

import type { QueryClient } from "@tanstack/react-query";

const DevTools = lazy(() =>
  import("@/components/dev-tools").then((module) => ({
    default: module.DevTools,
  }))
);

interface MyRouterContext {
  queryClient: QueryClient;
}

export const Route = createRootRouteWithContext<MyRouterContext>()({
  component: () => (
    <AuthProvider>
      <NuqsAdapter>
        <Outlet />
      </NuqsAdapter>
      <Toaster />
      {import.meta.env.DEV && (
        <Suspense fallback={null}>
          <DevTools />
        </Suspense>
      )}
    </AuthProvider>
  ),
});
