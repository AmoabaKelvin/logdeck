import { authenticatedFetch, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

import type { UsersResponse } from "../types";

const ENDPOINT = `${API_BASE_URL}/api/v1/settings/users`;

export async function getUsers(): Promise<UsersResponse> {
	const response = await authenticatedFetch(ENDPOINT);

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to load users");
	}

	return readJson<UsersResponse>(response);
}
