import { authenticatedFetch, readJson } from "@/lib/api-client";

import type { BackupSchedule } from "../types";
import { BACKUPS_ENDPOINT } from "./get-backups";

export async function updateBackupSchedule(
	schedule: BackupSchedule,
): Promise<string> {
	const response = await authenticatedFetch(`${BACKUPS_ENDPOINT}/schedule`, {
		method: "PUT",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify(schedule),
	});

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to update the backup schedule");
	}

	const data = await readJson<{ message?: string }>(response);
	return data.message ?? "Backup schedule updated";
}
