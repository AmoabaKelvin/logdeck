import { authenticatedFetch, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

import type { TestConnectionResult } from "../types";
import type { DokployHostInput } from "./update-dokploy-hosts";

const ENDPOINT = `${API_BASE_URL}/api/v1/settings/test/dokploy-host`;

export async function testDokployHost(
	host: DokployHostInput,
): Promise<TestConnectionResult> {
	const response = await authenticatedFetch(ENDPOINT, {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify(host),
	});

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to test Dokploy host");
	}

	return readJson<TestConnectionResult>(response);
}
