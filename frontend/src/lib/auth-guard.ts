import { redirect } from "@tanstack/react-router";

import { isAuthEnabled } from "@/lib/auth-config";

/**
 * Route guard: redirects to /login when auth is enabled and no token is
 * stored. The auth status comes from the cached /auth/config lookup, so
 * navigations after the first do not hit the network.
 *
 * Fails closed: when the auth status cannot be determined, the
 * AuthConfigUnavailableError from isAuthEnabled propagates and the router's
 * error boundary is shown instead of the app; its retry re-runs this guard
 * and re-fetches the status (failures are not cached).
 */
export async function requireAuthIfEnabled(): Promise<void> {
	const token = localStorage.getItem("logdeck_auth_token");
	if (token) {
		return;
	}

	if (await isAuthEnabled()) {
		throw redirect({ to: "/login" });
	}
}
