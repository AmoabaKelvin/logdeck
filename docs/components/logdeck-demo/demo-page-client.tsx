"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import { ContainersDashboard } from "@/components/logdeck-demo/containers/containers-dashboard";
import { type DemoView, DemoViewContext } from "@/components/logdeck-demo/demo-view";
import { ImagesPage } from "@/components/logdeck-demo/resources/images-page";
import { NetworksPage } from "@/components/logdeck-demo/resources/networks-page";
import { VolumesPage } from "@/components/logdeck-demo/resources/volumes-page";
import { Badge } from "@/components/logdeck-demo/ui/badge";
import { Toaster } from "@/components/logdeck-demo/ui/sonner";

import "@/app/logdeck-demo.css";

export function DemoPageClient() {
	const [queryClient] = useState(() => new QueryClient());
	const [view, setView] = useState<DemoView>("containers");

	useEffect(() => {
		document.body.classList.add("logdeck-demo-page");
		return () => {
			document.body.classList.remove("logdeck-demo-page");
		};
	}, []);

	return (
		<QueryClientProvider client={queryClient}>
			<DemoViewContext.Provider value={{ view, setView }}>
				<section className="logdeck-demo-root container mx-auto px-4 py-8">
					<div className="mb-4 flex items-center gap-2">
						<Badge variant="outline">Interactive Demo</Badge>
						<span className="text-xs text-muted-foreground">
							Running on simulated data — every feature is live.
						</span>
					</div>
					{view === "containers" && <ContainersDashboard />}
					{view === "images" && <ImagesPage />}
					{view === "volumes" && <VolumesPage />}
					{view === "networks" && <NetworksPage />}
				</section>
				<Toaster />
			</DemoViewContext.Provider>
		</QueryClientProvider>
	);
}
