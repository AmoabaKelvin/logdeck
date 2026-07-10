import type { ContainerInfo, ContainerStats } from "../types";

export type SortDirection = "asc" | "desc";
export type GroupByOption = "none" | "compose";
export type ContainerActionType = "start" | "stop" | "restart" | "remove";

export interface GroupedContainers {
	project: string;
	items: ContainerInfo[];
}

export interface StateCounts {
	running: number;
	exited: number;
	paused: number;
	restarting: number;
	dead: number;
	other: number;
}

// Docker Compose and recent podman-compose both set the com.docker label;
// older podman-compose releases only set the io.podman one.
const COMPOSE_PROJECT_LABELS = [
	"com.docker.compose.project",
	"io.podman.compose.project",
];

export function getComposeProject(labels?: Record<string, string>) {
	for (const label of COMPOSE_PROJECT_LABELS) {
		const project = labels?.[label]?.trim();
		if (project) {
			return project;
		}
	}
	return undefined;
}

export function formatContainerName(names: string[]) {
	if (!names.length) {
		return "—";
	}
	const [primary] = names;
	return primary.startsWith("/") ? primary.slice(1) : primary;
}

export function formatCreatedDate(createdSeconds: number) {
	const createdDate = new Date(createdSeconds * 1000);
	return createdDate.toLocaleString(undefined, {
		dateStyle: "medium",
		timeStyle: "short",
	});
}

export function toTitleCase(value: string) {
	if (!value) return value;
	return value.charAt(0).toUpperCase() + value.slice(1);
}

export function getStateBadgeClass(state: string) {
	const normalized = state.toLowerCase();
	switch (normalized) {
		case "running":
			return "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400";
		case "paused":
			return "bg-amber-500/10 text-amber-700 dark:text-amber-400";
		case "exited":
		case "dead":
			return "bg-rose-500/10 text-rose-700 dark:text-rose-400";
		case "restarting":
			return "bg-blue-500/10 text-blue-700 dark:text-blue-400";
		default:
			return "bg-muted text-muted-foreground";
	}
}

export function groupByCompose(
	containers: ContainerInfo[],
): GroupedContainers[] {
	const groups = new Map<string, ContainerInfo[]>();

	containers.forEach((container) => {
		const key = getComposeProject(container.labels) || "Standalone";
		if (!groups.has(key)) {
			groups.set(key, []);
		}
		groups.get(key)?.push(container);
	});

	return Array.from(groups.entries())
		.sort(([a], [b]) => a.localeCompare(b))
		.map(([project, items]) => ({ project, items }));
}

export function getInitialStateCounts(): StateCounts {
	return {
		running: 0,
		exited: 0,
		paused: 0,
		restarting: 0,
		dead: 0,
		other: 0,
	};
}

/**
 * Gets the container name for use in URLs (without leading slash)
 * Falls back to container ID if no name is available
 */
export function getContainerUrlIdentifier(container: ContainerInfo): string {
	if (container.names && container.names.length > 0) {
		const name = container.names[0];
		return name.startsWith("/") ? name.slice(1) : name;
	}
	// Fallback to short ID if no name
	return container.id.substring(0, 12);
}

export function isCoolifyManaged(labels?: Record<string, string>): boolean {
	return labels?.["coolify.managed"] === "true";
}

export function formatBytes(bytes: number): string {
	if (!bytes || bytes < 0) return "—";
	if (bytes === 0) return "0 B";

	const units = ["B", "KB", "MB", "GB", "TB"];
	const index = Math.min(
		Math.floor(Math.log(bytes) / Math.log(1024)),
		units.length - 1,
	);
	const value = bytes / 1024 ** index;

	return `${value.toFixed(index === 0 ? 0 : 1)}${units[index]}`;
}

export function formatCPUPercent(percent: number | undefined): string {
	return percent != null ? `${percent.toFixed(1)}%` : "—";
}

export function formatMemoryStats(stats: ContainerStats | undefined): string {
	if (!stats?.memory_percent || !stats?.memory_used) return "—";

	const percent = stats.memory_percent.toFixed(1);
	const usage = formatBytes(stats.memory_used);
	const limit = stats.memory_limit ? `/${formatBytes(stats.memory_limit)}` : "";

	return `${percent}% - ${usage}${limit}`;
}
