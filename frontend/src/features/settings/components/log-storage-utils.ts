const MIN_MB = 1;
const MAX_MB = 1_048_576;

// Parses a cap coming from a text input. Anything that is not a whole number of
// MB within the server's accepted range is rejected: a cap of zero would make
// the janitor evict the whole store on its next pass.
function parseMB(raw: string): number | null {
	const value = Number(raw.trim());
	if (!Number.isInteger(value) || value < MIN_MB || value > MAX_MB) return null;
	return value;
}

/**
 * Mirrors the server's validation so an invalid cap never leaves the browser.
 * Returns an error message, or null when both caps are acceptable.
 */
export function validateRetentionCaps(
	perContainerMB: string,
	totalMB: string,
): string | null {
	const perContainer = parseMB(perContainerMB);
	if (perContainer === null) {
		return `Per-container cap must be a whole number of MB between ${MIN_MB} and ${MAX_MB}`;
	}

	const total = parseMB(totalMB);
	if (total === null) {
		return `Total cap must be a whole number of MB between ${MIN_MB} and ${MAX_MB}`;
	}

	if (perContainer > total) {
		return "Per-container cap cannot exceed the total cap";
	}

	return null;
}
