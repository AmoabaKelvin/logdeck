import { authenticatedFetch, readJson } from "@/lib/api-client";

import type { RestorePreview } from "../types";
import { BACKUPS_ENDPOINT } from "./get-backups";

/** Either a file to upload or the name of a backup already on the server. */
export type RestoreSource = { file: File } | { name: string };

/**
 * Has the server verify a backup and stage it. Nothing on the instance
 * changes until the restore is confirmed. An uploaded file is streamed as the
 * request body; the passphrase travels in a header, never in the URL.
 */
export async function checkRestore(
	source: RestoreSource,
	passphrase: string,
): Promise<RestorePreview> {
	const url =
		"file" in source
			? `${BACKUPS_ENDPOINT}/restore`
			: `${BACKUPS_ENDPOINT}/${encodeURIComponent(source.name)}/restore`;
	const headers = new Headers({ "Content-Type": "application/octet-stream" });
	if (passphrase) headers.set("X-Backup-Passphrase", passphrase);

	const response = await authenticatedFetch(url, {
		method: "POST",
		headers,
		body: "file" in source ? source.file : undefined,
	});

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "The backup could not be checked");
	}

	return readJson<RestorePreview>(response);
}

export async function confirmRestore(): Promise<string> {
	const response = await authenticatedFetch(
		`${BACKUPS_ENDPOINT}/restore/confirm`,
		{ method: "POST" },
	);

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to confirm the restore");
	}

	const data = await readJson<{ message?: string }>(response);
	return data.message ?? "Restore scheduled";
}

export async function cancelRestore(): Promise<string> {
	const response = await authenticatedFetch(`${BACKUPS_ENDPOINT}/restore`, {
		method: "DELETE",
	});

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to cancel the restore");
	}

	const data = await readJson<{ message?: string }>(response);
	return data.message ?? "Restore cancelled";
}
