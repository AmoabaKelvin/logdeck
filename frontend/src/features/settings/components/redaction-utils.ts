import type {
	RedactionConfig,
	RedactionResponse,
	RedactionRule,
} from "../api/get-redaction";

export const EMPTY_REDACTION_RULE: RedactionRule = {
	name: "",
	enabled: true,
	pattern: "",
	replacement: "",
	hosts: [],
	containers: [],
	projects: [],
};

const orUndefined = <T>(values: T[] | undefined) =>
	values && values.length > 0 ? values : undefined;

/** The editable part of the server response. */
export function redactionDraft(response: RedactionResponse): RedactionConfig {
	return {
		enabled: response.enabled,
		replacement: response.replacement ?? "",
		builtins: response.builtins,
		rules: response.rules,
	};
}

/**
 * The config as it is sent and compared: names trimmed, and empty optional
 * fields dropped so an untouched draft equals what the server returned.
 */
export function cleanRedactionConfig(draft: RedactionConfig): RedactionConfig {
	return {
		enabled: draft.enabled,
		replacement: draft.replacement || undefined,
		builtins: draft.builtins,
		rules: draft.rules.map((rule) => ({
			name: rule.name.trim(),
			enabled: rule.enabled,
			pattern: rule.pattern,
			replacement: rule.replacement || undefined,
			hosts: orUndefined(rule.hosts),
			containers: orUndefined(rule.containers),
			projects: orUndefined(rule.projects),
		})),
	};
}

/** Catches what can be told without the server; the regex itself is checked there, in RE2. */
export function validateRedactionDraft(draft: RedactionConfig): string | null {
	const index = draft.rules.findIndex((rule) => rule.pattern === "");
	if (index >= 0) {
		const name = draft.rules[index].name.trim();
		return `${name ? `"${name}"` : `Custom pattern ${index + 1}`} needs a pattern`;
	}
	return null;
}
