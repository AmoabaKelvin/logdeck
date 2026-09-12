import type {
  ContainerInfo,
  ContainerStats,
} from "@/components/logdeck-demo/types";
import {
  compose,
  DAY,
  demoHosts,
  NOW_MS,
  seed,
  type SeedContainer,
  seedContainers,
} from "./seed-containers";

export { demoHosts };
export { demoImages, demoNetworks, demoVolumes } from "./seed-resources";

// Log types live here (not in the api shim) so the store can be imported by
// every api module without cycles; api/get-container-logs-parsed re-exports
// them under the frontend's names.
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
  containerId?: string;
  containerName?: string;
}

export interface StoredContainerRecord {
  host: string;
  name: string;
  composeProject?: string;
  image?: string;
  removed: boolean;
  logs: LogEntry[];
}

export interface ContainerResourcesRecord {
  memoryBytes: number;
  nanoCPUs: number;
  restartPolicy: { name: string; maximumRetryCount: number };
}

export interface ContainerEventRecord {
  host: string;
  containerId: string;
  containerName: string;
  action: string;
  timestamp: number;
}

// Deterministic PRNG so every visitor sees the same believable data.
function mulberry32(seed: number) {
  let a = seed;
  return () => {
    a |= 0;
    a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

const nameOf = (c: ContainerInfo) => c.names[0].slice(1);

const HTTP_ROUTES = [
  "/api/v1/containers",
  "/api/v1/containers/stats",
  "/api/v1/system/stats",
  "/api/v1/history/logs",
  "/api/v1/images",
  "/healthz",
];

function makeLine(
  kind: SeedContainer["kind"],
  _name: string,
  rand: () => number,
  index: number,
): {
  level: LogLevel;
  message: string;
  stream: "stdout" | "stderr";
  fields?: Record<string, string>;
} {
  const pick = <T>(arr: T[]) => arr[Math.floor(rand() * arr.length)];
  const roll = rand();

  switch (kind) {
    case "api": {
      if (roll < 0.45) {
        const route = pick(HTTP_ROUTES);
        const status = rand() < 0.06 ? 500 : rand() < 0.12 ? 404 : 200;
        const latency = Math.round(3 + rand() * 240);
        return {
          level: status >= 500 ? "ERROR" : "INFO",
          message: JSON.stringify({
            method: "GET",
            route,
            status,
            latencyMs: latency,
            requestId: `req-${index.toString(36)}${Math.floor(rand() * 9999)}`,
          }),
          stream: "stdout",
        };
      }
      if (roll < 0.55) {
        return {
          level: "DEBUG",
          message: `stats collector tick host=homelab containers=${24 + Math.floor(rand() * 4)} duration=${Math.round(rand() * 40)}ms`,
          stream: "stdout",
          fields: { component: "collector" },
        };
      }
      if (roll < 0.62) {
        return {
          level: "WARN",
          message: `slow docker api call op=ContainerList took=${Math.round(600 + rand() * 900)}ms`,
          stream: "stderr",
          fields: { component: "docker" },
        };
      }
      return {
        level: "INFO",
        message: `stream attached container=${pick(["marketmap-dj-postgres-1", "logdeck-states-redis-1", "edge-router"])} follow=true tail=100`,
        stream: "stdout",
      };
    }
    case "ui":
      return roll < 0.8
        ? {
            level: "INFO",
            message: `[serve] GET ${pick(["/", "/assets/index.js", "/assets/index.css", "/favicon.ico"])} 200 ${Math.round(rand() * 12)}ms`,
            stream: "stdout",
          }
        : {
            level: "DEBUG",
            message: "[serve] cache hit for /assets/index.js",
            stream: "stdout",
          };
    case "postgres": {
      if (roll < 0.12) {
        return {
          level: "WARN",
          message: `checkpoint complete: wrote ${Math.floor(rand() * 900)} buffers (${(rand() * 4).toFixed(1)}%); write=${(rand() * 30).toFixed(3)} s`,
          stream: "stderr",
        };
      }
      if (roll < 0.18) {
        return {
          level: "ERROR",
          message: `ERROR:  duplicate key value violates unique constraint "containers_pkey"`,
          stream: "stderr",
        };
      }
      return {
        level: "INFO",
        message: `LOG:  statement: ${pick([
          "SELECT * FROM logs WHERE container_id = $1 ORDER BY ts DESC LIMIT 500",
          "INSERT INTO logs (container_id, ts, level, message) VALUES ($1, $2, $3, $4)",
          "VACUUM (ANALYZE) logs",
          "SELECT pg_database_size('logdeck')",
        ])}`,
        stream: "stdout",
      };
    }
    case "clickhouse":
      return {
        level: roll < 0.1 ? "WARN" : "INFO",
        message:
          roll < 0.1
            ? `<Warning> MergeTreeBackgroundExecutor: Too many parts (${300 + Math.floor(rand() * 200)}) in table 'telemetry.events'`
            : `<Information> executeQuery: Read ${Math.floor(rand() * 900000)} rows, ${(rand() * 40).toFixed(2)} MiB in ${(rand() * 0.9).toFixed(3)} sec.`,
        stream: "stdout",
      };
    case "redis":
      return roll < 0.9
        ? {
            level: "INFO",
            message: `${Math.floor(rand() * 90)} changes in 60 seconds. Saving...`,
            stream: "stdout",
          }
        : {
            level: "WARN",
            message:
              "WARNING Memory overcommit must be enabled! Without it, a background save or replication may fail under low memory condition.",
            stream: "stderr",
          };
    case "worker": {
      if (roll < 0.2) {
        return {
          level: "ERROR",
          message: `billing job failed job=invoice-${1000 + Math.floor(rand() * 400)} err=connection refused`,
          stream: "stderr",
          fields: { queue: "billing" },
        };
      }
      return {
        level: roll < 0.35 ? "WARN" : "INFO",
        message:
          roll < 0.35
            ? `retrying job id=invoice-${1000 + Math.floor(rand() * 400)} attempt=${1 + Math.floor(rand() * 3)}`
            : `processed job id=invoice-${1000 + Math.floor(rand() * 400)} duration=${Math.round(rand() * 800)}ms`,
        stream: "stdout",
        fields: { queue: "billing" },
      };
    }
    case "ingest":
      return roll < 0.4
        ? {
            level: "ERROR",
            message: `s3 fetch failed bucket=logdeck-raw key=events/${index}.ndjson err=AccessDenied`,
            stream: "stderr",
          }
        : {
            level: "INFO",
            message: `ingested batch bucket=logdeck-raw records=${Math.floor(rand() * 5000)}`,
            stream: "stdout",
          };
    case "proxy": {
      const status = rand() < 0.04 ? 502 : 200;
      return {
        level: status >= 500 ? "ERROR" : "INFO",
        message: JSON.stringify({
          ts: index,
          logger: "http.log.access",
          msg: "handled request",
          request: {
            method: pick(["GET", "POST"]),
            uri: pick(["/", "/app", "/api/v1/logs", "/assets/app.js"]),
          },
          status,
          duration: Number((rand() * 0.6).toFixed(4)),
        }),
        stream: "stdout",
      };
    }
    case "otel":
      return {
        level: roll < 0.15 ? "WARN" : "INFO",
        message:
          roll < 0.15
            ? "Exporting failed. Will retry the request after interval. interval=5s"
            : `TracesExporter exported spans=${Math.floor(rand() * 400)} metrics=${Math.floor(rand() * 900)}`,
        stream: "stdout",
      };
    case "vector":
      return {
        level: "INFO",
        message: `vector: events_in=${Math.floor(rand() * 3000)} events_out=${Math.floor(rand() * 3000)} buffer_events=${Math.floor(rand() * 50)}`,
        stream: "stdout",
      };
    case "cron":
      return {
        level: "INFO",
        message: `cleanup pass complete removed=${Math.floor(rand() * 40)} candidates=${Math.floor(rand() * 120)}`,
        stream: "stdout",
      };
    case "minio":
      return roll < 0.25
        ? {
            level: "ERROR",
            message:
              "API: SYSTEM() health check failed, drive latency above threshold /data",
            stream: "stderr",
          }
        : {
            level: "INFO",
            message: `${pick(["s3.PutObject", "s3.GetObject", "s3.ListObjectsV2"])} bucket=logdeck-archive dur=${Math.round(rand() * 120)}ms`,
            stream: "stdout",
          };
    case "nginx":
      return {
        level: "ERROR",
        message: `[emerg] host not found in upstream "preview-app:3000" in /etc/nginx/conf.d/default.conf:12`,
        stream: "stderr",
      };
  }
}

// A JS stack trace whose lines arrive as separate UNKNOWN entries — this is
// what exercises the viewer's related-line grouping.
const STACK_LINES = [
  "at processInvoice (/app/dist/worker.js:412:19)",
  "at async Queue.run (/app/dist/queue.js:88:9)",
  "at async main (/app/dist/worker.js:31:5)",
];

function generateSeedLogs(
  container: SeedContainer,
  rand: () => number,
  options: { count: number; endMs: number; spanMs: number },
): LogEntry[] {
  const { count, endMs, spanMs } = options;
  const name = nameOf(container);
  const entries: LogEntry[] = [];
  let cursor = endMs - spanMs;
  const step = spanMs / count;

  for (let i = 0; i < count; i++) {
    cursor += step * (0.35 + rand() * 1.3);
    const at = new Date(Math.min(cursor, endMs)).toISOString();
    const line = makeLine(container.kind, name, rand, i);
    entries.push({
      timestamp: at,
      level: line.level,
      message: line.message,
      raw: line.message,
      stream: line.stream,
      fields: line.fields,
    });

    // Occasionally follow an ERROR with stack-trace continuation lines.
    if (line.level === "ERROR" && container.kind === "worker" && rand() < 0.5) {
      for (const stackLine of STACK_LINES) {
        entries.push({
          timestamp: at,
          level: "UNKNOWN",
          message: stackLine,
          raw: stackLine,
          stream: "stderr",
        });
      }
    }
  }

  return entries;
}

// ---------------------------------------------------------------------------
// Mutable state

interface DemoState {
  containers: ContainerInfo[];
  stats: ContainerStats[];
  logs: Record<string, LogEntry[]>;
  env: Record<string, Record<string, string>>;
  resources: Record<string, ContainerResourcesRecord>;
  // Log history per (host, name) — includes containers that no longer exist.
  stored: StoredContainerRecord[];
  system: { cpuPercent: number; memoryPercent: number };
  liveTick: number;
}

function buildInitialState(): DemoState {
  const containers: ContainerInfo[] = seedContainers.map(
    ({ kind: _kind, ...container }) => container,
  );

  const logs: Record<string, LogEntry[]> = {};
  const stored: StoredContainerRecord[] = [];
  for (const seed of seedContainers) {
    const generated = generateSeedLogs(
      seed,
      mulberry32(seed.id.length + seed.created),
      {
        count: seed.state === "running" ? 420 : 160,
        endMs:
          seed.state === "exited" || seed.state === "dead"
            ? NOW_MS - 27 * 60 * 1000
            : NOW_MS - 1500,
        spanMs: 6 * 60 * 60 * 1000,
      },
    );
    logs[seed.id] = generated;
    stored.push({
      host: seed.host,
      name: nameOf(seed),
      composeProject: seed.labels?.["com.docker.compose.project"],
      image: seed.image,
      removed: false,
      logs: generated,
    });
  }

  // A container that no longer exists on the host but still has stored logs:
  // the dashboard's "Removed" state, purge-history flow, and history-only
  // log browsing all hang off this record.
  const removedSeed = seed(
    {
      name: "logdeck-states-analytics-worker-1",
      image: "ghcr.io/logdeck/worker:1.3.9",
      command: "./worker analytics",
      age: 6 * DAY,
      state: "exited",
      status: "Removed",
      labels: compose("logdeck-states", "analytics-worker"),
      kind: "worker",
    },
    98,
  );
  stored.push({
    host: removedSeed.host,
    name: nameOf(removedSeed),
    composeProject: "logdeck-states",
    image: removedSeed.image,
    removed: true,
    logs: generateSeedLogs(removedSeed, mulberry32(99), {
      count: 260,
      endMs: NOW_MS - 26 * 60 * 60 * 1000,
      spanMs: 5 * 60 * 60 * 1000,
    }),
  });

  const MEMORY_LIMITS_MB = [512, 1024, 2048, 4096, 8192];
  const stats: ContainerStats[] = containers.map((container, idx) => {
    const memory_limit =
      MEMORY_LIMITS_MB[idx % MEMORY_LIMITS_MB.length] * 1024 * 1024;
    // A few sit near their ceiling so the meter's warn and fail tones show.
    const memory_percent =
      container.state === "running" ? 6 + ((idx * 17) % 88) : 0;
    return {
      id: container.id,
      host: container.host,
      cpu_percent: container.state === "running" ? 1 + ((idx * 11) % 35) : 0,
      memory_percent,
      memory_used: Math.round((memory_percent / 100) * memory_limit),
      memory_limit,
    };
  });

  const env: Record<string, Record<string, string>> = Object.fromEntries(
    containers.map((c, idx) => [
      c.id,
      {
        NODE_ENV: "production",
        LOG_LEVEL: idx % 3 === 0 ? "debug" : "info",
        REGION: c.host,
        FEATURE_FLAGS: "containers,logs,metrics",
        BUILD_SHA: `a1b2c3d${idx}`,
      },
    ]),
  );

  const resources: Record<string, ContainerResourcesRecord> =
    Object.fromEntries(
      containers.map((c, idx) => [
        c.id,
        {
          memoryBytes: stats[idx].memory_limit,
          nanoCPUs: idx % 3 === 0 ? 2_000_000_000 : 0,
          restartPolicy: { name: "unless-stopped", maximumRetryCount: 0 },
        },
      ]),
    );

  return {
    containers,
    stats,
    logs,
    env,
    resources,
    stored,
    system: { cpuPercent: 34, memoryPercent: 41 },
    liveTick: 0,
  };
}

export const state: DemoState = buildInitialState();

const kindById = new Map<string, SeedContainer["kind"]>(
  seedContainers.map((seed) => [seed.id, seed.kind]),
);

export function getContainerById(id: string): ContainerInfo | undefined {
  // Callers pass either the full id or the name-based identifier.
  return state.containers.find(
    (c) => c.id === id || nameOf(c) === id || c.names.includes(`/${id}`),
  );
}

export function containerName(container: ContainerInfo): string {
  return nameOf(container);
}

export function storedRecordFor(
  container: ContainerInfo,
): StoredContainerRecord | undefined {
  return state.stored.find(
    (record) =>
      record.host === container.host && record.name === nameOf(container),
  );
}

// ---------------------------------------------------------------------------
// Live log generation

const liveRand = mulberry32(NOW_MS % 100_000);

export function buildLiveLog(container: ContainerInfo): LogEntry {
  state.liveTick += 1;
  const kind = kindById.get(container.id) ?? "worker";
  const line = makeLine(kind, nameOf(container), liveRand, state.liveTick);
  return {
    timestamp: new Date().toISOString(),
    level: line.level,
    message: line.message,
    raw: line.message,
    stream: line.stream,
    fields: line.fields,
  };
}

export function appendLog(container: ContainerInfo, entry: LogEntry): void {
  const entries = state.logs[container.id] ?? [];
  entries.push(entry);
  state.logs[container.id] = entries.slice(-3000);
  const record = storedRecordFor(container);
  if (record) {
    record.logs.push(entry);
    record.logs = record.logs.slice(-3000);
  }
}

// ---------------------------------------------------------------------------
// Container events (pub/sub so the dashboard reacts to demo actions)

type EventListener = (event: ContainerEventRecord) => void;
const eventListeners = new Set<EventListener>();

export function subscribeContainerEvents(listener: EventListener): () => void {
  eventListeners.add(listener);
  return () => eventListeners.delete(listener);
}

export function emitContainerEvent(
  container: ContainerInfo,
  action: string,
): void {
  const event: ContainerEventRecord = {
    host: container.host,
    containerId: container.id,
    containerName: nameOf(container),
    action,
    timestamp: Math.floor(Date.now() / 1000),
  };
  for (const listener of eventListeners) {
    listener(event);
  }
}

export function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise<void>((resolve, reject) => {
    const timer = setTimeout(() => {
      signal?.removeEventListener("abort", onAbort);
      resolve();
    }, ms);

    const onAbort = () => {
      clearTimeout(timer);
      reject(new DOMException("aborted", "AbortError"));
    };

    signal?.addEventListener("abort", onAbort, { once: true });
  });
}

// ---------------------------------------------------------------------------
// Static resource seeds

export function seedStatsHistory(): Record<string, number[]> {
  const history: Record<string, number[]> = {};
  for (const stat of state.stats) {
    const container = getContainerById(stat.id);
    if (!container || container.state !== "running") continue;
    const rand = mulberry32(stat.id.charCodeAt(stat.id.length - 1) * 7919);
    let cpu = stat.cpu_percent;
    const samples: number[] = [];
    for (let i = 0; i < 40; i++) {
      cpu = Math.max(0.5, Math.min(96, cpu + (rand() - 0.5) * 6));
      samples.push(cpu);
    }
    history[stat.id] = samples;
  }
  return history;
}

export function seedSystemHistory(): {
  cpuPercent: number;
  memoryPercent: number;
}[] {
  const rand = mulberry32(4242);
  let cpu = state.system.cpuPercent;
  let mem = state.system.memoryPercent;
  const samples: { cpuPercent: number; memoryPercent: number }[] = [];
  for (let i = 0; i < 40; i++) {
    cpu = Math.max(4, Math.min(92, cpu + (rand() - 0.5) * 8));
    mem = Math.max(10, Math.min(90, mem + (rand() - 0.5) * 4));
    samples.push({ cpuPercent: cpu, memoryPercent: mem });
  }
  return samples;
}
