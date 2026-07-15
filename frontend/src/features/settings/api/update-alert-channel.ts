import { authenticatedFetch } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

import type { AlertChannelPayload } from "./create-alert-channel";
import type { AlertChannel } from "./get-alert-channels";

const ENDPOINT = `${API_BASE_URL}/api/v1/alerts/channels`;

export async function updateAlertChannel(
	id: string,
	channel: AlertChannelPayload,
): Promise<AlertChannel> {
	const response = await authenticatedFetch(
		`${ENDPOINT}/${encodeURIComponent(id)}`,
		{
			method: "PUT",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify(channel),
		},
	);

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to update alert channel");
	}

	return (await response.json()) as AlertChannel;
}
