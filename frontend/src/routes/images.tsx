import { createFileRoute } from "@tanstack/react-router";

import { AppHeader } from "@/components/app-header";

import { ImagesPage } from "@/features/resources/components/images-page";
import { requireAuthIfEnabled } from "@/lib/auth-guard";

export const Route = createFileRoute("/images")({
	beforeLoad: async () => {
		await requireAuthIfEnabled();
	},
	component: Images,
});

function Images() {
	return (
		<div className="isolate min-h-dvh bg-background">
			<AppHeader />
			<main className="app-width px-4 py-8 sm:px-6 lg:px-8">
				<ImagesPage />
			</main>
		</div>
	);
}
