import { authenticatedFetch, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

import type { UserRole } from "../types";

const ENDPOINT = `${API_BASE_URL}/api/v1/settings/users`;

/** Either field may be left out to keep it. */
export interface UpdateUserPayload {
	role?: UserRole;
	password?: string;
}

export async function updateUser(
	username: string,
	payload: UpdateUserPayload,
): Promise<string> {
	const response = await authenticatedFetch(
		`${ENDPOINT}/${encodeURIComponent(username)}`,
		{
			method: "PUT",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify(payload),
		},
	);

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to update user");
	}

	const data = await readJson<{ message?: string }>(response);
	return data.message ?? "User updated";
}
