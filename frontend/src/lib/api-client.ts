const TOKEN_KEY = "logdeck_auth_token";

/**
 * Authenticated fetch wrapper that automatically adds Authorization header
 * and handles 401 responses by logging out the user
 */
export async function authenticatedFetch(
	input: RequestInfo | URL,
	init?: RequestInit,
): Promise<Response> {
	const token = localStorage.getItem(TOKEN_KEY);

	const headers =
		input instanceof Request ? new Headers(input.headers) : new Headers();

	if (init?.headers) {
		const initHeaders = new Headers(init.headers);
		initHeaders.forEach((value, key) => {
			headers.set(key, value);
		});
	}

	if (token) {
		headers.set("Authorization", `Bearer ${token}`);
	}

	const response = await fetch(input, {
		...init,
		headers,
	});

	// Handle 401 Unauthorized - token expired or invalid: clear the token and
	// send the user to the login page (unless already there).
	if (response.status === 401) {
		localStorage.removeItem(TOKEN_KEY);

		if (window.location.pathname !== "/login") {
			window.location.href = "/login";
		}
	}

	return response;
}

export function getAuthToken(): string | null {
	return localStorage.getItem(TOKEN_KEY);
}

export function setAuthToken(token: string): void {
	localStorage.setItem(TOKEN_KEY, token);
}

export function removeAuthToken(): void {
	localStorage.removeItem(TOKEN_KEY);
}
