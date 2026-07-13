import { authenticatedFetch } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

const ENDPOINT = `${API_BASE_URL}/api/v1/alerts/history`;

export async function clearAlertHistory(): Promise<string> {
	const response = await authenticatedFetch(ENDPOINT, { method: "DELETE" });

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to clear alert history");
	}

	const data = (await response.json()) as { message?: string };
	return data.message ?? "Alert history cleared";
}
