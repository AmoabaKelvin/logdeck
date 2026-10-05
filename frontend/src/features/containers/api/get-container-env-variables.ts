import { authenticatedFetch, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

export interface CoolifyEnvVar {
	uuid: string;
	key: string;
	value: string | null;
	is_preview: boolean;
	is_literal: boolean;
	is_multiline: boolean;
	is_shown_once: boolean;
	is_runtime?: boolean;
	is_buildtime?: boolean;
	is_shared: boolean;
}

export interface EnvVariablesResponse {
	source: "docker" | "coolify";
	variables?: CoolifyEnvVar[];
	resource_type?: "application" | "service";
	env: Record<string, string>;
}

export async function getContainerEnvVariables(
	id: string,
	host: string,
): Promise<EnvVariablesResponse> {
	const response = await authenticatedFetch(
		`${API_BASE_URL}/api/v1/containers/${encodeURIComponent(id)}/env?host=${encodeURIComponent(host)}`,
	);

	if (!response.ok) {
		throw new Error(
			(await response.text()).trim() ||
				"Failed to fetch container environment variables",
		);
	}

	const data = await readJson<EnvVariablesResponse>(response);
	return data;
}
