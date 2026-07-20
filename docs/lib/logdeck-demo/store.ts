import type {
	ContainerInfo,
	ContainerStats,
	DockerHost,
	ImageInfo,
	NetworkInfo,
	VolumeInfo,
} from "@/components/logdeck-demo/types";

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

const NOW_MS = Date.now();
const NOW_SEC = Math.floor(NOW_MS / 1000);

export const demoHosts: DockerHost[] = [
	{ name: "local-dev", host: "unix:///var/run/docker.sock" },
	{ name: "staging-eu", host: "tcp://10.0.2.14:2375" },
	{ name: "edge-us", host: "tcp://10.0.8.9:2375" },
];

interface SeedContainer extends ContainerInfo {
	// Which log flavor the generator uses for this container.
	kind:
		| "api"
		| "ui"
		| "postgres"
		| "redis"
		| "worker"
		| "ingest"
		| "proxy"
		| "otel"
		| "vector"
		| "cron"
		| "minio"
		| "nginx";
}

const seedContainers: SeedContainer[] = [
	{
		id: "c7f4f9b13c2111a5f47dba1a00001",
		names: ["/logdeck-api-1"],
		image: "ghcr.io/logdeck/api:0.14.2",
		image_id: "sha256:1a7c",
		command: "./logdeck-server --port 8080",
		created: NOW_SEC - 60 * 60 * 36,
		state: "running",
		status: "Up 36 hours (healthy)",
		health: "healthy",
		labels: {
			"com.docker.compose.project": "logdeck",
			"com.docker.compose.service": "api",
		},
		host: "local-dev",
		kind: "api",
	},
	{
		id: "c7f4f9b13c2111a5f47dba1a00002",
		names: ["/logdeck-ui-1"],
		image: "ghcr.io/logdeck/ui:0.14.2",
		image_id: "sha256:2b81",
		command: "bun run serve",
		created: NOW_SEC - 60 * 60 * 32,
		state: "running",
		status: "Up 32 hours",
		labels: {
			"com.docker.compose.project": "logdeck",
			"com.docker.compose.service": "ui",
		},
		host: "local-dev",
		kind: "ui",
	},
	{
		id: "c7f4f9b13c2111a5f47dba1a00003",
		names: ["/postgres-main"],
		image: "postgres:16-alpine",
		image_id: "sha256:3c55",
		command: "docker-entrypoint.sh postgres",
		created: NOW_SEC - 60 * 60 * 24 * 4,
		state: "running",
		status: "Up 4 days (healthy)",
		health: "healthy",
		labels: { "com.docker.compose.project": "platform" },
		host: "local-dev",
		kind: "postgres",
	},
	{
		id: "c7f4f9b13c2111a5f47dba1a00004",
		names: ["/redis-cache"],
		image: "redis:7-alpine",
		image_id: "sha256:4d19",
		command: "redis-server --save 60 1000",
		created: NOW_SEC - 60 * 60 * 18,
		state: "running",
		status: "Up 18 hours (healthy)",
		health: "healthy",
		labels: { "com.docker.compose.project": "platform" },
		host: "staging-eu",
		kind: "redis",
	},
	{
		id: "c7f4f9b13c2111a5f47dba1a00005",
		names: ["/worker-billing-1"],
		image: "ghcr.io/logdeck/worker:0.14.2",
		image_id: "sha256:5e42",
		command: "./worker billing",
		created: NOW_SEC - 60 * 60 * 9,
		state: "restarting",
		status: "Restarting (1) 8 seconds ago",
		labels: { "com.docker.compose.project": "jobs" },
		host: "staging-eu",
		kind: "worker",
	},
	{
		id: "c7f4f9b13c2111a5f47dba1a00006",
		names: ["/ingest-s3"],
		image: "ghcr.io/logdeck/ingest:0.13.8",
		image_id: "sha256:6f77",
		command: "./ingest --provider s3",
		created: NOW_SEC - 60 * 60 * 2,
		state: "exited",
		status: "Exited (137) 27 minutes ago",
		labels: { "com.docker.compose.project": "jobs" },
		host: "staging-eu",
		kind: "ingest",
	},
	{
		id: "c7f4f9b13c2111a5f47dba1a00007",
		names: ["/caddy-edge"],
		image: "caddy:2.9",
		image_id: "sha256:7a03",
		command: "caddy run --config /etc/caddy/Caddyfile",
		created: NOW_SEC - 60 * 60 * 24,
		state: "running",
		status: "Up 24 hours (healthy)",
		health: "healthy",
		labels: { "com.docker.compose.project": "edge" },
		host: "edge-us",
		kind: "proxy",
	},
	{
		id: "c7f4f9b13c2111a5f47dba1a00008",
		names: ["/otel-collector"],
		image: "otel/opentelemetry-collector:0.100.0",
		image_id: "sha256:8b64",
		command: "/otelcol --config=/etc/otelcol/config.yaml",
		created: NOW_SEC - 60 * 60 * 12,
		state: "paused",
		status: "Up 12 hours (Paused)",
		labels: { "com.docker.compose.project": "observability" },
		host: "edge-us",
		kind: "otel",
	},
	{
		id: "c7f4f9b13c2111a5f47dba1a00009",
		names: ["/vector-agent"],
		image: "timberio/vector:0.40.0-alpine",
		image_id: "sha256:9c28",
		command: "vector --config /etc/vector/vector.toml",
		created: NOW_SEC - 60 * 60 * 20,
		state: "running",
		status: "Up 20 hours",
		labels: { "com.docker.compose.project": "observability" },
		host: "edge-us",
		kind: "vector",
	},
	{
		id: "c7f4f9b13c2111a5f47dba1a00010",
		names: ["/job-cleanup"],
		image: "ghcr.io/logdeck/cron:0.14.2",
		image_id: "sha256:a1f6",
		command: "./cron cleanup",
		created: NOW_SEC - 60 * 30,
		state: "exited",
		status: "Exited (0) 3 minutes ago",
		labels: { "com.docker.compose.project": "jobs" },
		host: "local-dev",
		kind: "cron",
	},
	{
		id: "c7f4f9b13c2111a5f47dba1a00011",
		names: ["/minio"],
		image: "minio/minio:RELEASE.2025-01-20",
		image_id: "sha256:b2d0",
		command: "minio server /data --console-address :9001",
		created: NOW_SEC - 60 * 60 * 48,
		state: "running",
		status: "Up 2 days (unhealthy)",
		health: "unhealthy",
		labels: { "com.docker.compose.project": "platform" },
		host: "staging-eu",
		kind: "minio",
	},
	{
		id: "c7f4f9b13c2111a5f47dba1a00012",
		names: ["/nginx-preview"],
		image: "nginx:1.27-alpine",
		image_id: "sha256:c3e9",
		command: "nginx -g 'daemon off;'",
		created: NOW_SEC - 60 * 60 * 7,
		state: "dead",
		status: "Dead",
		labels: { "com.docker.compose.project": "preview" },
		host: "local-dev",
		kind: "nginx",
	},
];

// ---------------------------------------------------------------------------
// Log generation

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
): { level: LogLevel; message: string; stream: "stdout" | "stderr"; fields?: Record<string, string> } {
	const pick = <T,>(arr: T[]) => arr[Math.floor(rand() * arr.length)];
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
					message: `stats collector tick host=local-dev containers=${8 + Math.floor(rand() * 4)} duration=${Math.round(rand() * 40)}ms`,
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
				message: `stream attached container=${pick(["postgres-main", "redis-cache", "caddy-edge"])} follow=true tail=100`,
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
					request: { method: pick(["GET", "POST"]), uri: pick(["/", "/app", "/api/v1/logs", "/assets/app.js"]) },
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
						message: "API: SYSTEM() health check failed, drive latency above threshold /data",
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
		const generated = generateSeedLogs(seed, mulberry32(seed.id.length + seed.created), {
			count: seed.state === "running" ? 420 : 160,
			endMs:
				seed.state === "exited" || seed.state === "dead"
					? NOW_MS - 27 * 60 * 1000
					: NOW_MS - 1500,
			spanMs: 6 * 60 * 60 * 1000,
		});
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
	const removedSeed: SeedContainer = {
		id: "c7f4f9b13c2111a5f47dba1a00099",
		names: ["/analytics-worker"],
		image: "ghcr.io/logdeck/worker:0.13.9",
		image_id: "sha256:dead",
		command: "./worker analytics",
		created: NOW_SEC - 60 * 60 * 24 * 6,
		state: "exited",
		status: "Removed",
		labels: { "com.docker.compose.project": "jobs" },
		host: "local-dev",
		kind: "worker",
	};
	stored.push({
		host: removedSeed.host,
		name: nameOf(removedSeed),
		composeProject: "jobs",
		image: removedSeed.image,
		removed: true,
		logs: generateSeedLogs(removedSeed, mulberry32(99), {
			count: 260,
			endMs: NOW_MS - 26 * 60 * 60 * 1000,
			spanMs: 5 * 60 * 60 * 1000,
		}),
	});

	const stats: ContainerStats[] = containers.map((container, idx) => ({
		id: container.id,
		host: container.host,
		cpu_percent: container.state === "running" ? 2 + ((idx * 11) % 35) : 0,
		memory_percent: container.state === "running" ? 8 + ((idx * 7) % 52) : 0,
		memory_used:
			container.state === "running" ? (180 + idx * 43) * 1024 * 1024 : 0,
		memory_limit: 2048 * 1024 * 1024,
	}));

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

	const resources: Record<string, ContainerResourcesRecord> = Object.fromEntries(
		containers.map((c, idx) => [
			c.id,
			{
				memoryBytes: idx % 2 === 0 ? 2048 * 1024 * 1024 : 0,
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

export const demoImages: ImageInfo[] = [
	...seedContainers.map((seed, idx) => ({
		id: `${seed.image_id}${"0".repeat(8)}${idx.toString(16)}`,
		repo_tags: [seed.image],
		size: (85 + idx * 37) * 1024 * 1024,
		created: seed.created - 60 * 60 * 24 * 3,
		host: seed.host,
	})),
	{
		id: "sha256:f00ddead0000beef",
		repo_tags: null,
		size: 412 * 1024 * 1024,
		created: NOW_SEC - 60 * 60 * 24 * 21,
		host: "local-dev",
	},
];

export const demoVolumes: VolumeInfo[] = [
	{
		name: "postgres-data",
		driver: "local",
		mountpoint: "/var/lib/docker/volumes/postgres-data/_data",
		created: new Date((NOW_SEC - 60 * 60 * 24 * 30) * 1000).toISOString(),
		labels: { "com.docker.compose.project": "platform" },
		host: "local-dev",
	},
	{
		name: "logdeck-history",
		driver: "local",
		mountpoint: "/var/lib/docker/volumes/logdeck-history/_data",
		created: new Date((NOW_SEC - 60 * 60 * 24 * 12) * 1000).toISOString(),
		labels: { "com.docker.compose.project": "logdeck" },
		host: "local-dev",
	},
	{
		name: "minio-data",
		driver: "local",
		mountpoint: "/var/lib/docker/volumes/minio-data/_data",
		created: new Date((NOW_SEC - 60 * 60 * 24 * 45) * 1000).toISOString(),
		host: "staging-eu",
	},
	{
		name: "redis-data",
		driver: "local",
		mountpoint: "/var/lib/docker/volumes/redis-data/_data",
		created: new Date((NOW_SEC - 60 * 60 * 20) * 1000).toISOString(),
		host: "staging-eu",
	},
	{
		name: "caddy-config",
		driver: "local",
		mountpoint: "/var/lib/docker/volumes/caddy-config/_data",
		created: new Date((NOW_SEC - 60 * 60 * 24 * 8) * 1000).toISOString(),
		host: "edge-us",
	},
];

export const demoNetworks: NetworkInfo[] = [
	{
		id: "9f1c44aa10b2",
		name: "bridge",
		driver: "bridge",
		scope: "local",
		subnets: ["172.17.0.0/16"],
		host: "local-dev",
	},
	{
		id: "1d2e9b00c4a7",
		name: "logdeck_default",
		driver: "bridge",
		scope: "local",
		subnets: ["172.21.0.0/16"],
		host: "local-dev",
	},
	{
		id: "77aa0e91d3f5",
		name: "platform_default",
		driver: "bridge",
		scope: "local",
		subnets: ["172.22.0.0/16"],
		host: "staging-eu",
	},
	{
		id: "b8c1f2a97e60",
		name: "edge_public",
		driver: "overlay",
		scope: "swarm",
		subnets: ["10.0.9.0/24"],
		host: "edge-us",
	},
	{
		id: "40d6c7e8aa19",
		name: "host",
		driver: "host",
		scope: "local",
		host: "edge-us",
	},
];

// Pre-baked sparkline history so trend lines are visible on first paint
// instead of only after a minute of polling.
export function seedStatsHistory(): Record<
	string,
	{ cpu: number; memoryPercent: number }[]
> {
	const history: Record<string, { cpu: number; memoryPercent: number }[]> = {};
	for (const stat of state.stats) {
		const container = getContainerById(stat.id);
		if (!container || container.state !== "running") continue;
		const rand = mulberry32(stat.id.charCodeAt(stat.id.length - 1) * 7919);
		let cpu = stat.cpu_percent;
		let mem = stat.memory_percent;
		const samples: { cpu: number; memoryPercent: number }[] = [];
		for (let i = 0; i < 40; i++) {
			cpu = Math.max(0.5, Math.min(96, cpu + (rand() - 0.5) * 6));
			mem = Math.max(1, Math.min(95, mem + (rand() - 0.5) * 3));
			samples.push({ cpu, memoryPercent: mem });
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
