import { createFileRoute } from "@tanstack/react-router";

import { AppHeader } from "@/components/app-header";

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
		<div className="isolate min-h-dvh bg-background">
			<AppHeader />
			<main className="app-width px-4 py-8 sm:px-6 lg:px-8">
				<VolumesPage />
			</main>
		</div>
	);
}
