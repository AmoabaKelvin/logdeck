import { authenticatedFetch } from "@/lib/api-client";
import { iterateNDJSONStream } from "@/lib/ndjson";
import { API_BASE_URL } from "@/types/api";

import type {
	ContainerLogsOptions,
	ContainerLogsParsedResponse,
	LogEntry,
	LogStreamHeartbeat,
} from "./get-container-logs-parsed";

const AGGREGATE_URL = `${API_BASE_URL}/api/v1/logs/aggregate`;

export interface AggregateLogTarget {
	id: string;
	host: string;
	name?: string;
}

const DEFAULT_OPTIONS: Required<
	Pick<ContainerLogsOptions, "tail" | "details" | "stdout" | "stderr">
> = {
	tail: "100",
	details: false,
	stdout: true,
	stderr: true,
};

// Mirrors buildLogsUrl in get-container-logs-parsed.ts; targets are passed
// as repeated "host~id~name" triples ("~" cannot appear in container names
// or IDs).
function buildAggregateLogsUrl(
	targets: AggregateLogTarget[],
	options?: ContainerLogsOptions,
) {
	const query = new URLSearchParams();
	const merged: ContainerLogsOptions = {
		...DEFAULT_OPTIONS,
		...options,
	};

	for (const target of targets) {
		query.append("targets", `${target.host}~${target.id}~${target.name ?? ""}`);
	}

	if (merged.since) {
		query.set("since", merged.since);
	}
	if (merged.until) {
		query.set("until", merged.until);
	}
	if (merged.tail !== undefined) {
		query.set("tail", String(merged.tail));
	}
	if (merged.details !== undefined) {
		query.set("details", String(merged.details));
	}
	if (merged.stdout !== undefined) {
		query.set("stdout", String(merged.stdout));
	}
	if (merged.stderr !== undefined) {
		query.set("stderr", String(merged.stderr));
	}
	if (merged.follow !== undefined) {
		query.set("follow", String(merged.follow));
	}
	if (merged.search) {
		query.set("search", merged.search);
	}

	return `${AGGREGATE_URL}?${query.toString()}`;
}

export async function getAggregatedLogs(
	targets: AggregateLogTarget[],
	options?: ContainerLogsOptions,
): Promise<LogEntry[]> {
	const response = await authenticatedFetch(
		buildAggregateLogsUrl(targets, options),
		{
			headers: {
				Accept: "application/json",
			},
		},
	);

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to fetch aggregated logs");
	}

	const data: ContainerLogsParsedResponse = await response.json();
	return data.logs || [];
}

export async function* streamAggregatedLogs(
	targets: AggregateLogTarget[],
	options?: ContainerLogsOptions,
	signal?: AbortSignal,
): AsyncGenerator<LogEntry | LogStreamHeartbeat, void, unknown> {
	const streamOptions = { ...options, follow: true };
	const response = await authenticatedFetch(
		buildAggregateLogsUrl(targets, streamOptions),
		{
			headers: {
				Accept: "application/x-ndjson",
			},
			signal,
		},
	);

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to stream aggregated logs");
	}

	if (!response.body) {
		throw new Error("Streaming is not supported in this environment.");
	}

	for await (const entry of iterateNDJSONStream<LogEntry | LogStreamHeartbeat>(
		response.body,
		signal,
	)) {
		yield entry;
	}
}
