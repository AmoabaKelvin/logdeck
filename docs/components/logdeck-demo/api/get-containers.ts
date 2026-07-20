import { demoHosts, state } from "@/lib/logdeck-demo/store";

import type { ContainerInfo, DockerHost, HostError } from "../types";

export interface GetContainersResponse {
	containers: ContainerInfo[];
	readOnly: boolean;
	hosts: DockerHost[];
	hostErrors: HostError[];
	coolifyConfigured: boolean;
}

export async function getContainers(): Promise<GetContainersResponse> {
	return {
		containers: structuredClone(state.containers),
		readOnly: false,
		hosts: structuredClone(demoHosts),
		hostErrors: [],
		coolifyConfigured: false,
	};
}
