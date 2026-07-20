import {
	emitContainerEvent,
	getContainerById,
	sleep,
	state,
} from "@/lib/logdeck-demo/store";

import type { RestartPolicy } from "./get-container-resources";

export interface UpdateResourcesRequest {
	memoryBytes?: number;
	nanoCPUs?: number;
	restartPolicy?: RestartPolicy;
}

export async function updateContainerResources(
	id: string,
	_host: string,
	request: UpdateResourcesRequest,
): Promise<void> {
	await sleep(400);
	const container = getContainerById(id);
	if (!container) throw new Error("Container not found");

	const current = state.resources[container.id] ?? {
		memoryBytes: 0,
		nanoCPUs: 0,
		restartPolicy: { name: "no", maximumRetryCount: 0 },
	};
	state.resources[container.id] = {
		memoryBytes: request.memoryBytes ?? current.memoryBytes,
		nanoCPUs: request.nanoCPUs ?? current.nanoCPUs,
		restartPolicy: request.restartPolicy ?? current.restartPolicy,
	};
	emitContainerEvent(container, "update");
}
