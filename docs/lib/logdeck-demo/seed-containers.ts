import type {
  ContainerInfo,
  ContainerPort,
  DockerHost,
} from "@/components/logdeck-demo/types";

export const NOW_MS = Date.now();
export const NOW_SEC = Math.floor(NOW_MS / 1000);

export const demoHosts: DockerHost[] = [
  { name: "homelab", host: "unix:///var/run/docker.sock" },
];

export type SeedKind =
  | "api"
  | "ui"
  | "postgres"
  | "clickhouse"
  | "redis"
  | "worker"
  | "ingest"
  | "proxy"
  | "otel"
  | "vector"
  | "cron"
  | "minio"
  | "nginx";

export interface SeedContainer extends ContainerInfo {
  // Which log flavor the generator uses for this container.
  kind: SeedKind;
}

export const HOUR = 60 * 60;
export const DAY = 24 * HOUR;

export const compose = (project: string, service: string) => ({
  "com.docker.compose.project": project,
  "com.docker.compose.service": service,
});

const tcp = (...ports: (number | [number, number])[]): ContainerPort[] =>
  ports.map((port) =>
    Array.isArray(port)
      ? { publicPort: port[0], privatePort: port[1], type: "tcp" }
      : { publicPort: port, privatePort: port, type: "tcp" },
  );

interface SeedSpec {
  name: string;
  image: string;
  command: string;
  age: number;
  state: string;
  status: string;
  kind: SeedKind;
  health?: string;
  labels?: Record<string, string>;
  ports?: ContainerPort[];
}

export const seed = (spec: SeedSpec, index: number): SeedContainer => ({
  id: `c7f4f9b13c2111a5f47dba1a${String(index + 1).padStart(5, "0")}`,
  names: [`/${spec.name}`],
  image: spec.image,
  image_id: `sha256:${(0x1a7c + index * 0x3b1).toString(16)}`,
  command: spec.command,
  created: NOW_SEC - spec.age,
  state: spec.state,
  status: spec.status,
  health: spec.health,
  labels: spec.labels,
  ports: spec.ports,
  host: "homelab",
  kind: spec.kind,
});

export const seedContainers: SeedContainer[] = (
  [
    {
      name: "edge-router",
      image: "traefik:v3.3",
      command: "traefik --providers.docker",
      age: 11 * DAY + 3 * HOUR,
      state: "running",
      status: "Up 11 days (healthy)",
      health: "healthy",
      ports: tcp(80, 443, 8080),
      kind: "proxy",
    },
    {
      name: "logdeck",
      image: "ghcr.io/amoabakelvin/logdeck:0.1.0",
      command: "/app/logdeck",
      age: 3 * HOUR + 12 * 60,
      state: "running",
      status: "Up 3 hours (healthy)",
      health: "healthy",
      ports: tcp(3000),
      kind: "api",
    },
    {
      name: "watchtower",
      image: "containrrr/watchtower:1.7.1",
      command: "/watchtower --cleanup --interval 3600",
      age: 11 * DAY + 2 * HOUR,
      state: "running",
      status: "Up 11 days",
      kind: "cron",
    },
    {
      name: "pihole",
      image: "pihole/pihole:2025.03.0",
      command: "start.sh",
      age: 11 * DAY,
      state: "running",
      status: "Up 11 days (healthy)",
      health: "healthy",
      ports: tcp(53, [8053, 80]),
      kind: "proxy",
    },
    {
      name: "nginx-preview",
      image: "nginx:1.27-alpine",
      command: "nginx -g 'daemon off;'",
      age: 7 * HOUR,
      state: "dead",
      status: "Dead",
      kind: "nginx",
    },
    {
      name: "alpine-scratch",
      image: "alpine:3.20",
      command: "/bin/sh",
      age: 2 * DAY + 5 * HOUR,
      state: "exited",
      status: "Exited (0) 2 days ago",
      kind: "cron",
    },
    {
      name: "logdeck-edge-edge-proxy-1",
      image: "caddy:2.9-alpine",
      command: "caddy run --config /etc/caddy/Caddyfile",
      age: 5 * DAY + 4 * HOUR,
      state: "running",
      status: "Up 5 days (healthy)",
      health: "healthy",
      labels: {
        ...compose("logdeck-edge", "edge-proxy"),
        "coolify.managed": "true",
      },
      ports: tcp(8443),
      kind: "proxy",
    },
    {
      name: "logdeck-edge-api-1",
      image: "ghcr.io/logdeck/api:0.14.2",
      command: "./logdeck-server --port 8080",
      age: 5 * DAY + 4 * HOUR,
      state: "running",
      status: "Up 5 days (healthy)",
      health: "healthy",
      labels: compose("logdeck-edge", "api"),
      ports: tcp(8080),
      kind: "api",
    },
    {
      name: "logdeck-edge-ui-1",
      image: "ghcr.io/logdeck/ui:0.14.2",
      command: "bun run serve",
      age: 5 * DAY + 4 * HOUR,
      state: "running",
      status: "Up 5 days",
      labels: compose("logdeck-edge", "ui"),
      kind: "ui",
    },
    {
      name: "logdeck-states-api-gateway-1",
      image: "ghcr.io/logdeck/gateway:1.4.0",
      command: "./gateway --listen :8000",
      age: 26 * HOUR,
      state: "running",
      status: "Up 26 hours (healthy)",
      health: "healthy",
      labels: compose("logdeck-states", "api-gateway"),
      ports: tcp(8000),
      kind: "api",
    },
    {
      name: "logdeck-states-search-indexer-1",
      image: "ghcr.io/logdeck/indexer:1.4.0",
      command: "./indexer --source s3",
      age: 26 * HOUR,
      state: "running",
      status: "Up 26 hours (unhealthy)",
      health: "unhealthy",
      labels: compose("logdeck-states", "search-indexer"),
      kind: "ingest",
    },
    {
      name: "logdeck-states-payments-worker-1",
      image: "ghcr.io/logdeck/worker:1.4.0",
      command: "./worker payments",
      age: 26 * HOUR,
      state: "running",
      status: "Up 26 hours (unhealthy)",
      health: "unhealthy",
      labels: compose("logdeck-states", "payments-worker"),
      kind: "worker",
    },
    {
      name: "logdeck-states-postgres-1",
      image: "postgres:17.10-alpine",
      command: "docker-entrypoint.sh postgres",
      age: 26 * HOUR,
      state: "running",
      status: "Up 26 hours (healthy)",
      health: "healthy",
      labels: compose("logdeck-states", "postgres"),
      ports: tcp([5433, 5432]),
      kind: "postgres",
    },
    {
      name: "logdeck-states-redis-1",
      image: "redis:7.4-alpine",
      command: "redis-server --save 60 1000",
      age: 26 * HOUR,
      state: "running",
      status: "Up 26 hours (healthy)",
      health: "healthy",
      labels: compose("logdeck-states", "redis"),
      kind: "redis",
    },
    {
      name: "logdeck-states-scheduler-1",
      image: "ghcr.io/logdeck/cron:1.4.0",
      command: "./cron schedule",
      age: 26 * HOUR,
      state: "restarting",
      status: "Restarting (1) 8 seconds ago",
      labels: compose("logdeck-states", "scheduler"),
      kind: "cron",
    },
    {
      name: "logdeck-states-migrate-1",
      image: "ghcr.io/logdeck/migrate:1.4.0",
      command: "./migrate up",
      age: 26 * HOUR,
      state: "exited",
      status: "Exited (0) 26 hours ago",
      labels: compose("logdeck-states", "migrate"),
      kind: "cron",
    },
    {
      name: "marketmap-dj-postgres-1",
      image: "pgvector/pgvector:0.8.2-pg15-trixie",
      command: "docker-entrypoint.sh postgres",
      age: 9 * DAY + 6 * HOUR,
      state: "running",
      status: "Up 9 days (healthy)",
      health: "healthy",
      labels: compose("marketmap-dj", "postgres"),
      ports: tcp(5432),
      kind: "postgres",
    },
    {
      name: "marketmap-dj-web-1",
      image: "marketmap-dj-web:latest",
      command: "gunicorn marketmap.wsgi --bind 0.0.0.0:8001",
      age: 9 * DAY + 6 * HOUR,
      state: "running",
      status: "Up 9 days",
      labels: compose("marketmap-dj", "web"),
      ports: tcp(8001),
      kind: "api",
    },
    {
      name: "marketmap-dj-celery-1",
      image: "marketmap-dj-web:latest",
      command: "celery -A marketmap worker -l info",
      age: 9 * DAY + 6 * HOUR,
      state: "running",
      status: "Up 9 days",
      labels: compose("marketmap-dj", "celery"),
      kind: "worker",
    },
    {
      name: "marketmap-dj-redis-1",
      image: "redis:7.4-alpine",
      command: "redis-server",
      age: 9 * DAY + 6 * HOUR,
      state: "running",
      status: "Up 9 days (healthy)",
      health: "healthy",
      labels: compose("marketmap-dj", "redis"),
      kind: "redis",
    },
    {
      name: "marketmap-dj-beat-1",
      image: "marketmap-dj-web:latest",
      command: "celery -A marketmap beat -l info",
      age: 9 * DAY + 6 * HOUR,
      state: "exited",
      status: "Exited (137) 3 days ago",
      labels: compose("marketmap-dj", "beat"),
      kind: "cron",
    },
    {
      name: "telemetrydev-clickhouse-1",
      image: "clickhouse/clickhouse-server:26.5.1.882-alpine",
      command: "/entrypoint.sh",
      age: 18 * HOUR,
      state: "running",
      status: "Up 18 hours (healthy)",
      health: "healthy",
      labels: compose("telemetrydev", "clickhouse"),
      ports: tcp([8124, 8123], [9001, 9000]),
      kind: "clickhouse",
    },
    {
      name: "telemetrydev-postgres-1",
      image: "postgres:17.10-alpine",
      command: "docker-entrypoint.sh postgres",
      age: 18 * HOUR,
      state: "running",
      status: "Up 18 hours (healthy)",
      health: "healthy",
      labels: compose("telemetrydev", "postgres"),
      ports: tcp([5434, 5432]),
      kind: "postgres",
    },
    {
      name: "telemetrydev-otel-collector-1",
      image: "otel/opentelemetry-collector-contrib:0.121.0",
      command: "/otelcol-contrib --config=/etc/otelcol/config.yaml",
      age: 18 * HOUR,
      state: "running",
      status: "Up 18 hours",
      labels: compose("telemetrydev", "otel-collector"),
      ports: tcp(4317, 4318),
      kind: "otel",
    },
    {
      name: "telemetrydev-vector-1",
      image: "timberio/vector:0.45.0-alpine",
      command: "vector --config /etc/vector/vector.toml",
      age: 18 * HOUR,
      state: "paused",
      status: "Up 18 hours (Paused)",
      labels: compose("telemetrydev", "vector"),
      kind: "vector",
    },
    {
      name: "telemetrydev-grafana-1",
      image: "grafana/grafana:11.5.2",
      command: "/run.sh",
      age: 18 * HOUR,
      state: "running",
      status: "Up 18 hours (health: starting)",
      health: "starting",
      labels: compose("telemetrydev", "grafana"),
      ports: tcp([3001, 3000]),
      kind: "ui",
    },
    {
      name: "telemetrydev-minio-1",
      image: "minio/minio:RELEASE.2025-01-20",
      command: "minio server /data --console-address :9001",
      age: 2 * DAY + 2 * HOUR,
      state: "exited",
      status: "Exited (0) 2 days ago",
      labels: compose("telemetrydev", "minio"),
      kind: "minio",
    },
    {
      name: "telemetrydev-ingest-1",
      image: "ghcr.io/logdeck/ingest:0.13.8",
      command: "./ingest --provider s3",
      age: 2 * HOUR,
      state: "exited",
      status: "Exited (137) 27 minutes ago",
      labels: compose("telemetrydev", "ingest"),
      kind: "ingest",
    },
  ] satisfies SeedSpec[]
).map(seed);

// ---------------------------------------------------------------------------
// Log generation
