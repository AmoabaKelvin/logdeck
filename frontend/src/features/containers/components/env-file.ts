/** Parses a .env file body into key/value pairs, ignoring blanks and comments. */
export function parseEnvFile(content: string): Record<string, string> {
	const env: Record<string, string> = {};
	const lines = content.split("\n");

	for (const line of lines) {
		const trimmed = line.trim();

		if (!trimmed || trimmed.startsWith("#")) {
			continue;
		}

		const equalIndex = trimmed.indexOf("=");
		if (equalIndex === -1) {
			continue;
		}

		const key = trimmed.substring(0, equalIndex).trim();
		let value = trimmed.substring(equalIndex + 1).trim();

		if (
			(value.startsWith('"') && value.endsWith('"')) ||
			(value.startsWith("'") && value.endsWith("'"))
		) {
			value = value.slice(1, -1);
		}

		if (key) {
			env[key] = value;
		}
	}

	return env;
}

// Keys whose values should not sit in plain sight on a shared screen. Matching
// on the name rather than the value keeps this predictable: a reader can tell
// at a glance why a row is masked.
const SECRET_KEY_PATTERN =
	/(^|_)(SECRET|SECRETS|TOKEN|PASSWORD|PASSWD|PASS|APIKEY|CREDENTIAL|CREDENTIALS|PRIVATE|DSN|SALT|SIGNATURE)($|_)|(^|_)KEY($|_)/i;

export function isSecretKey(key: string): boolean {
	return SECRET_KEY_PATTERN.test(key);
}

/** A fixed-width stand-in, so the mask never hints at the value's length. */
export const MASKED_VALUE = "••••••••••••";
