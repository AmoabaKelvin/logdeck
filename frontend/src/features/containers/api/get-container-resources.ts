import { authenticatedFetch, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

export interface RestartPolicy {
	name: string;
	maximumRetryCount: number;
}

export interface ResourceLimits {
	memoryBytes: number;
	nanoCPUs: number;
}

/**
 * The platform that recreates the container from saved settings, so a live
 * change would not survive its next deployment. Plain Compose still updates
 * live; the Compose file must carry the limits to keep them.
 */
export interface ResourcesOwner {
	platform: "coolify" | "dokploy" | "swarm" | "compose";
	/** LogDeck can save memory and CPU limits in the platform. */
	persistent: boolean;
	/** What the platform applies on its next deployment. */
	configured?: ResourceLimits;
	/** Why the limits cannot be saved through LogDeck. */
	reason?: string;
	url?: string;
}

/** The limits the container runs with now, and who owns them. */
export interface ContainerResources extends ResourceLimits {
	restartPolicy: RestartPolicy;
	owner?: ResourcesOwner;
}

/** A confirmed Dokploy deployment, as "type/id", the server checks again. */
export function resourcesUrl(
	id: string,
	host: string,
	dokployMapping?: string,
	suffix = "",
) {
	const query = new URLSearchParams({ host });
	const [type, resource] = dokployMapping?.split("/") ?? [];
	if (type && resource) {
		query.set("resource", resource);
		query.set("resource_type", type);
	}
	return `${API_BASE_URL}/api/v1/containers/${encodeURIComponent(id)}/resources${suffix}?${query}`;
}

export async function getContainerResources(
	id: string,
	host: string,
	dokployMapping?: string,
): Promise<ContainerResources> {
	const response = await authenticatedFetch(
		resourcesUrl(id, host, dokployMapping),
	);

	if (!response.ok) {
		throw new Error("Failed to fetch container resources");
	}

	return readJson<ContainerResources>(response);
}
