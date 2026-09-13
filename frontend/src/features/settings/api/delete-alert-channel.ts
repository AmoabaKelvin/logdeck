import { authenticatedFetch, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

const ENDPOINT = `${API_BASE_URL}/api/v1/alerts/channels`;

export async function deleteAlertChannel(id: string): Promise<string> {
	const response = await authenticatedFetch(
		`${ENDPOINT}/${encodeURIComponent(id)}`,
		{ method: "DELETE" },
	);

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to delete alert channel");
	}

	const data = await readJson<{ message?: string }>(response);
	return data.message ?? "Alert channel deleted";
}
