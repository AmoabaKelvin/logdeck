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
  value: LogEntry | LogStreamHeartbeat,
): value is LogStreamHeartbeat {
  return "type" in value && value.type === "heartbeat";
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
  const record = state.stored.find((r) => r.name === id || `/${r.name}` === id);
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
  // Aggregate streams interleave containers; never fold a line into another
  // container's entry.
  return (
    entry.continuation === true &&
    entry.containerName === previous.containerName
  );
}

function appendContinuationLogEntry<TLogEntry extends LogEntry>(
  entry: TLogEntry,
  continuation: LogEntry,
): TLogEntry {
  const message = (continuation.message ?? continuation.raw ?? "").trim();
  const raw = continuation.raw?.trim();
  const fields = { ...entry.fields };
  const fieldMatch = message.match(STRUCTURED_FIELD_REGEX);

  if (fieldMatch) {
    fields[fieldMatch[1]] = fieldMatch[2].trim();
  }

  // SAFETY: the spread keeps every TLogEntry property and only base LogEntry
  // fields are overridden, with values of their LogEntry types. TLogEntry only
  // ever adds fields to LogEntry, so the result is still a TLogEntry.
  return {
    ...entry,
    message: [entry.message, message].filter(Boolean).join("\n"),
    raw: [entry.raw, raw].filter(Boolean).join("\n"),
    fields: Object.keys(fields).length > 0 ? fields : entry.fields,
    continuationCount: (entry.continuationCount ?? 0) + 1,
  } as TLogEntry;
}
