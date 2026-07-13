import { authenticatedFetch } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

const ENDPOINT = `${API_BASE_URL}/api/v1/alerts/webhook`;

export async function updateAlertWebhook(url: string): Promise<string> {
	const response = await authenticatedFetch(ENDPOINT, {
		method: "PUT",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify({ url }),
	});

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to update alert webhook");
	}

	const data = (await response.json()) as { message?: string };
	return data.message ?? "Webhook updated";
}
