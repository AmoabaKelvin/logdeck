import { sleep, state } from "@/lib/logdeck-demo/store";
import type { StoredContainerRecord } from "@/lib/logdeck-demo/store";

import type { LogEntry, LogLevel } from "./get-container-logs-parsed";

export interface HistoryStatus {
	enabled: boolean;
	dbSizeBytes?: number;
	perContainerMB?: number;
	totalMB?: number;
}

// A container the log store knows about. It may no longer exist on the host
// (removed), in which case history is the only way to read its logs.
export interface StoredContainer {
	host: string;
	name: string;
	composeProject?: string;
	image?: string;
	storedBytes: number;
	oldestTs: string;
	newestTs: string;
	removed: boolean;
	excluded: boolean;
	excludedReason?: string;
}

export interface HistoryLogsPage {
	logs: LogEntry[];
	nextCursor?: string;
	count: number;
}

export interface HistoryLogsParams {
	container: string;
	host?: string;
	since?: string;
	until?: string;
	levels?: LogLevel[];
	search?: string;
	regex?: boolean;
	limit?: number;
	cursor?: string;
}

export async function getHistoryStatus(): Promise<HistoryStatus> {
	return {
		enabled: true,
		dbSizeBytes: 48 * 1024 * 1024,
		perContainerMB: 100,
		totalMB: 1024,
	};
}

function recordSize(record: StoredContainerRecord): number {
	return record.logs.reduce(
		(sum, entry) => sum + (entry.raw?.length ?? entry.message?.length ?? 0),
		0,
	);
}

export async function getHistoryContainers(): Promise<StoredContainer[]> {
	return state.stored.map((record) => ({
		host: record.host,
		name: record.name,
		composeProject: record.composeProject,
		image: record.image,
		storedBytes: recordSize(record),
		oldestTs: record.logs[0]?.timestamp ?? new Date(0).toISOString(),
		newestTs:
			record.logs[record.logs.length - 1]?.timestamp ??
			new Date(0).toISOString(),
		removed: record.removed,
		excluded: false,
	}));
}

export interface DeleteHistoryResult {
	message: string;
	linesDeleted: number;
}

export async function deleteHistoryContainer(
	name: string,
	host: string,
): Promise<DeleteHistoryResult> {
	await sleep(300);
	const index = state.stored.findIndex(
		(record) => record.name === name && record.host === host,
	);
	if (index === -1) {
		throw new Error(`No stored logs found for ${name}`);
	}

	const record = state.stored[index];
	const linesDeleted = record.logs.length;
	if (record.removed) {
		// Nothing else references a removed container; drop the record so it
		// disappears from the "Removed" state entirely.
		state.stored.splice(index, 1);
	} else {
		record.logs = [];
	}

	return { message: `Deleted stored logs for ${name}`, linesDeleted };
}

// Fetch one page of stored logs. Pagination walks backward in time: the first
// page (no cursor) is the newest, and `nextCursor` fetches older entries.
export async function getHistoryLogs({
	container,
	host,
	since,
	until,
	levels,
	search,
	regex,
	limit,
	cursor,
}: HistoryLogsParams): Promise<HistoryLogsPage> {
	const record = state.stored.find(
		(r) => r.name === container && (!host || r.host === host),
	);
	let logs = record?.logs ?? [];

	if (since) {
		const sinceMs = new Date(since).getTime();
		if (!Number.isNaN(sinceMs)) {
			logs = logs.filter(
				(entry) =>
					!entry.timestamp || new Date(entry.timestamp).getTime() >= sinceMs,
			);
		}
	}
	if (until) {
		const untilMs = new Date(until).getTime();
		if (!Number.isNaN(untilMs)) {
			logs = logs.filter(
				(entry) =>
					!entry.timestamp || new Date(entry.timestamp).getTime() <= untilMs,
			);
		}
	}
	if (levels && levels.length > 0) {
		const wanted = new Set(levels);
		logs = logs.filter((entry) => wanted.has(entry.level));
	}
	if (search) {
		if (regex) {
			let pattern: RegExp | null = null;
			try {
				pattern = new RegExp(search, "i");
			} catch {
				throw new Error(`Invalid regular expression: ${search}`);
			}
			logs = logs.filter((entry) =>
				pattern?.test(entry.message ?? entry.raw ?? ""),
			);
		} else {
			const needle = search.toLowerCase();
			logs = logs.filter((entry) =>
				(entry.message ?? entry.raw ?? "").toLowerCase().includes(needle),
			);
		}
	}

	const pageSize = limit && limit > 0 ? limit : 500;
	// The cursor is the exclusive end index of the next-older page.
	const end = cursor ? Number(cursor) : logs.length;
	const safeEnd = Number.isFinite(end)
		? Math.max(0, Math.min(end, logs.length))
		: logs.length;
	const start = Math.max(0, safeEnd - pageSize);
	const page = logs.slice(start, safeEnd);

	return {
		logs: structuredClone(page),
		nextCursor: start > 0 ? String(start) : undefined,
		count: page.length,
	};
}
