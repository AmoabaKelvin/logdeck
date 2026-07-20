import { emitContainerEvent, sleep, state } from "@/lib/logdeck-demo/store";

export type ComposeAction = "start" | "stop" | "restart";

interface ComposeActionFailure {
	id: string;
	name: string;
	error: string;
}

export interface ComposeActionResult {
	project: string;
	host: string;
	total: number;
	succeeded: number;
	failed: ComposeActionFailure[];
}

export async function performComposeAction(
	project: string,
	action: ComposeAction,
	host: string,
): Promise<ComposeActionResult> {
	await sleep(400 + Math.floor(Math.random() * 400));

	const members = state.containers.filter(
		(container) =>
			container.host === host &&
			container.labels?.["com.docker.compose.project"] === project,
	);

	for (const container of members) {
		if (action === "stop") {
			container.state = "exited";
			container.status = "Exited (0) just now";
		} else {
			container.state = "running";
			container.status = container.health
				? "Up a few seconds (healthy)"
				: "Up a few seconds";
		}
		emitContainerEvent(container, action);
	}

	return {
		project,
		host,
		total: members.length,
		succeeded: members.length,
		failed: [],
	};
}
