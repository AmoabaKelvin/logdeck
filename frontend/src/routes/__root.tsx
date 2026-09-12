import type { QueryClient } from "@tanstack/react-query";
import { createRootRouteWithContext, Outlet } from "@tanstack/react-router";
import { ThemeProvider } from "next-themes";
import { NuqsAdapter } from "nuqs/adapters/tanstack-router";
import { lazy, Suspense } from "react";
import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { AuthProvider } from "@/contexts/auth-context";

const DevTools = lazy(() =>
	import("@/components/dev-tools").then((module) => ({
		default: module.DevTools,
	})),
);

interface MyRouterContext {
	queryClient: QueryClient;
}

export const Route = createRootRouteWithContext<MyRouterContext>()({
	component: () => (
		<ThemeProvider attribute="class" defaultTheme="system" enableSystem>
			<AuthProvider>
				<TooltipProvider delayDuration={300}>
					<NuqsAdapter>
						<Outlet />
					</NuqsAdapter>
				</TooltipProvider>
				<Toaster />
				{import.meta.env.DEV && (
					<Suspense fallback={null}>
						<DevTools />
					</Suspense>
				)}
			</AuthProvider>
		</ThemeProvider>
	),
});
