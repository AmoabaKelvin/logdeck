import {
  demoContainers,
  demoEnvSeed,
  demoHosts,
  demoLogsSeed,
  demoStatsSeed,
} from "@/lib/logdeck-demo/demo-data";

import type { ContainerInfo, ContainerStats, DockerHost, HostError } from "@/components/logdeck-demo/containers/types";
import type { LogEntry, LogLevel } from "@/types/logs";

export interface GetContainersResponse {
  containers: ContainerInfo[];
  readOnly: boolean;
  hosts: DockerHost[];
  hostErrors: HostError[];
}

export interface SystemStats {
  hostInfo: {
    hostname: string;
    platform: string;
    platformVersion: string;
    kernelVersion: string;
    arch: string;
    uptime: number;
  };
  usage: {
    cpuPercent: number;
    memoryPercent: number;
    memoryTotal: number;
    memoryUsed: number;
  };
}

export interface ContainerLogsOptions {
  since?: string;
  until?: string;
  tail?: string | number;
  details?: boolean;
  stdout?: boolean;
  stderr?: boolean;
  follow?: boolean;
}

const state = {
  containers: structuredClone(demoContainers),
  stats: structuredClone(demoStatsSeed),
  logs: structuredClone(demoLogsSeed),
  env: structuredClone(demoEnvSeed),
};

function getContainerById(id: string) {
  return state.containers.find((c) => c.id === id);
}

function sleep(ms: number, signal?: AbortSignal) {
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

export async function getContainersDemo(): Promise<GetContainersResponse> {
  return {
    containers: structuredClone(state.containers),
    readOnly: false,
    hosts: structuredClone(demoHosts),
    hostErrors: [],
  };
}

export async function getSystemStatsDemo(): Promise<SystemStats> {
  const running = state.containers.filter((c) => c.state === "running").length;
  const cpuPercent = Math.min(92, 18 + running * 4 + Math.floor(Math.random() * 8));
  const memoryPercent = Math.min(89, 22 + running * 3 + Math.floor(Math.random() * 8));

  return {
    hostInfo: {
      hostname: "docs-demo-host",
      platform: "linux",
      platformVersion: "6.10",
      kernelVersion: "6.10.12-generic",
      arch: "x64",
      uptime: 940_000,
    },
    usage: {
      cpuPercent,
      memoryPercent,
      memoryTotal: 16 * 1024 * 1024 * 1024,
      memoryUsed: Math.round((memoryPercent / 100) * 16 * 1024 * 1024 * 1024),
    },
  };
}

export async function getContainerStatsDemo(): Promise<{ stats: ContainerStats[] }> {
  const next = state.stats.map((s) => {
    const container = getContainerById(s.id);
    if (!container || container.state !== "running") {
      return { ...s, cpu_percent: 0, memory_percent: 0, memory_used: 0 };
    }

    const cpuJitter = (Math.random() - 0.5) * 3.5;
    const memJitter = (Math.random() - 0.5) * 2.4;

    const cpu = Math.max(0, Math.min(99, s.cpu_percent + cpuJitter));
    const memory = Math.max(1, Math.min(95, s.memory_percent + memJitter));

    return {
      ...s,
      cpu_percent: cpu,
      memory_percent: memory,
      memory_used: Math.round((memory / 100) * s.memory_limit),
    };
  });

  state.stats = next;
  return { stats: structuredClone(next) };
}

export async function startContainerDemo(id: string): Promise<string> {
  const container = getContainerById(id);
  if (!container) throw new Error("Container not found");
  container.state = "running";
  container.status = "Up a few seconds";
  return `Started ${container.names[0].slice(1)}`;
}

export async function stopContainerDemo(id: string): Promise<string> {
  const container = getContainerById(id);
  if (!container) throw new Error("Container not found");
  container.state = "exited";
  container.status = "Exited (0) just now";
  return `Stopped ${container.names[0].slice(1)}`;
}

export async function restartContainerDemo(id: string): Promise<string> {
  const container = getContainerById(id);
  if (!container) throw new Error("Container not found");
  container.state = "running";
  container.status = "Up less than a second";
  return `Restarted ${container.names[0].slice(1)}`;
}

export async function removeContainerDemo(id: string): Promise<string> {
  const idx = state.containers.findIndex((c) => c.id === id);
  if (idx === -1) throw new Error("Container not found");
  const [removed] = state.containers.splice(idx, 1);
  delete state.logs[id];
  delete state.env[id];
  state.stats = state.stats.filter((s) => s.id !== id);
  return `Removed ${removed.names[0].slice(1)}`;
}

function normalizeLevel(level: LogLevel): LogLevel {
  return level;
}

function buildLiveLog(container: ContainerInfo): LogEntry {
  const levels: LogLevel[] = ["INFO", "INFO", "INFO", "WARN", "DEBUG", "ERROR"];
  const level = levels[Math.floor(Math.random() * levels.length)];
  const isJson = Math.random() > 0.78;
  const message = isJson
    ? JSON.stringify({
        source: container.names[0].slice(1),
        level,
        latencyMs: 10 + Math.floor(Math.random() * 250),
        queueDepth: Math.floor(Math.random() * 300),
      })
    : `${container.names[0].slice(1)} ${level.toLowerCase()} heartbeat ok req=${Math.floor(Math.random() * 20000)}`;

  return {
    timestamp: new Date().toISOString(),
    level: normalizeLevel(level),
    message,
    raw: message,
    stream: Math.random() > 0.86 ? "stderr" : "stdout",
  };
}

export async function getContainerLogsParsedDemo(
  id: string,
  _host: string,
  options?: ContainerLogsOptions
): Promise<LogEntry[]> {
  const logs = state.logs[id] ?? [];
  const tail = Number(options?.tail ?? 100);
  return logs.slice(Math.max(0, logs.length - tail));
}

export async function* streamContainerLogsParsedDemo(
  id: string,
  _host: string,
  _options?: ContainerLogsOptions,
  signal?: AbortSignal
): AsyncGenerator<LogEntry, void, unknown> {
  const container = getContainerById(id);
  if (!container) return;

  while (!signal?.aborted) {
    await sleep(700 + Math.floor(Math.random() * 650), signal);
    const line = buildLiveLog(container);
    const entries = state.logs[id] ?? [];
    entries.push(line);
    state.logs[id] = entries.slice(-1800);
    yield line;
  }
}

export async function getContainerEnvVariablesDemo(id: string): Promise<Record<string, string>> {
  return structuredClone(state.env[id] ?? {});
}

export async function updateContainerEnvVariablesDemo(
  id: string,
  env: Record<string, string>
): Promise<string> {
  state.env[id] = { ...env };
  return id;
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
