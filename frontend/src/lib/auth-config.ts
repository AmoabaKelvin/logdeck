import { API_BASE_URL } from "@/types/api";

/** Thrown when the server cannot be reached to determine the auth status. */
export class AuthConfigUnavailableError extends Error {
	constructor() {
		super(
			"Unable to determine authentication status. Is the LogDeck server reachable?",
		);
		this.name = "AuthConfigUnavailableError";
	}
}

let authEnabledPromise: Promise<boolean> | null = null;

/**
 * Whether authentication is enabled on the server, from GET /auth/config.
 *
 * The answer is fetched once per page load and cached. Failed lookups are
 * not cached, so a retry (or route reload) re-fetches. Older servers without
 * the endpoint respond 404; auth is then treated as enabled (fail closed)
 * and the login flow reveals whether auth is actually configured.
 */
export function isAuthEnabled(): Promise<boolean> {
	if (!authEnabledPromise) {
		authEnabledPromise = fetchAuthEnabled().catch((error) => {
			authEnabledPromise = null;
			throw error;
		});
	}
	return authEnabledPromise;
}

async function fetchAuthEnabled(): Promise<boolean> {
	let response: Response;
	try {
		response = await fetch(`${API_BASE_URL}/api/v1/auth/config`);
	} catch {
		throw new AuthConfigUnavailableError();
	}

	if (!response.ok) {
		return true;
	}

	try {
		const config = await response.json();
		return config.authEnabled === true;
	} catch {
		throw new AuthConfigUnavailableError();
	}
}

/** Clears the cached auth config. Exposed for tests. */
export function resetAuthConfigCache(): void {
	authEnabledPromise = null;
}
