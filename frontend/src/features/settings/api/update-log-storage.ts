import { authenticatedFetch } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

const ENDPOINT = `${API_BASE_URL}/api/v1/settings/log-storage`;

/** Only the provided fields are changed; env-sourced caps are omitted. */
export interface UpdateLogStoragePayload {
	enabled?: boolean;
	perContainerMB?: number;
	totalMB?: number;
}

export async function updateLogStorage(
	payload: UpdateLogStoragePayload,
): Promise<string> {
	const response = await authenticatedFetch(ENDPOINT, {
		method: "PUT",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify(payload),
	});

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to update log storage settings");
	}

	const data = (await response.json()) as { message?: string };
	return data.message ?? "Log storage settings updated";
}
