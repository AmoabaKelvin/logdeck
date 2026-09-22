import { formatDistanceToNowStrict } from "date-fns";

import type { StoredContainer } from "../api/get-history";
import type { ContainerInfo, ContainerStats } from "../types";

export type SortDirection = "asc" | "desc";
export const SORT_KEYS = [
	"name",
	"host",
	"status",
	"uptime",
	"created",
	"ports",
	"cpu",
	"memory",
] as const;
export type SortKey = (typeof SORT_KEYS)[number];
export type GroupByOption = "none" | "compose";
export type ContainerActionType = "start" | "stop" | "restart" | "remove";

// Containers that no longer exist on the host but still have stored logs are
// surfaced as a synthetic dashboard state rather than a separate page.
export const REMOVED_STATE = "removed";
// Exited containers pile up once history is kept, so the dashboard opens on
// running ones and everything else is a filter away.
export const DEFAULT_STATE_FILTER = "running";

export interface GroupedContainers {
	project: string;
	items: ContainerInfo[];
}

export interface RemovedContainerInfo extends ContainerInfo {
	storedBytes: number;
	oldestTs: string;
	newestTs: string;
}

export interface StateCounts {
	running: number;
	exited: number;
	paused: number;
	restarting: number;
	dead: number;
	removed: number;
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

const SHORT_DIGEST_LENGTH = 12;

const shortenDigest = (reference: string) =>
	reference.replace(
		new RegExp(`(sha256:[0-9a-f]{${SHORT_DIGEST_LENGTH}})[0-9a-f]+`),
		"$1",
	);

/** A full digest is 71 characters that shoulder every other column off the row. */
export function formatImageName(image: string) {
	const [reference, digest] = image.split("@");
	const segments = reference.split("/");
	if (segments.length > 1 && /[.:]/.test(segments[0])) {
		segments.shift();
	}
	if (segments.length > 1 && segments[0] === "library") {
		segments.shift();
	}
	const name = shortenDigest(segments.join("/"));

	// A tag identifies the image on its own; without one the digest is all there is.
	return digest && !name.includes(":")
		? `${name}@${shortenDigest(digest)}`
		: name;
}

export function formatCreatedDate(createdSeconds: number) {
	const createdDate = new Date(createdSeconds * 1000);
	return createdDate.toLocaleString(undefined, {
		dateStyle: "medium",
		timeStyle: "short",
	});
}

/** Compact form for a table column: "3 days ago" rather than a full stamp. */
export function formatRelativeCreated(createdSeconds: number) {
	return formatDistanceToNowStrict(new Date(createdSeconds * 1000), {
		addSuffix: true,
	});
}

/**
 * Docker reports uptime inside the status string ("Up 4 days (healthy)").
 * Deriving it from `created` would be wrong for anything that has restarted
 * since, so read Docker's own answer and drop the prefix and health suffix.
 */
export function formatUptime(status: string) {
	const match = /^Up\s+(.+?)(?:\s+\(.*\))?$/.exec(status);
	return match ? match[1] : null;
}

export function toTitleCase(value: string) {
	if (!value) return value;
	return value.charAt(0).toUpperCase() + value.slice(1);
}

// Docker packs state, exit code and duration into one string. A table wants
// them apart: "Exited (0) 6 hours ago" is a state column and a duration column
// glued together.
const UP_STATUS = /^Up\s+(.+?)(?:\s+\([^)]*\))?$/i;
const CODED_STATUS = /^(?:Exited|Restarting)\s+\((-?\d+)\)\s+(.+?)\s+ago$/i;

export interface ContainerStatusParts {
	/** Short state label, carrying the exit code where Docker reports one. */
	label: string;
	/** How long it has been in that state, e.g. "47 hours". */
	duration: string | null;
	/** Docker's exit code where it reports one. Non-zero means an unclean stop. */
	exitCode: number | null;
}

export function splitContainerStatus(
	container: ContainerInfo,
): ContainerStatusParts {
	const label = toTitleCase(container.state);

	const up = UP_STATUS.exec(container.status);
	if (up) {
		return { label, duration: up[1], exitCode: null };
	}

	const coded = CODED_STATUS.exec(container.status);
	if (coded) {
		return {
			label: `${label} (${coded[1]})`,
			duration: coded[2],
			exitCode: Number(coded[1]),
		};
	}

	return { label, duration: null, exitCode: null };
}

export function getStateBadgeClass(state: string) {
	const normalized = state.toLowerCase();
	switch (normalized) {
		case "running":
			return "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400";
		case "paused":
			return "bg-amber-500/10 text-amber-700 dark:text-amber-400";
		// A clean exit is not a failure; only "dead" earns an alarm colour.
		case "exited":
			return "bg-muted text-muted-foreground";
		case "dead":
			return "bg-rose-500/10 text-rose-700 dark:text-rose-400";
		case "restarting":
			return "bg-blue-500/10 text-blue-700 dark:text-blue-400";
		case REMOVED_STATE:
			return "bg-muted text-muted-foreground";
		default:
			return "bg-muted text-muted-foreground";
	}
}

export function getHealthBadgeClass(health: string) {
	switch (health) {
		case "healthy":
			return "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400";
		case "unhealthy":
			return "bg-rose-500/10 text-rose-700 dark:text-rose-400";
		case "starting":
			return "bg-amber-500/10 text-amber-700 dark:text-amber-400";
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
		removed: 0,
		other: 0,
	};
}

function stripLeadingSlash(name: string) {
	return name.startsWith("/") ? name.slice(1) : name;
}

function containerKey(host: string, name: string) {
	return `${host}\0${stripLeadingSlash(name)}`;
}

function toUnixSeconds(timestamp: string) {
	const parsed = Date.parse(timestamp);
	return Number.isNaN(parsed) ? 0 : Math.floor(parsed / 1000);
}

function toRemovedContainer(stored: StoredContainer): RemovedContainerInfo {
	const name = stripLeadingSlash(stored.name);

	return {
		// The logs route resolves by name and falls back to history for names that
		// are not live, so using the name as the id keeps the existing link working.
		id: name,
		names: [`/${name}`],
		image: stored.image ?? "—",
		image_id: "",
		command: "",
		created: toUnixSeconds(stored.oldestTs),
		state: REMOVED_STATE,
		status: "",
		labels: stored.composeProject
			? {
					"com.docker.compose.project": stored.composeProject,
					"io.podman.compose.project": stored.composeProject,
				}
			: undefined,
		host: stored.host,
		storedBytes: stored.storedBytes,
		oldestTs: stored.oldestTs,
		newestTs: stored.newestTs,
	};
}

/**
 * Turns stored-log entries for containers that no longer exist into dashboard
 * rows. Entries that are still live, not removed, or excluded from the store
 * are skipped.
 */
export function synthesizeRemovedContainers(
	stored: StoredContainer[],
	liveContainers: ContainerInfo[],
): RemovedContainerInfo[] {
	const liveKeys = new Set(
		liveContainers.flatMap((container) =>
			container.names.map((name) => containerKey(container.host, name)),
		),
	);

	return stored
		.filter(
			(entry) =>
				entry.removed &&
				!entry.excluded &&
				!liveKeys.has(containerKey(entry.host, entry.name)),
		)
		.map(toRemovedContainer);
}

export function isRemovedContainer(
	container: ContainerInfo,
): container is RemovedContainerInfo {
	return container.state === REMOVED_STATE;
}

/**
 * Removed containers are noise for day-to-day container management, so they are
 * only listed when the state filter explicitly asks for them — not under "all".
 */
export function selectVisibleContainers(
	liveContainers: ContainerInfo[],
	removedContainers: RemovedContainerInfo[],
	stateFilter: string,
): ContainerInfo[] {
	return stateFilter === REMOVED_STATE ? removedContainers : liveContainers;
}

const UNIT_SECONDS = new Map([
	["second", 1],
	["minute", 60],
	["hour", 3600],
	["day", 86_400],
	["week", 604_800],
	["month", 2_592_000],
	["year", 31_536_000],
]);

/** Docker's humanised duration ("4 days", "About an hour") back to seconds. */
export function parseDurationSeconds(duration: string | null) {
	const match = /(\d+|an?)\s+(second|minute|hour|day|week|month|year)/i.exec(
		duration ?? "",
	);
	const unit = UNIT_SECONDS.get(match?.[2].toLowerCase() ?? "");
	if (!match || !unit) return undefined;
	return (Number(match[1]) || 1) * unit;
}

type TextSortKey = "name" | "host" | "status";

function textSortValue(container: ContainerInfo, key: TextSortKey) {
	if (key === "name") return formatContainerName(container.names);
	return key === "host" ? container.host : container.state;
}

function numericSortValue(
	container: ContainerInfo,
	key: Exclude<SortKey, TextSortKey>,
	statsMap: Record<string, ContainerStats>,
) {
	switch (key) {
		case "uptime":
			return parseDurationSeconds(splitContainerStatus(container).duration);
		case "created":
			return container.created;
		case "ports":
			return container.ports?.[0]?.publicPort;
		case "cpu":
			return statsMap[container.id]?.cpu_percent;
		case "memory":
			return isRemovedContainer(container)
				? container.storedBytes
				: statsMap[container.id]?.memory_used;
	}
}

/** Rows with no value for the key (no stats, no ports) sink in either direction. */
export function sortContainers(
	containers: ContainerInfo[],
	key: SortKey,
	direction: SortDirection,
	statsMap: Record<string, ContainerStats>,
): ContainerInfo[] {
	const sign = direction === "asc" ? 1 : -1;

	if (key === "name" || key === "host" || key === "status") {
		return [...containers].sort(
			(a, b) =>
				sign *
				textSortValue(a, key).localeCompare(textSortValue(b, key), undefined, {
					sensitivity: "base",
				}),
		);
	}

	return [...containers].sort((a, b) => {
		const av = numericSortValue(a, key, statsMap);
		const bv = numericSortValue(b, key, statsMap);
		if (av === undefined || bv === undefined) {
			return Number(av === undefined) - Number(bv === undefined);
		}
		return sign * (av - bv);
	});
}

/**
 * Members of a compose project, including containers that were removed but
 * still have stored logs. Removed members are listed last: live containers are
 * what users act on, the dead ones only carry history.
 */
export function selectStackMembers(
	liveContainers: ContainerInfo[],
	removedContainers: RemovedContainerInfo[],
	project: string,
): ContainerInfo[] {
	const inProject = (container: ContainerInfo) =>
		getComposeProject(container.labels) === project;

	return [
		...liveContainers.filter(inProject),
		...removedContainers.filter(inProject),
	];
}

export function countContainerStates(
	liveContainers: ContainerInfo[],
	removedContainers: RemovedContainerInfo[],
	matches: (container: ContainerInfo) => boolean,
): StateCounts {
	const counts = getInitialStateCounts();

	liveContainers.forEach((container) => {
		if (!matches(container)) return;
		const state = container.state.toLowerCase();
		if (state === "running") counts.running++;
		else if (state === "exited") counts.exited++;
		else if (state === "paused") counts.paused++;
		else if (state === "restarting") counts.restarting++;
		else if (state === "dead") counts.dead++;
		else counts.other++;
	});

	removedContainers.forEach((container) => {
		if (matches(container)) counts.removed++;
	});

	return counts;
}

/**
 * Gets the container name for use in URLs (without leading slash)
 * Falls back to container ID if no name is available
 */
/**
 * Resolves a detail-page URL to a container. The identifier is a name, or an
 * ID for old links. Names are only unique per host, so `host` narrows the
 * match; without it the first host wins.
 */
export function findContainerByIdentifier(
	containers: ContainerInfo[],
	identifier: string,
	host?: string,
) {
	return containers.find(
		(container) =>
			(!host || container.host === host) &&
			(getContainerUrlIdentifier(container) === identifier ||
				container.id.startsWith(identifier)),
	);
}

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

/** One decimal only where it carries something: "7.8 GB" earns it, "800.0 KB" does not. */
export function formatBytes(bytes: number): string {
	// An em dash is for a reading we do not have; zero is a reading. Returning
	// it here also keeps 0 away from the log below, which is -Infinity.
	if (!Number.isFinite(bytes) || bytes < 0) return "—";
	if (bytes === 0) return "0 B";

	const units = ["B", "KB", "MB", "GB", "TB"];
	const index = Math.min(
		Math.floor(Math.log(bytes) / Math.log(1024)),
		units.length - 1,
	);
	const value = bytes / 1024 ** index;

	const decimals = index > 0 && value < 10 ? 1 : 0;
	// Number() drops a zero tenth: 1024 bytes is "1 KB", not "1.0 KB".
	return `${Number(value.toFixed(decimals))} ${units[index]}`;
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

/**
 * Where a container's published ports can be reached from the browser.
 *
 * The Docker host address tells us: a unix socket means the engine runs beside
 * LogDeck, so the ports are on whatever host the browser already reached us
 * on; ssh:// and tcp:// name a remote machine. Anything we cannot read returns
 * null and the port renders as plain text rather than a link that lies.
 */
export function resolvePublishedHost(
	dockerHostAddress: string | undefined,
	localHostname: string,
): string | null {
	if (!dockerHostAddress) return localHostname;

	const [scheme, rest] = dockerHostAddress.split("://");
	if (rest === undefined) return localHostname;

	switch (scheme) {
		case "unix":
		case "npipe":
			return localHostname;
		case "ssh":
		case "tcp":
		case "http":
		case "https": {
			// Drop any user@ prefix and :port suffix; keep bracketed IPv6 intact.
			const authority = rest.split("/")[0].split("@").pop() ?? "";
			if (authority.startsWith("[")) {
				return authority.slice(0, authority.indexOf("]") + 1) || null;
			}
			const host = authority.split(":")[0];
			return host || null;
		}
		default:
			return null;
	}
}
