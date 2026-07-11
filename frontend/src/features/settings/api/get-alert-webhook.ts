import { authenticatedFetch } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

const ENDPOINT = `${API_BASE_URL}/api/v1/alerts/webhook`;

export interface AlertWebhookResponse {
	url: string;
}

export async function getAlertWebhook(): Promise<AlertWebhookResponse> {
	const response = await authenticatedFetch(ENDPOINT);

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to load alert webhook");
	}

	return (await response.json()) as AlertWebhookResponse;
}
