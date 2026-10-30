import { authenticatedFetch, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

const ENDPOINT = `${API_BASE_URL}/api/v1/settings/redaction`;

export interface RedactionRule {
	name: string;
	enabled: boolean;
	/** RE2 syntax. */
	pattern: string;
	/** Empty uses the config's replacement; $1 expands a capture group. */
	replacement?: string;
	hosts?: string[];
	containers?: string[];
	projects?: string[];
}

export interface RedactionConfig {
	enabled: boolean;
	replacement?: string;
	/** IDs of the enabled built-in patterns. */
	builtins: string[];
	rules: RedactionRule[];
}

export interface RedactionBuiltin {
	id: string;
	name: string;
	description: string;
	default: boolean;
}

export interface RedactionResponse extends RedactionConfig {
	catalog: RedactionBuiltin[];
	defaultReplacement: string;
}

export async function getRedaction(): Promise<RedactionResponse> {
	const response = await authenticatedFetch(ENDPOINT);

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to load redaction settings");
	}

	return readJson<RedactionResponse>(response);
}
