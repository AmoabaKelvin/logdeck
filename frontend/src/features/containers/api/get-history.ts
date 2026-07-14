import { authenticatedFetch } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";
import type { LogEntry, LogLevel } from "./get-container-logs-parsed";

const BASE_URL = `${API_BASE_URL}/api/v1/history`;

export interface HistoryStatus {
	enabled: boolean;
	// Only present while persistence is enabled.
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
	// Cursor for the next (older) page. Empty or absent means the store has no
	// more history before this page.
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

async function readError(response: Response, fallback: string): Promise<Error> {
	const body = await response.text();
	try {
		const parsed = JSON.parse(body) as { error?: string };
		if (parsed.error) return new Error(parsed.error);
	} catch {
		// Not a JSON error envelope; fall through to the raw body.
	}
	return new Error(body || fallback);
}

export async function getHistoryStatus(): Promise<HistoryStatus> {
	const response = await authenticatedFetch(`${BASE_URL}/status`, {
		headers: { Accept: "application/json" },
	});

	if (!response.ok) {
		throw await readError(response, "Failed to fetch history status");
	}

	return response.json();
}

export async function getHistoryContainers(): Promise<StoredContainer[]> {
	const response = await authenticatedFetch(`${BASE_URL}/containers`, {
		headers: { Accept: "application/json" },
	});

	if (!response.ok) {
		throw await readError(response, "Failed to fetch stored containers");
	}

	const data: { containers: StoredContainer[] } = await response.json();
	return data.containers ?? [];
}

export interface DeleteHistoryResult {
	message: string;
	linesDeleted: number;
}

// Permanently drops every stored log line for a container. The server rejects
// this for read-scoped tokens and in read-only mode, so callers must surface
// the error rather than assume the rows are gone.
export async function deleteHistoryContainer(
	name: string,
	host: string,
): Promise<DeleteHistoryResult> {
	const query = new URLSearchParams({ host });
	const response = await authenticatedFetch(
		`${BASE_URL}/containers/${encodeURIComponent(name)}?${query.toString()}`,
		{ method: "DELETE", headers: { Accept: "application/json" } },
	);

	if (!response.ok) {
		throw await readError(response, `Failed to delete stored logs for ${name}`);
	}

	const data: Partial<DeleteHistoryResult> = await response.json();
	return {
		message: data.message ?? `Deleted stored logs for ${name}`,
		linesDeleted: data.linesDeleted ?? 0,
	};
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
	const query = new URLSearchParams();
	query.set("container", container);
	if (host) query.set("host", host);
	if (since) query.set("since", since);
	if (until) query.set("until", until);
	if (levels && levels.length > 0) query.set("levels", levels.join(","));
	if (search) {
		query.set("search", search);
		if (regex) query.set("regex", "true");
	}
	if (limit !== undefined) query.set("limit", String(limit));
	if (cursor) query.set("cursor", cursor);

	const response = await authenticatedFetch(
		`${BASE_URL}/logs?${query.toString()}`,
		{ headers: { Accept: "application/json" } },
	);

	if (!response.ok) {
		throw await readError(
			response,
			`Failed to fetch stored logs for ${container}`,
		);
	}

	const data: HistoryLogsPage = await response.json();
	return { ...data, logs: data.logs ?? [] };
}
