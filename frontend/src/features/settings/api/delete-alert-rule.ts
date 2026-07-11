import { authenticatedFetch } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

const ENDPOINT = `${API_BASE_URL}/api/v1/alerts/rules`;

export async function deleteAlertRule(id: string): Promise<string> {
	const response = await authenticatedFetch(
		`${ENDPOINT}/${encodeURIComponent(id)}`,
		{ method: "DELETE" },
	);

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to delete alert rule");
	}

	const data = (await response.json()) as { message?: string };
	return data.message ?? "Alert rule deleted";
}
