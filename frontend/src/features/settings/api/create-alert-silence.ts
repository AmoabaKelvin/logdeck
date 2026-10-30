import { authenticatedFetch, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

import type { AlertSilence } from "./get-alert-silences";

const ENDPOINT = `${API_BASE_URL}/api/v1/alerts/silences`;

export type AlertSilencePayload = Omit<
	AlertSilence,
	"id" | "createdAt" | "active" | "until"
>;

export async function createAlertSilence(
	silence: AlertSilencePayload,
): Promise<AlertSilence> {
	const response = await authenticatedFetch(ENDPOINT, {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify(silence),
	});

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to create silence");
	}

	return readJson<AlertSilence>(response);
}
