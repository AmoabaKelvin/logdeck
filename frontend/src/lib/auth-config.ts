import { API_BASE_URL } from "@/types/api";

import { readJson } from "./api-client";

/** Thrown when the server cannot be reached to determine the auth status. */
export class AuthConfigUnavailableError extends Error {
	constructor() {
		super(
			"Unable to determine authentication status. Is the LogDeck server reachable?",
		);
		this.name = "AuthConfigUnavailableError";
	}
}

export interface AuthConfig {
	authEnabled: boolean;
	/** Whether the login page offers sign-in through the OIDC provider. */
	oidcEnabled: boolean;
}

let authConfigPromise: Promise<AuthConfig> | null = null;

/**
 * The server's auth status, from GET /auth/config.
 *
 * The answer is fetched once per page load and cached. Failed lookups are
 * not cached, so a retry (or route reload) re-fetches. Older servers without
 * the endpoint respond 404; auth is then treated as enabled (fail closed)
 * and the login flow reveals whether auth is actually configured.
 */
export function getAuthConfig(): Promise<AuthConfig> {
	if (!authConfigPromise) {
		authConfigPromise = fetchAuthConfig().catch((error) => {
			authConfigPromise = null;
			throw error;
		});
	}
	return authConfigPromise;
}

/** Whether authentication is enabled on the server. See getAuthConfig. */
export async function isAuthEnabled(): Promise<boolean> {
	return (await getAuthConfig()).authEnabled;
}

async function fetchAuthConfig(): Promise<AuthConfig> {
	let response: Response;
	try {
		response = await fetch(`${API_BASE_URL}/api/v1/auth/config`);
	} catch {
		throw new AuthConfigUnavailableError();
	}

	if (!response.ok) {
		return { authEnabled: true, oidcEnabled: false };
	}

	try {
		const config = await readJson<Partial<AuthConfig>>(response);
		return {
			authEnabled: config.authEnabled === true,
			oidcEnabled: config.oidcEnabled === true,
		};
	} catch {
		throw new AuthConfigUnavailableError();
	}
}

/** Clears the cached auth config. Exposed for tests. */
export function resetAuthConfigCache(): void {
	authConfigPromise = null;
}
