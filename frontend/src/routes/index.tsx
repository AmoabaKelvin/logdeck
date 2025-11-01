import { createFileRoute } from "@tanstack/react-router";

import { ContainersDashboard } from "@/features/containers/components/containers-dashboard";
import { requireAuthIfEnabled } from "@/lib/auth-guard";

export const Route = createFileRoute("/")({
  beforeLoad: async () => {
    await requireAuthIfEnabled();
  },
  component: Index,
});

function Index() {
  return (
    <main className="container mx-auto px-4 py-8">
      <ContainersDashboard />
    </main>
  );
}
