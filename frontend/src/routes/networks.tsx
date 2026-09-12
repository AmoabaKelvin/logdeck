import { createFileRoute } from "@tanstack/react-router";

import { AppHeader } from "@/components/app-header";

import { NetworksPage } from "@/features/resources/components/networks-page";
import { requireAuthIfEnabled } from "@/lib/auth-guard";

export const Route = createFileRoute("/networks")({
	beforeLoad: async () => {
		await requireAuthIfEnabled();
	},
	component: Networks,
});

function Networks() {
	return (
		<div className="isolate min-h-dvh bg-background">
			<AppHeader />
			<main className="app-width px-4 py-8 sm:px-6 lg:px-8">
				<NetworksPage />
			</main>
		</div>
	);
}
