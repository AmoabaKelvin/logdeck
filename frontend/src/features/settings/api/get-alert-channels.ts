import { authenticatedFetch } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

const ENDPOINT = `${API_BASE_URL}/api/v1/alerts/channels`;

export type AlertChannelType = "webhook" | "ntfy" | "gotify" | "telegram";

export interface AlertChannel {
	id: string;
	type: AlertChannelType;
	name?: string;
	enabled: boolean;
	url?: string;
	token?: string;
	target?: string;
}

export interface AlertChannelsResponse {
	channels: AlertChannel[];
}

export async function getAlertChannels(): Promise<AlertChannelsResponse> {
	const response = await authenticatedFetch(ENDPOINT);

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to load alert channels");
	}

	return (await response.json()) as AlertChannelsResponse;
}
