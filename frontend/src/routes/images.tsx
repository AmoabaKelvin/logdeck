import { createFileRoute } from "@tanstack/react-router";

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
		<main className="container mx-auto px-4 py-8">
			<ImagesPage />
		</main>
	);
}
