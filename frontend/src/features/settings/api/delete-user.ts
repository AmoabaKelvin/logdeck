import { authenticatedFetch, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

const ENDPOINT = `${API_BASE_URL}/api/v1/settings/users`;

export async function deleteUser(username: string): Promise<string> {
	const response = await authenticatedFetch(
		`${ENDPOINT}/${encodeURIComponent(username)}`,
		{ method: "DELETE" },
	);

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to delete user");
	}

	const data = await readJson<{ message?: string }>(response);
	return data.message ?? "User deleted";
}
