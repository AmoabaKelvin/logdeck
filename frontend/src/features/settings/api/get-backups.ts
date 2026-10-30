import { authenticatedFetch, getAuthToken, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

import type { BackupStatus } from "../types";

export const BACKUPS_ENDPOINT = `${API_BASE_URL}/api/v1/settings/backups`;

export async function getBackups(): Promise<BackupStatus> {
	const response = await authenticatedFetch(BACKUPS_ENDPOINT);

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to load backups");
	}

	return readJson<BackupStatus>(response);
}

/**
 * A plain link the browser downloads straight to disk, so a large backup is
 * never held in memory. The session token rides along as the token query
 * parameter, the same way the terminal's WebSocket authenticates.
 */
export function backupDownloadUrl(name: string): string {
	const token = getAuthToken();
	const query = token ? `?${new URLSearchParams({ token })}` : "";
	return `${BACKUPS_ENDPOINT}/${encodeURIComponent(name)}${query}`;
}
