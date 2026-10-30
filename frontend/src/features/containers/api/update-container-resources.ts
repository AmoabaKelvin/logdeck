import { authenticatedFetch, readJson } from "@/lib/api-client";

import { type RestartPolicy, resourcesUrl } from "./get-container-resources";

export interface UpdateResourcesRequest {
	memoryBytes?: number;
	nanoCPUs?: number;
	restartPolicy?: RestartPolicy;
}

async function errorMessage(response: Response, fallback: string) {
	const message = response.headers
		.get("Content-Type")
		?.includes("application/json")
		? (await readJson<{ message: string }>(response)).message
		: (await response.text()).trim();
	return message || fallback;
}

/** Live for standalone and Compose containers; saved only, for a platform. */
export async function updateContainerResources(
	id: string,
	host: string,
	request: UpdateResourcesRequest,
	dokployMapping?: string,
): Promise<{ applied: boolean }> {
	const response = await authenticatedFetch(
		resourcesUrl(id, host, dokployMapping),
		{
			method: "PUT",
			headers: {
				"Content-Type": "application/json",
			},
			body: JSON.stringify(request),
		},
	);

	if (!response.ok) {
		throw new Error(
			await errorMessage(response, "Failed to update container resources"),
		);
	}
	return readJson<{ applied: boolean }>(response);
}

/** Asks the owning platform to deploy its saved limits. */
export async function deployContainerResources(
	id: string,
	host: string,
	dokployMapping?: string,
): Promise<void> {
	const response = await authenticatedFetch(
		resourcesUrl(id, host, dokployMapping, "/deploy"),
		{ method: "POST" },
	);
	if (!response.ok) {
		throw new Error(await errorMessage(response, "Deployment request failed"));
	}
}
