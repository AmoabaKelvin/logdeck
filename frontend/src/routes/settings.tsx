import { createFileRoute } from "@tanstack/react-router";

import { AppHeader } from "@/components/app-header";
import { SettingsPage } from "@/features/settings/components/settings-page";
import { requireAuthIfEnabled } from "@/lib/auth-guard";

export const Route = createFileRoute("/settings")({
	beforeLoad: async () => {
		await requireAuthIfEnabled();
	},
	component: Settings,
});

function Settings() {
	return (
		<div className="isolate min-h-dvh bg-background">
			<AppHeader />
			<SettingsPage />
		</div>
	);
}
