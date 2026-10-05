import { authenticatedFetch, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

import type { DokployHost } from "../types";

const ENDPOINT = `${API_BASE_URL}/api/v1/settings/dokploy-hosts`;

export type DokployHostInput = Omit<DokployHost, "source">;

export async function updateDokployHosts(
	hosts: DokployHostInput[],
	revision?: string,
): Promise<string> {
	const response = await authenticatedFetch(ENDPOINT, {
		method: "PUT",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify({ hosts, revision }),
	});

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to update Dokploy hosts");
	}

	const data = await readJson<{ message?: string }>(response);
	return data.message ?? "Dokploy hosts updated";
}
