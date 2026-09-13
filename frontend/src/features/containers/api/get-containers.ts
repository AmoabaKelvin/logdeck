import { authenticatedFetch, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

import type { ContainerInfo, DockerHost, HostError } from "../types";

const CONTAINERS_ENDPOINT = `${API_BASE_URL}/api/v1/containers`;

export interface GetContainersResponse {
	containers: ContainerInfo[];
	readOnly: boolean;
	hosts: DockerHost[];
	hostErrors: HostError[];
	coolifyConfigured: boolean;
}

export async function getContainers(): Promise<GetContainersResponse> {
	const response = await authenticatedFetch(CONTAINERS_ENDPOINT);

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || `Request failed with status ${response.status}`);
	}

	const data = await readJson<Partial<GetContainersResponse> | null>(response);

	if (!data) {
		throw new Error("Unexpected response format");
	}

	const containers = data.containers;
	const readOnly = data.readOnly ?? false;
	const hosts = data.hosts;
	const hostErrors = data.hostErrors;
	const coolifyConfigured = data.coolifyConfigured ?? false;

	if (!Array.isArray(containers)) {
		throw new Error("Unexpected response format");
	}

	if (!Array.isArray(hosts)) {
		throw new Error("Unexpected response format");
	}

	return {
		containers,
		readOnly,
		hosts,
		hostErrors: Array.isArray(hostErrors) ? hostErrors : [],
		coolifyConfigured,
	};
}
