import { authenticatedFetch } from "@/lib/api-client";
import { iterateNDJSONStream } from "@/lib/ndjson";
import { API_BASE_URL } from "@/types/api";

const BASE_URL = `${API_BASE_URL}/api/v1/containers`;

export type LogLevel =
  | "TRACE"
  | "DEBUG"
  | "INFO"
  | "WARN"
  | "WARNING"
  | "ERROR"
  | "FATAL"
  | "PANIC"
  | "UNKNOWN";

export interface LogEntry {
  timestamp?: string;
  level: LogLevel;
  message?: string;
  stream?: "stdout" | "stderr";
  raw?: string;
  fields?: Record<string, string>;
  continuationCount?: number;
}

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
  value: unknown
): value is LogStreamHeartbeat {
  return (
    typeof value === "object" &&
    value !== null &&
    (value as { type?: unknown }).type === "heartbeat"
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

const DEFAULT_OPTIONS: Required<
  Pick<ContainerLogsOptions, "tail" | "details" | "stdout" | "stderr">
> = {
  tail: "100",
  details: false,
  stdout: true,
  stderr: true,
};

function buildLogsUrl(id: string, host: string, options?: ContainerLogsOptions) {
  const query = new URLSearchParams();
  const merged: ContainerLogsOptions = {
    ...DEFAULT_OPTIONS,
    ...options,
  };

  query.set("host", host);

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

  const path = `${BASE_URL}/${encodeURIComponent(id)}/logs/parsed`;
  const queryString = query.toString();
  return queryString ? `${path}?${queryString}` : path;
}

export async function getContainerLogsParsed(
  id: string,
  host: string,
  options?: ContainerLogsOptions
): Promise<LogEntry[]> {
  const response = await authenticatedFetch(buildLogsUrl(id, host, options), {
    headers: {
      Accept: "application/json",
    },
  });

  if (!response.ok) {
    const message = await response.text();
    throw new Error(message || `Failed to fetch logs for container ${id}`);
  }

  const data: ContainerLogsParsedResponse = await response.json();
  return data.logs || [];
}

export async function* streamContainerLogsParsed(
  id: string,
  host: string,
  options?: ContainerLogsOptions,
  signal?: AbortSignal
): AsyncGenerator<LogEntry | LogStreamHeartbeat, void, unknown> {
  const streamOptions = { ...options, follow: true };
  const response = await authenticatedFetch(buildLogsUrl(id, host, streamOptions), {
    headers: {
      Accept: "application/x-ndjson",
    },
    signal,
  });

  if (!response.ok) {
    const message = await response.text();
    throw new Error(
      message || `Failed to stream logs for container ${id}`
    );
  }

  if (!response.body) {
    throw new Error("Streaming is not supported in this environment.");
  }

  for await (const entry of iterateNDJSONStream<LogEntry | LogStreamHeartbeat>(
    response.body,
    signal
  )) {
    yield entry;
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
  entries: TLogEntry[]
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

  const message = (entry.message ?? entry.raw ?? "").trim();
  const previousMessage = (previous.message ?? previous.raw ?? "").trim();
  if (!message || !previousMessage) return false;

  if (STRUCTURED_FIELD_REGEX.test(message)) return true;

  return isProblemLevel(previous.level) && isStackTraceContinuation(message);
}

function appendContinuationLogEntry<TLogEntry extends LogEntry>(
  entry: TLogEntry,
  continuation: LogEntry
): TLogEntry {
  const message = (continuation.message ?? continuation.raw ?? "").trim();
  const raw = continuation.raw?.trim();
  const fields = { ...(entry.fields ?? {}) };
  const fieldMatch = message.match(STRUCTURED_FIELD_REGEX);

  if (fieldMatch) {
    fields[fieldMatch[1]] = fieldMatch[2].trim();
  }

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
