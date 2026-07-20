import { getContainerById, state } from "@/lib/logdeck-demo/store";

export interface RestartPolicy {
	name: string;
	maximumRetryCount: number;
}

export interface ContainerResources {
	memoryBytes: number;
	nanoCPUs: number;
	restartPolicy: RestartPolicy;
}

export async function getContainerResources(
	id: string,
	_host: string,
): Promise<ContainerResources> {
	const container = getContainerById(id);
	if (!container) throw new Error("Container not found");
	return structuredClone(
		state.resources[container.id] ?? {
			memoryBytes: 0,
			nanoCPUs: 0,
			restartPolicy: { name: "no", maximumRetryCount: 0 },
		},
	);
}
