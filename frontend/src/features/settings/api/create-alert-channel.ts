import { authenticatedFetch } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

import type { AlertChannel } from "./get-alert-channels";

const ENDPOINT = `${API_BASE_URL}/api/v1/alerts/channels`;

export type AlertChannelPayload = Omit<AlertChannel, "id">;

export async function createAlertChannel(
	channel: AlertChannelPayload,
): Promise<AlertChannel> {
	const response = await authenticatedFetch(ENDPOINT, {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify(channel),
	});

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to create alert channel");
	}

	return (await response.json()) as AlertChannel;
}
