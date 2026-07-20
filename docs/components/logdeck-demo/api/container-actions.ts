import {
	containerName,
	emitContainerEvent,
	getContainerById,
	sleep,
	state,
	storedRecordFor,
} from "@/lib/logdeck-demo/store";

async function requireContainer(id: string) {
	// A short pause keeps the pending spinners visible in the demo.
	await sleep(350 + Math.floor(Math.random() * 300));
	const container = getContainerById(id);
	if (!container) throw new Error("Container not found");
	return container;
}

export async function startContainer(id: string, _host: string) {
	const container = await requireContainer(id);
	container.state = "running";
	container.status = container.health
		? "Up a few seconds (healthy)"
		: "Up a few seconds";
	emitContainerEvent(container, "start");
	return `Started ${containerName(container)}`;
}

export async function stopContainer(id: string, _host: string) {
	const container = await requireContainer(id);
	container.state = "exited";
	container.status = "Exited (0) just now";
	emitContainerEvent(container, "stop");
	return `Stopped ${containerName(container)}`;
}

export async function restartContainer(id: string, _host: string) {
	const container = await requireContainer(id);
	container.state = "running";
	container.status = container.health
		? "Up less than a second (healthy)"
		: "Up less than a second";
	emitContainerEvent(container, "restart");
	return `Restarted ${containerName(container)}`;
}

export async function removeContainer(id: string, _host: string) {
	const container = await requireContainer(id);
	const index = state.containers.findIndex((c) => c.id === container.id);
	if (index === -1) throw new Error("Container not found");

	// Keep the stored-log record but flag it removed, so the container shows
	// up under the dashboard's "Removed" state with browsable history.
	const record = storedRecordFor(container);
	if (record) record.removed = true;

	state.containers.splice(index, 1);
	state.stats = state.stats.filter((s) => s.id !== container.id);
	delete state.logs[container.id];
	delete state.env[container.id];
	delete state.resources[container.id];
	emitContainerEvent(container, "destroy");
	return `Removed ${containerName(container)}`;
}
