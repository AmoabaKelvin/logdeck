import { authenticatedFetch, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

import type { RedactionConfig } from "./get-redaction";

const ENDPOINT = `${API_BASE_URL}/api/v1/settings/redaction/preview`;

/** Runs an unsaved config over the sample. The server keeps neither. */
export async function previewRedaction(
	config: RedactionConfig,
	sample: string,
): Promise<string> {
	const response = await authenticatedFetch(ENDPOINT, {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify({ config, sample }),
	});

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message.trim() || "Failed to preview redaction");
	}

	const data = await readJson<{ output: string }>(response);
	return data.output;
}
