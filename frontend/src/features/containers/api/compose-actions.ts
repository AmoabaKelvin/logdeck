import { authenticatedFetch, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

const BASE_URL = `${API_BASE_URL}/api/v1/compose`;

export type ComposeAction = "start" | "stop" | "restart";

interface ComposeActionFailure {
	id: string;
	name: string;
	error: string;
}

export interface ComposeActionResult {
	project: string;
	host: string;
	total: number;
	succeeded: number;
	failed: ComposeActionFailure[];
}

export async function performComposeAction(
	project: string,
	action: ComposeAction,
	host: string,
): Promise<ComposeActionResult> {
	const endpoint = `${BASE_URL}/${encodeURIComponent(project)}/${action}?host=${encodeURIComponent(host)}`;
	const response = await authenticatedFetch(endpoint, {
		method: "POST",
		headers: {
			"Content-Type": "application/json",
		},
	});

	if (!response.ok) {
		const text = await response.text();
		let message = text || `Failed to ${action} compose project ${project}`;
		try {
			// SAFETY: compose_handlers.go writes non-OK bodies either as plain text
			// via http.Error (JSON.parse throws into the catch) or as the
			// ComposeActionResult JSON; every field here is optional.
			const parsed = JSON.parse(text) as Partial<ComposeActionResult> & {
				error?: string;
			};
			if (Array.isArray(parsed.failed) && parsed.failed.length > 0) {
				const names = parsed.failed
					.map((failure) => failure.name || failure.id.slice(0, 12))
					.join(", ");
				message = `Failed to ${action} ${parsed.failed.length} of ${parsed.total} container(s) in ${project}: ${names}`;
			} else if (parsed.error) {
				message = parsed.error;
			}
		} catch {
			// Non-JSON error body; keep the raw text.
		}
		throw new Error(message);
	}

	return readJson<ComposeActionResult>(response);
}
