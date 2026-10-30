import { authenticatedFetch, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

import type { ImageUpdate } from "../types";

async function post<T>(
	id: string,
	host: string,
	action: string,
	fallback: string,
) {
	const response = await authenticatedFetch(
		`${API_BASE_URL}/api/v1/containers/${encodeURIComponent(id)}/image/${action}?host=${encodeURIComponent(host)}`,
		{ method: "POST" },
	);
	if (!response.ok) {
		throw new Error((await response.text()).trim() || fallback);
	}
	return readJson<T>(response);
}

/** Asks the registry now instead of waiting for the background check. */
export async function checkImageUpdate(
	id: string,
	host: string,
): Promise<ImageUpdate> {
	const data = await post<{ image_update: ImageUpdate }>(
		id,
		host,
		"check",
		"Image check failed",
	);
	return data.image_update;
}

export interface ImageUpdateResult {
	message: string;
	/** False when the container already ran the newest image. */
	updated: boolean;
	new_container_id?: string;
}

/** Pulls the container's tag and recreates a standalone container from it. */
export function updateContainerImage(id: string, host: string) {
	return post<ImageUpdateResult>(id, host, "update", "Image update failed");
}
