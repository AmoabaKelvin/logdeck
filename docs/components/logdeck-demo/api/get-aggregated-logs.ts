import {
	appendLog,
	buildLiveLog,
	getContainerById,
	sleep,
} from "@/lib/logdeck-demo/store";

import type {
	ContainerLogsOptions,
	LogEntry,
	LogStreamHeartbeat,
} from "./get-container-logs-parsed";
import { getContainerLogsParsed } from "./get-container-logs-parsed";

export interface AggregateLogTarget {
	id: string;
	host: string;
	name?: string;
}

export async function getAggregatedLogs(
	targets: AggregateLogTarget[],
	options?: ContainerLogsOptions,
): Promise<LogEntry[]> {
	const perContainer = await Promise.all(
		targets.map(async (target) => {
			const logs = await getContainerLogsParsed(target.id, target.host, options);
			return logs.map((entry) => ({
				...entry,
				containerId: target.id,
				containerName: target.name,
			}));
		}),
	);

	return perContainer
		.flat()
		.sort(
			(a, b) =>
				new Date(a.timestamp ?? 0).getTime() -
				new Date(b.timestamp ?? 0).getTime(),
		);
}

export async function* streamAggregatedLogs(
	targets: AggregateLogTarget[],
	_options?: ContainerLogsOptions,
	signal?: AbortSignal,
): AsyncGenerator<LogEntry | LogStreamHeartbeat, void, unknown> {
	while (!signal?.aborted) {
		await sleep(500 + Math.floor(Math.random() * 600), signal);
		const target = targets[Math.floor(Math.random() * targets.length)];
		const container = target ? getContainerById(target.id) : undefined;
		if (!container || container.state !== "running") {
			yield { type: "heartbeat" };
			continue;
		}
		const line = buildLiveLog(container);
		appendLog(container, line);
		yield { ...line, containerId: target.id, containerName: target.name };
	}
}
