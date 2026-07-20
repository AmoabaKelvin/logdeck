import {
	appendLog,
	buildLiveLog,
	getContainerById,
	sleep,
	state,
	storedRecordFor,
} from "@/lib/logdeck-demo/store";
import type { LogEntry, LogLevel } from "@/lib/logdeck-demo/store";

export type { LogEntry, LogLevel };

export interface ContainerLogsParsedResponse {
	logs: LogEntry[];
	count: number;
}

// Liveness line the server writes on quiet follow streams. Never displayed;
// consumers use it to tell a quiet stream from a dead one.
export interface LogStreamHeartbeat {
	type: "heartbeat";
}

export function isLogStreamHeartbeat(
	value: unknown,
): value is LogStreamHeartbeat {
	return (
		typeof value === "object" &&
		value !== null &&
		"type" in value &&
		value.type === "heartbeat"
	);
}

export interface ContainerLogsOptions {
	since?: string;
	until?: string;
	tail?: string | number;
	details?: boolean;
	stdout?: boolean;
	stderr?: boolean;
	follow?: boolean;
	search?: string;
}

function resolveLogs(id: string): LogEntry[] {
	const container = getContainerById(id);
	if (container) {
		if (state.logs[container.id]) return state.logs[container.id];
		const record = storedRecordFor(container);
		return record?.logs ?? [];
	}
	// Removed containers are addressed by name; their logs only exist in the
	// history store.
	const record = state.stored.find(
		(r) => r.name === id || `/${r.name}` === id,
	);
	return record?.logs ?? [];
}

function applyOptions(
	logs: LogEntry[],
	options?: ContainerLogsOptions,
): LogEntry[] {
	let result = logs;

	if (options?.since) {
		const since = new Date(options.since).getTime();
		if (!Number.isNaN(since)) {
			result = result.filter(
				(entry) =>
					!entry.timestamp || new Date(entry.timestamp).getTime() >= since,
			);
		}
	}
	if (options?.until) {
		const until = new Date(options.until).getTime();
		if (!Number.isNaN(until)) {
			result = result.filter(
				(entry) =>
					!entry.timestamp || new Date(entry.timestamp).getTime() <= until,
			);
		}
	}
	if (options?.search) {
		const needle = options.search.toLowerCase();
		result = result.filter((entry) =>
			(entry.message ?? entry.raw ?? "").toLowerCase().includes(needle),
		);
	}

	const tail = Number(options?.tail ?? 100);
	if (Number.isFinite(tail) && tail > 0 && result.length > tail) {
		result = result.slice(result.length - tail);
	}

	return result;
}

export async function getContainerLogsParsed(
	id: string,
	_host: string,
	options?: ContainerLogsOptions,
): Promise<LogEntry[]> {
	return structuredClone(applyOptions(resolveLogs(id), options));
}

export async function* streamContainerLogsParsed(
	id: string,
	_host: string,
	_options?: ContainerLogsOptions,
	signal?: AbortSignal,
): AsyncGenerator<LogEntry | LogStreamHeartbeat, void, unknown> {
	while (!signal?.aborted) {
		await sleep(700 + Math.floor(Math.random() * 700), signal);
		const container = getContainerById(id);
		if (!container || container.state !== "running") {
			yield { type: "heartbeat" };
			continue;
		}
		const line = buildLiveLog(container);
		appendLog(container, line);
		yield line;
	}
}

export function getLogLevelBadgeColor(level: LogLevel | undefined): string {
	switch (level ?? "UNKNOWN") {
		case "TRACE":
		case "DEBUG":
			return "bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300";
		case "INFO":
			return "bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300";
		case "WARN":
		case "WARNING":
			return "bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300";
		case "ERROR":
			return "bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300";
		case "FATAL":
		case "PANIC":
			return "bg-red-200 text-red-900 dark:bg-red-950 dark:text-red-200 font-semibold";
		default:
			return "bg-muted text-muted-foreground";
	}
}

const STRUCTURED_FIELD_REGEX = /^([A-Za-z_][A-Za-z0-9_.-]*)\s*[:=]\s*(.+)$/;
const STACK_TRACE_PREFIXES = [
	"at ",
	"File ",
	"Traceback ",
	"Caused by:",
	"... ",
	"goroutine ",
];

export function groupRelatedLogEntries<TLogEntry extends LogEntry>(
	entries: TLogEntry[],
): TLogEntry[] {
	const grouped: TLogEntry[] = [];

	for (const entry of entries) {
		const previous = grouped.at(-1);
		if (previous && isContinuationLogEntry(entry, previous)) {
			grouped[grouped.length - 1] = appendContinuationLogEntry(previous, entry);
			continue;
		}

		grouped.push(entry);
	}

	return grouped;
}

function isContinuationLogEntry(entry: LogEntry, previous: LogEntry): boolean {
	if (entry.level !== "UNKNOWN") return false;
	// Aggregate streams interleave containers; never fold a line into another
	// container's entry.
	if (entry.containerName !== previous.containerName) return false;

	const message = (entry.message ?? entry.raw ?? "").trim();
	const previousMessage = (previous.message ?? previous.raw ?? "").trim();
	if (!message || !previousMessage) return false;

	if (STRUCTURED_FIELD_REGEX.test(message)) return true;

	return isProblemLevel(previous.level) && isStackTraceContinuation(message);
}

function appendContinuationLogEntry<TLogEntry extends LogEntry>(
	entry: TLogEntry,
	continuation: LogEntry,
): TLogEntry {
	const message = (continuation.message ?? continuation.raw ?? "").trim();
	const raw = continuation.raw?.trim();
	const fields = { ...(entry.fields ?? {}) };
	const fieldMatch = message.match(STRUCTURED_FIELD_REGEX);

	if (fieldMatch) {
		fields[fieldMatch[1]] = fieldMatch[2].trim();
	}

	// Cast needed: spreading a generic and overriding base fields is not
	// assignable back to TLogEntry as far as the compiler can prove.
	return {
		...entry,
		message: [entry.message, message].filter(Boolean).join("\n"),
		raw: [entry.raw, raw].filter(Boolean).join("\n"),
		fields: Object.keys(fields).length > 0 ? fields : entry.fields,
		continuationCount: (entry.continuationCount ?? 0) + 1,
	} as TLogEntry;
}

function isProblemLevel(level: LogLevel | undefined): boolean {
	return (
		level === "WARN" ||
		level === "WARNING" ||
		level === "ERROR" ||
		level === "FATAL" ||
		level === "PANIC"
	);
}

function isStackTraceContinuation(message: string): boolean {
	return (
		STACK_TRACE_PREFIXES.some((prefix) => message.startsWith(prefix)) ||
		(message.startsWith("/") && message.includes(":"))
	);
}
