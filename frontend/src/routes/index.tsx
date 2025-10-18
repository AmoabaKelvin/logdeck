import { createFileRoute } from "@tanstack/react-router";

import { ContainersDashboard } from "@/features/containers/components/containers-dashboard";

export const Route = createFileRoute("/")({
  component: Index,
});

function Index() {
  return (
    <main className="container mx-auto px-4 py-8">
      <ContainersDashboard />
    </main>
  );
}
