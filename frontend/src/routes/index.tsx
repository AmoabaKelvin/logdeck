import { createFileRoute, redirect } from "@tanstack/react-router";

import { ContainersDashboard } from "@/features/containers/components/containers-dashboard";
import { API_BASE_URL } from "@/types/api";

export const Route = createFileRoute("/")({
  beforeLoad: async () => {
    const token = localStorage.getItem("logdeck_auth_token");

    // If no token, check if auth is required
    if (!token) {
      try {
        const authUrl = `${API_BASE_URL}/api/v1/auth/login`.replace(
          /([^:]\/)\/+/g,
          "$1"
        );
        const response = await fetch(authUrl, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ username: "", password: "" }),
        });

        // If 404, auth is disabled - allow access
        if (response.status === 404) {
          return;
        }

        // Auth is enabled but no token - redirect to login
        throw redirect({ to: "/login" });
      } catch (error) {
        // If we can't reach the server, allow access (fail open for development)
        if (error instanceof Error && error.message.includes("redirect")) {
          throw error;
        }
      }
    }
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
