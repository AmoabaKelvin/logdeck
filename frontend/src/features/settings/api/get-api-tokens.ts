import { authenticatedFetch } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

import type { APITokensResponse } from "../types";

const ENDPOINT = `${API_BASE_URL}/api/v1/settings/api-tokens`;

export async function getApiTokens(): Promise<APITokensResponse> {
	const response = await authenticatedFetch(ENDPOINT);

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to load API tokens");
	}

	return (await response.json()) as APITokensResponse;
}
