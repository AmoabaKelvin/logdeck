import { authenticatedFetch, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

const ENDPOINT = `${API_BASE_URL}/api/v1/settings/coolify-hosts`;

export async function updateCoolifyHosts(
	hosts: { hostName: string; apiURL: string; apiToken: string }[],
	revision?: string,
): Promise<string> {
	const response = await authenticatedFetch(ENDPOINT, {
		method: "PUT",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify({ hosts, revision }),
	});

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to update Coolify hosts");
	}

	const data = await readJson<{ message?: string }>(response);
	return data.message ?? "Coolify hosts updated";
}
