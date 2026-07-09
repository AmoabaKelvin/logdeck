import type { HostError } from "@/features/containers/types";
import { authenticatedFetch } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";
import type { ImageInfo, NetworkInfo, VolumeInfo } from "../types";

export interface ResourceResponse<T> {
	items: T[];
	hostErrors: HostError[];
}

async function getResource<T>(
	path: string,
	key: string,
): Promise<ResourceResponse<T>> {
	const response = await authenticatedFetch(`${API_BASE_URL}/api/v1/${path}`);

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || `Request failed with status ${response.status}`);
	}

	const data = (await response.json()) as Record<string, unknown> | null;
	const items = data?.[key];

	if (!Array.isArray(items)) {
		throw new Error("Unexpected response format");
	}

	const hostErrors = data?.hostErrors;

	return {
		items: items as T[],
		hostErrors: Array.isArray(hostErrors) ? (hostErrors as HostError[]) : [],
	};
}

export function getImages(): Promise<ResourceResponse<ImageInfo>> {
	return getResource<ImageInfo>("images", "images");
}

export function getVolumes(): Promise<ResourceResponse<VolumeInfo>> {
	return getResource<VolumeInfo>("volumes", "volumes");
}

export function getNetworks(): Promise<ResourceResponse<NetworkInfo>> {
	return getResource<NetworkInfo>("networks", "networks");
}
