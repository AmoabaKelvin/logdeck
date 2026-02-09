import type { ContainerInfo, ContainerStats, DockerHost } from "@/components/logdeck-demo/containers/types";
import type { LogEntry, LogLevel } from "@/types/logs";

const NOW_SEC = Math.floor(Date.now() / 1000);

export const demoHosts: DockerHost[] = [
  { Name: "local-dev", Host: "unix:///var/run/docker.sock" },
  { Name: "staging-eu", Host: "tcp://10.0.2.14:2375" },
  { Name: "edge-us", Host: "tcp://10.0.8.9:2375" },
];

export const demoContainers: ContainerInfo[] = [
  {
    id: "c7f4f9b13c2111a5f47dba1a00001",
    names: ["/logdeck-api-1"],
    image: "ghcr.io/logdeck/api:0.14.2",
    image_id: "sha256:1",
    command: "./logdeck-server --port 8080",
    created: NOW_SEC - 60 * 60 * 36,
    state: "running",
    status: "Up 36 hours",
    labels: { "com.docker.compose.project": "logdeck" },
    host: "local-dev",
  },
  {
    id: "c7f4f9b13c2111a5f47dba1a00002",
    names: ["/logdeck-ui-1"],
    image: "ghcr.io/logdeck/ui:0.14.2",
    image_id: "sha256:2",
    command: "bun run serve",
    created: NOW_SEC - 60 * 60 * 32,
    state: "running",
    status: "Up 32 hours",
    labels: { "com.docker.compose.project": "logdeck" },
    host: "local-dev",
  },
  {
    id: "c7f4f9b13c2111a5f47dba1a00003",
    names: ["/postgres-main"],
    image: "postgres:16-alpine",
    image_id: "sha256:3",
    command: "docker-entrypoint.sh postgres",
    created: NOW_SEC - 60 * 60 * 24 * 4,
    state: "running",
    status: "Up 4 days",
    labels: { "com.docker.compose.project": "platform" },
    host: "local-dev",
  },
  {
    id: "c7f4f9b13c2111a5f47dba1a00004",
    names: ["/redis-cache"],
    image: "redis:7-alpine",
    image_id: "sha256:4",
    command: "redis-server --save 60 1000",
    created: NOW_SEC - 60 * 60 * 18,
    state: "running",
    status: "Up 18 hours",
    labels: { "com.docker.compose.project": "platform" },
    host: "staging-eu",
  },
  {
    id: "c7f4f9b13c2111a5f47dba1a00005",
    names: ["/worker-billing-1"],
    image: "ghcr.io/logdeck/worker:0.14.2",
    image_id: "sha256:5",
    command: "./worker billing",
    created: NOW_SEC - 60 * 60 * 9,
    state: "restarting",
    status: "Restarting (1) 8 seconds ago",
    labels: { "com.docker.compose.project": "jobs" },
    host: "staging-eu",
  },
  {
    id: "c7f4f9b13c2111a5f47dba1a00006",
    names: ["/ingest-s3"],
    image: "ghcr.io/logdeck/ingest:0.13.8",
    image_id: "sha256:6",
    command: "./ingest --provider s3",
    created: NOW_SEC - 60 * 60 * 2,
    state: "exited",
    status: "Exited (137) 27 minutes ago",
    labels: { "com.docker.compose.project": "jobs" },
    host: "staging-eu",
  },
  {
    id: "c7f4f9b13c2111a5f47dba1a00007",
    names: ["/caddy-edge"],
    image: "caddy:2.9",
    image_id: "sha256:7",
    command: "caddy run --config /etc/caddy/Caddyfile",
    created: NOW_SEC - 60 * 60 * 24,
    state: "running",
    status: "Up 24 hours",
    labels: { "com.docker.compose.project": "edge" },
    host: "edge-us",
  },
  {
    id: "c7f4f9b13c2111a5f47dba1a00008",
    names: ["/otel-collector"],
    image: "otel/opentelemetry-collector:0.100.0",
    image_id: "sha256:8",
    command: "/otelcol --config=/etc/otelcol/config.yaml",
    created: NOW_SEC - 60 * 60 * 12,
    state: "paused",
    status: "Up 12 hours (Paused)",
    labels: { "com.docker.compose.project": "observability" },
    host: "edge-us",
  },
  {
    id: "c7f4f9b13c2111a5f47dba1a00009",
    names: ["/vector-agent"],
    image: "timberio/vector:0.40.0-alpine",
    image_id: "sha256:9",
    command: "vector --config /etc/vector/vector.toml",
    created: NOW_SEC - 60 * 60 * 20,
    state: "running",
    status: "Up 20 hours",
    labels: { "com.docker.compose.project": "observability" },
    host: "edge-us",
  },
  {
    id: "c7f4f9b13c2111a5f47dba1a00010",
    names: ["/job-cleanup"],
    image: "ghcr.io/logdeck/cron:0.14.2",
    image_id: "sha256:10",
    command: "./cron cleanup",
    created: NOW_SEC - 60 * 30,
    state: "exited",
    status: "Exited (0) 3 minutes ago",
    labels: { "com.docker.compose.project": "jobs" },
    host: "local-dev",
  },
  {
    id: "c7f4f9b13c2111a5f47dba1a00011",
    names: ["/minio"],
    image: "minio/minio:RELEASE.2025-01-20",
    image_id: "sha256:11",
    command: "minio server /data --console-address :9001",
    created: NOW_SEC - 60 * 60 * 48,
    state: "running",
    status: "Up 2 days",
    labels: { "com.docker.compose.project": "platform" },
    host: "staging-eu",
  },
  {
    id: "c7f4f9b13c2111a5f47dba1a00012",
    names: ["/nginx-preview"],
    image: "nginx:1.27-alpine",
    image_id: "sha256:12",
    command: "nginx -g 'daemon off;'",
    created: NOW_SEC - 60 * 60 * 7,
    state: "dead",
    status: "Dead",
    labels: { "com.docker.compose.project": "preview" },
    host: "local-dev",
  },
];

export const demoStatsSeed: ContainerStats[] = demoContainers.map((container, idx) => ({
  id: container.id,
  host: container.host,
  cpu_percent: container.state === "running" ? 2 + ((idx * 11) % 35) : 0,
  memory_percent: container.state === "running" ? 8 + ((idx * 7) % 52) : 0,
  memory_used: container.state === "running" ? (180 + idx * 43) * 1024 * 1024 : 0,
  memory_limit: 2048 * 1024 * 1024,
}));

function levelFromState(state: string): LogLevel {
  if (state === "dead" || state === "restarting") return "ERROR";
  if (state === "exited") return "WARN";
  return "INFO";
}

export const demoLogsSeed: Record<string, LogEntry[]> = Object.fromEntries(
  demoContainers.map((container, idx) => {
    const baseLevel = levelFromState(container.state);
    const now = Date.now();
    const lines: LogEntry[] = Array.from({ length: 140 }, (_, i) => {
      const at = new Date(now - (140 - i) * 15000).toISOString();
      const level: LogLevel = i % 37 === 0 ? "ERROR" : i % 11 === 0 ? "WARN" : baseLevel;
      const message =
        i % 19 === 0
          ? JSON.stringify({
              traceId: `trace-${idx}-${i}`,
              route: "/api/v1/containers",
              latencyMs: 12 + ((i * 7) % 190),
              status: level === "ERROR" ? 500 : 200,
            })
          : `container=${container.names[0].slice(1)} level=${level} event=${i} host=${container.host}`;

      return {
        timestamp: at,
        level,
        message,
        raw: message,
        stream: i % 9 === 0 ? "stderr" : "stdout",
      };
    });

    return [container.id, lines];
  })
);

export const demoEnvSeed: Record<string, Record<string, string>> = Object.fromEntries(
  demoContainers.map((c, idx) => [
    c.id,
    {
      NODE_ENV: "production",
      LOG_LEVEL: idx % 3 === 0 ? "debug" : "info",
      REGION: c.host,
      FEATURE_FLAGS: "containers,logs,metrics",
      BUILD_SHA: `a1b2c3d${idx}`,
    },
  ])
);
