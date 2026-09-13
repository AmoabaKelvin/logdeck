const JSON_CACHE_LIMIT = 1000;

interface FormattedJson {
  formatted: string;
  isJson: boolean;
}

const jsonFormatCache = new Map<string, FormattedJson>();

function setCachedValue(text: string, value: FormattedJson) {
  if (jsonFormatCache.size >= JSON_CACHE_LIMIT) {
    const oldestKey = jsonFormatCache.keys().next().value;
    if (oldestKey) {
      jsonFormatCache.delete(oldestKey);
    }
  }
  jsonFormatCache.set(text, value);
}

/**
 * Attempts to parse and pretty-print JSON.
 * Returns formatted JSON with isJson flag on success.
 * Returns original text with isJson: false on failure.
 */
export function formatJson(text: string): FormattedJson {
  const cached = jsonFormatCache.get(text);
  if (cached) return cached;

  const trimmed = text.trim();
  if (!trimmed.startsWith("{") && !trimmed.startsWith("[")) {
    const result = { formatted: text, isJson: false };
    setCachedValue(text, result);
    return result;
  }

  try {
    // Text starting with "{" or "[" can only parse to an object or an array.
    const result = {
      formatted: JSON.stringify(JSON.parse(trimmed), null, 2),
      isJson: true,
    };
    setCachedValue(text, result);
    return result;
  } catch {
    // fall through to non-JSON result
  }

  const result = { formatted: text, isJson: false };
  setCachedValue(text, result);
  return result;
}

/**
 * Detects if a string is valid JSON object or array.
 * Returns false for primitives (strings, numbers, booleans, null).
 * Only returns true for objects and arrays.
 */
export function isJsonString(text: string): boolean {
  return formatJson(text).isJson;
}
