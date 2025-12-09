import { createRootRouteWithContext, Outlet } from "@tanstack/react-router";
import { lazy } from "react";
import { NuqsAdapter } from "nuqs/adapters/tanstack-router";

import { Toaster } from "@/components/ui/sonner";
import { AuthProvider } from "@/contexts/auth-context";

import type { QueryClient } from "@tanstack/react-query";

const TanStackDevtools = import.meta.env.DEV
  ? lazy(() =>
      import("@tanstack/react-devtools").then((module) => ({
        default: module.TanStackDevtools,
      }))
    )
  : () => null;

const TanStackRouterDevtoolsPanel = import.meta.env.DEV
  ? lazy(() =>
      import("@tanstack/react-router-devtools").then((module) => ({
        default: module.TanStackRouterDevtoolsPanel,
      }))
    )
  : () => null;

const TanStackQueryDevtools = import.meta.env.DEV
  ? lazy(() => import("../integrations/tanstack-query/devtools"))
  : () => null;

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
        <TanStackDevtools
          config={{
            position: "bottom-right",
          }}
          plugins={[
            {
              name: "Tanstack Router",
              render: <TanStackRouterDevtoolsPanel />,
            },
            TanStackQueryDevtools,
          ]}
        />
      )}
    </AuthProvider>
  ),
});
