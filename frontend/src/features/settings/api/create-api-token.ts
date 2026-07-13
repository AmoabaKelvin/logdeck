import { authenticatedFetch } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

import type { APITokenScope, CreatedAPIToken } from "../types";

const ENDPOINT = `${API_BASE_URL}/api/v1/settings/api-tokens`;

export async function createApiToken(
	name: string,
	scope: APITokenScope,
): Promise<CreatedAPIToken> {
	const response = await authenticatedFetch(ENDPOINT, {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify({ name, scope }),
	});

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to create API token");
	}

	return (await response.json()) as CreatedAPIToken;
}
