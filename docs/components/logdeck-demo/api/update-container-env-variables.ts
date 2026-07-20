import {
	emitContainerEvent,
	getContainerById,
	sleep,
	state,
} from "@/lib/logdeck-demo/store";

export interface UpdateEnvResult {
	newContainerId: string;
	coolifySynced?: boolean;
	coolifyError?: string;
}

// Env changes recreate the container (as they do against a real engine), so
// the demo mints a new id and moves every per-container record across.
export async function updateContainerEnvVariables(
	id: string,
	_host: string,
	env: Record<string, string>,
): Promise<UpdateEnvResult> {
	await sleep(600);
	const container = getContainerById(id);
	if (!container) throw new Error("Container not found");

	const oldId = container.id;
	const newId = `${oldId.slice(0, -4)}${Math.floor(Math.random() * 0xffff)
		.toString(16)
		.padStart(4, "0")}`;

	container.id = newId;
	container.created = Math.floor(Date.now() / 1000);
	container.state = "running";
	container.status = container.health
		? "Up a few seconds (healthy)"
		: "Up a few seconds";

	state.env[newId] = { ...env };
	delete state.env[oldId];
	state.logs[newId] = state.logs[oldId] ?? [];
	delete state.logs[oldId];
	if (state.resources[oldId]) {
		state.resources[newId] = state.resources[oldId];
		delete state.resources[oldId];
	}
	for (const stat of state.stats) {
		if (stat.id === oldId) stat.id = newId;
	}

	emitContainerEvent(container, "recreate");
	return { newContainerId: newId };
}
