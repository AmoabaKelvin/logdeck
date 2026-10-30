import { authenticatedFetch, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

const ENDPOINT = `${API_BASE_URL}/api/v1/alerts/silences`;

export async function deleteAlertSilence(id: string): Promise<string> {
	const response = await authenticatedFetch(
		`${ENDPOINT}/${encodeURIComponent(id)}`,
		{ method: "DELETE" },
	);

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to delete silence");
	}

	const data = await readJson<{ message?: string }>(response);
	return data.message ?? "Silence deleted";
}
