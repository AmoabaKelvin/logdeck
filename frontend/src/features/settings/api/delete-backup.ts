import { authenticatedFetch, readJson } from "@/lib/api-client";

import { BACKUPS_ENDPOINT } from "./get-backups";

export async function deleteBackup(name: string): Promise<string> {
	const response = await authenticatedFetch(
		`${BACKUPS_ENDPOINT}/${encodeURIComponent(name)}`,
		{ method: "DELETE" },
	);

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to delete the backup");
	}

	const data = await readJson<{ message?: string }>(response);
	return data.message ?? "Backup deleted";
}
