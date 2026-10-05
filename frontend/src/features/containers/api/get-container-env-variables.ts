import { authenticatedFetch, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

import type { DokployEnvResponse } from "./dokploy-env";

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

export interface ContainerEnvResponse {
	source: "docker" | "coolify";
	variables?: CoolifyEnvVar[];
	resource_type?: "application" | "service";
	env: Record<string, string>;
	/** The runtime editor was chosen over a Dokploy mapping on this host. */
	plain_compose?: boolean;
}

export type EnvVariablesResponse = ContainerEnvResponse | DokployEnvResponse;

export async function getContainerEnvVariables(
	id: string,
	host: string,
	plainCompose = false,
): Promise<EnvVariablesResponse> {
	const response = await authenticatedFetch(
		`${API_BASE_URL}/api/v1/containers/${encodeURIComponent(id)}/env?host=${encodeURIComponent(host)}${plainCompose ? "&platform=docker&confirmed=true" : ""}`,
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
