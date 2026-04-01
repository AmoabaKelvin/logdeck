import { createFileRoute } from "@tanstack/react-router";

import { SettingsPage } from "@/features/settings/components/settings-page";
import { requireAuthIfEnabled } from "@/lib/auth-guard";

export const Route = createFileRoute("/settings")({
  beforeLoad: async () => {
    await requireAuthIfEnabled();
  },
  component: SettingsPage,
});
