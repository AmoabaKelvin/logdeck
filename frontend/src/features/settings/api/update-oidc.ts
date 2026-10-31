import { authenticatedFetch, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

import type { OIDCConfig } from "../types";

const ENDPOINT = `${API_BASE_URL}/api/v1/settings/oidc`;

export type UpdateOIDCPayload = Omit<OIDCConfig, "source">;

export async function updateOIDC(payload: UpdateOIDCPayload): Promise<string> {
	const response = await authenticatedFetch(ENDPOINT, {
		method: "PUT",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify(payload),
	});

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to update OIDC settings");
	}

	const data = await readJson<{ message?: string }>(response);
	return data.message ?? "OIDC settings updated";
}
