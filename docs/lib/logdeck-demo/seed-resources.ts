import type {
  ImageInfo,
  NetworkInfo,
  VolumeInfo,
} from "@/components/logdeck-demo/types";
import { NOW_SEC, seedContainers } from "./seed-containers";

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
    host: "homelab",
  },
];

export const demoVolumes: VolumeInfo[] = [
  {
    name: "postgres-data",
    driver: "local",
    mountpoint: "/var/lib/docker/volumes/postgres-data/_data",
    created: new Date((NOW_SEC - 60 * 60 * 24 * 30) * 1000).toISOString(),
    labels: { "com.docker.compose.project": "marketmap-dj" },
    host: "homelab",
  },
  {
    name: "logdeck-history",
    driver: "local",
    mountpoint: "/var/lib/docker/volumes/logdeck-history/_data",
    created: new Date((NOW_SEC - 60 * 60 * 24 * 12) * 1000).toISOString(),
    labels: { "com.docker.compose.project": "logdeck-edge" },
    host: "homelab",
  },
  {
    name: "minio-data",
    driver: "local",
    mountpoint: "/var/lib/docker/volumes/minio-data/_data",
    created: new Date((NOW_SEC - 60 * 60 * 24 * 45) * 1000).toISOString(),
    host: "homelab",
  },
  {
    name: "redis-data",
    driver: "local",
    mountpoint: "/var/lib/docker/volumes/redis-data/_data",
    created: new Date((NOW_SEC - 60 * 60 * 20) * 1000).toISOString(),
    host: "homelab",
  },
  {
    name: "caddy-config",
    driver: "local",
    mountpoint: "/var/lib/docker/volumes/caddy-config/_data",
    created: new Date((NOW_SEC - 60 * 60 * 24 * 8) * 1000).toISOString(),
    host: "homelab",
  },
];

export const demoNetworks: NetworkInfo[] = [
  {
    id: "9f1c44aa10b2",
    name: "bridge",
    driver: "bridge",
    scope: "local",
    subnets: ["172.17.0.0/16"],
    host: "homelab",
  },
  {
    id: "1d2e9b00c4a7",
    name: "logdeck-edge_default",
    driver: "bridge",
    scope: "local",
    subnets: ["172.21.0.0/16"],
    host: "homelab",
  },
  {
    id: "77aa0e91d3f5",
    name: "logdeck-states_default",
    driver: "bridge",
    scope: "local",
    subnets: ["172.22.0.0/16"],
    host: "homelab",
  },
  {
    id: "b8c1f2a97e60",
    name: "telemetrydev_default",
    driver: "bridge",
    scope: "local",
    subnets: ["172.23.0.0/16"],
    host: "homelab",
  },
  {
    id: "40d6c7e8aa19",
    name: "host",
    driver: "host",
    scope: "local",
    host: "homelab",
  },
];

// Pre-baked sparkline history so trend lines are visible on first paint
// instead of only after a minute of polling.
