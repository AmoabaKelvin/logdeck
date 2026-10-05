import { readStorage, writeStorage } from "@/lib/safe-storage";

const STORAGE_KEY = "logdeck.dokployMappings";

/** Stored for a Compose container the user said Dokploy does not manage. */
export const PLAIN_COMPOSE = "plain";

// One "host/container<TAB>mapping" entry per line.
function readAll(): Map<string, string> {
	const entries = (readStorage(STORAGE_KEY) ?? "")
		.split("\n")
		.filter(Boolean)
		.map((line) => line.split("\t"));
	return new Map(entries.map(([key, mapping]) => [key, mapping ?? ""]));
}

/**
 * Which Dokploy deployment the user confirmed for a container, as "type/id",
 * so reopening the panel does not ask again. The server still checks the
 * mapping on every request, so a stale entry fails safely.
 */
export function readDokployMapping(
	host: string,
	containerId: string,
): string | undefined {
	return readAll().get(`${host}/${containerId}`) || undefined;
}

export function writeDokployMapping(
	host: string,
	containerId: string,
	mapping: string | undefined,
) {
	const all = readAll();
	if (mapping) all.set(`${host}/${containerId}`, mapping);
	else all.delete(`${host}/${containerId}`);
	writeStorage(
		STORAGE_KEY,
		[...all].map((entry) => entry.join("\t")).join("\n"),
	);
}
