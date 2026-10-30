import { authenticatedFetch, readJson } from "@/lib/api-client";

import { BACKUPS_ENDPOINT } from "./get-backups";

export interface CreateBackupPayload {
	includeLogs: boolean;
	includeSecrets: boolean;
	/** Empty for an unencrypted backup. */
	passphrase: string;
}

export async function createBackup(
	payload: CreateBackupPayload,
): Promise<string> {
	const response = await authenticatedFetch(BACKUPS_ENDPOINT, {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify(payload),
	});

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to start the backup");
	}

	const data = await readJson<{ message?: string }>(response);
	return data.message ?? "Backup started";
}
