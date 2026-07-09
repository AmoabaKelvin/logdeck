import { createFileRoute } from "@tanstack/react-router";

import { VolumesPage } from "@/features/resources/components/volumes-page";
import { requireAuthIfEnabled } from "@/lib/auth-guard";

export const Route = createFileRoute("/volumes")({
	beforeLoad: async () => {
		await requireAuthIfEnabled();
	},
	component: Volumes,
});

function Volumes() {
	return (
		<main className="container mx-auto px-4 py-8">
			<VolumesPage />
		</main>
	);
}
