import { Badge } from "@/components/ui/badge";
import type { ContainerInfo, ContainerPort, ContainerStatsMap } from "../types";
import { formatBytes, formatCPUPercent } from "./container-utils";
import { Meter } from "./meter";
import { Sparkline } from "./sparkline";

// Flush with the page container: only the inner gutters are padded.
export const cellClass =
	"h-14 px-3 align-middle whitespace-nowrap first:pl-0 last:pr-0";
export const headClass =
	"h-11 px-3 text-left align-middle font-medium whitespace-nowrap text-muted-foreground first:pl-0 last:pr-0";

const TONE_BADGE_CLASS = {
	warn: "border-transparent bg-amber-500/10 text-amber-700 dark:text-amber-400",
	fail: "border-transparent bg-rose-500/10 text-rose-700 dark:text-rose-400",
} as const;

/**
 * A badge on every row is a shape that carries no information, so only states
 * worth acting on get one. "starting" stays quiet on purpose: an unsettled
 * healthcheck resolves itself.
 */
function statusTone(
	container: ContainerInfo,
	exitCode: number | null,
): "quiet" | "warn" | "fail" {
	const state = container.state.toLowerCase();

	if (container.health === "unhealthy" || state === "dead") return "fail";
	// Any non-zero code is an unclean stop, not just the SIGKILL 137.
	if (state === "exited" && exitCode !== null && exitCode !== 0) return "fail";
	if (state === "restarting" || state === "paused") return "warn";
	return "quiet";
}

export function StatusCell({
	container,
	label,
	exitCode,
}: {
	container: ContainerInfo;
	label: string;
	exitCode: number | null;
}) {
	const tone = statusTone(container, exitCode);
	// Health qualifies the state, so it rides inside the same label.
	const text =
		container.health && container.health !== "healthy"
			? `${label} · ${container.health}`
			: label;

	if (tone === "quiet") {
		return <span className="text-muted-foreground">{text}</span>;
	}
	return <Badge className={TONE_BADGE_CLASS[tone]}>{text}</Badge>;
}

const metricLabelClass =
	"w-7 shrink-0 text-[0.625rem] uppercase tracking-wide text-muted-foreground";
const metricValueClass = "w-14 shrink-0 text-right tabular-nums";

/** Memory as a length rather than a percentage: "1.5 GB ▓▓▓░░ of 8.0 GB". */
function MemoryMeter({ used, limit }: { used: number; limit: number }) {
	return (
		<div className="flex min-w-0 items-center gap-2">
			<Meter used={used} limit={limit} />
			{/* No room for the words beside the bar below xl. */}
			<span className="truncate text-muted-foreground tabular-nums max-xl:hidden">
				of {formatBytes(limit)}
			</span>
		</div>
	);
}

/** CPU is a trend, so a sparkline. Memory is a ratio against a ceiling, so a meter. */
export function MetricsCell({
	stats,
	history,
}: {
	stats: ContainerStatsMap[string] | undefined;
	history: number[];
}) {
	return (
		<div className="flex flex-col gap-1 font-mono text-xs">
			<div className="flex items-center gap-2">
				<span className={metricLabelClass}>CPU</span>
				<span className={metricValueClass}>
					{formatCPUPercent(stats?.cpu_percent)}
				</span>
				<Sparkline values={history} />
			</div>
			<div className="flex items-center gap-2">
				<span className={metricLabelClass}>Mem</span>
				<span className={metricValueClass}>
					{stats ? formatBytes(stats.memory_used) : "—"}
				</span>
				{stats && stats.memory_limit > 0 && (
					<MemoryMeter used={stats.memory_used} limit={stats.memory_limit} />
				)}
			</div>
		</div>
	);
}

const MAX_VISIBLE_PORTS = 2;

export function PortsCell({ ports }: { ports: ContainerPort[] }) {
	if (ports.length === 0) {
		return <span className="text-muted-foreground">—</span>;
	}

	const shown = ports.slice(0, MAX_VISIBLE_PORTS);
	const overflow = ports.length - shown.length;

	return (
		<div className="flex items-center gap-1">
			{shown.map((port) => (
				<span
					key={`${port.publicPort}-${port.privatePort}-${port.type}`}
					title={`${port.publicPort} → ${port.privatePort}/${port.type}`}
					className="shrink-0 rounded bg-muted px-1 py-0.5 font-mono text-xs tabular-nums"
				>
					{port.publicPort}
				</span>
			))}
			{overflow > 0 && (
				<span
					className="font-mono text-xs text-muted-foreground tabular-nums"
					title={ports
						.slice(MAX_VISIBLE_PORTS)
						.map((p) => `${p.publicPort} → ${p.privatePort}/${p.type}`)
						.join("\n")}
				>
					+{overflow}
				</span>
			)}
		</div>
	);
}
