import { authenticatedFetch, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

import type { RedactionConfig } from "./get-redaction";

const ENDPOINT = `${API_BASE_URL}/api/v1/settings/redaction`;

/** Replaces the whole redaction config. Invalid rules are rejected with the reason. */
export async function updateRedaction(
	config: RedactionConfig,
): Promise<string> {
	const response = await authenticatedFetch(ENDPOINT, {
		method: "PUT",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify(config),
	});

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message.trim() || "Failed to update redaction settings");
	}

	const data = await readJson<{ message?: string }>(response);
	return data.message ?? "Redaction settings updated";
}
