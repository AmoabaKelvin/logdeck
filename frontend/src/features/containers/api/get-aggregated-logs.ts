import { authenticatedFetch, readJson } from "@/lib/api-client";
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

// One target's outcome in an aggregated read; mirrors docker.SourceCoverage.
// "stopped" and "removed" are expected; only "failed" means logs are missing.
export interface LogSource {
	host: string;
	containerId: string;
	containerName?: string;
	status: "ok" | "no_matches" | "stopped" | "removed" | "failed";
	error?: string;
}

// The line an aggregated stream writes when a source opens, fails or ends.
interface LogSourceEvent extends LogSource {
	type: "source";
}

function isLogSourceEvent(
	value: LogEntry | LogStreamHeartbeat | LogSourceEvent,
): value is LogSourceEvent {
	return "type" in value && value.type === "source";
}

type OnSource = (source: LogSource) => void;

// A read where no source could be read is a 502 whose JSON body still lists
// every source; report them before throwing.
async function aggregateError(
	response: Response,
	fallback: string,
	onSource?: OnSource,
): Promise<Error> {
	const text = await response.text();
	try {
		// SAFETY: writeAggregateError in aggregate_logs_handlers.go.
		const body = JSON.parse(text) as { error?: string; sources?: LogSource[] };
		body.sources?.forEach((source) => onSource?.(source));
		return new Error(body.error || fallback);
	} catch {
		return new Error(text || fallback);
	}
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

// onSource receives every target's outcome, including when the read fails.
export async function getAggregatedLogs(
	targets: AggregateLogTarget[],
	options?: ContainerLogsOptions,
	onSource?: OnSource,
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
		throw await aggregateError(
			response,
			"Failed to fetch aggregated logs",
			onSource,
		);
	}

	const data = await readJson<
		ContainerLogsParsedResponse & { sources?: LogSource[] }
	>(response);
	data.sources?.forEach((source) => onSource?.(source));
	return data.logs || [];
}

// onSource receives each source line instead of the caller: one per target
// when the stream opens, and another when a target's live stream ends.
export async function* streamAggregatedLogs(
	targets: AggregateLogTarget[],
	options?: ContainerLogsOptions,
	signal?: AbortSignal,
	onSource?: OnSource,
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
		throw await aggregateError(
			response,
			"Failed to stream aggregated logs",
			onSource,
		);
	}

	if (!response.body) {
		throw new Error("Streaming is not supported in this environment.");
	}

	for await (const entry of iterateNDJSONStream<
		LogEntry | LogStreamHeartbeat | LogSourceEvent
	>(response.body, signal)) {
		if (isLogSourceEvent(entry)) {
			onSource?.(entry);
			continue;
		}
		yield entry;
	}
}
