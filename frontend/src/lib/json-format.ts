const JSON_CACHE_LIMIT = 1000;
const jsonFormatCache = new Map<
	string,
	{ formatted: string; isJson: boolean }
>();

function setCachedValue(
	text: string,
	value: { formatted: string; isJson: boolean },
) {
	if (jsonFormatCache.size >= JSON_CACHE_LIMIT) {
		const oldestKey = jsonFormatCache.keys().next().value;
		if (oldestKey) {
			jsonFormatCache.delete(oldestKey);
		}
	}
	jsonFormatCache.set(text, value);
}

// Only objects and arrays count as JSON here; primitives like numbers or
// booleans are technically valid JSON too, but formatting them would turn
// plain log lines that happen to be a bare number into "JSON".
export function formatJson(text: string): {
	formatted: string;
	isJson: boolean;
} {
	const cached = jsonFormatCache.get(text);
	if (cached) return cached;

	const trimmed = text.trim();
	if (!trimmed.startsWith("{") && !trimmed.startsWith("[")) {
		const result = { formatted: text, isJson: false };
		setCachedValue(text, result);
		return result;
	}

	try {
		const parsed = JSON.parse(trimmed);
		if (typeof parsed === "object" && parsed !== null) {
			const result = {
				formatted: JSON.stringify(parsed, null, 2),
				isJson: true,
			};
			setCachedValue(text, result);
			return result;
		}
	} catch {}

	const result = { formatted: text, isJson: false };
	setCachedValue(text, result);
	return result;
}

export function isJsonString(text: string): boolean {
	return formatJson(text).isJson;
}
