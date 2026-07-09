import { createFileRoute } from "@tanstack/react-router";

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
		<main className="container mx-auto px-4 py-8">
			<NetworksPage />
		</main>
	);
}
