import { Link } from "@tanstack/react-router";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	ArrowLeftIcon,
	PlayIcon,
	RotateCwIcon,
	SquareIcon,
	TerminalIcon,
	Trash2Icon,
} from "@/components/ui/icons";
import { Spinner } from "@/components/ui/spinner";
import type { ContainerInspect } from "../api/get-container-inspect";
import type { ContainerInfo, ContainerStats } from "../types";
import {
	formatBytes,
	formatCPUPercent,
	formatImageName,
	formatRelativeCreated,
	getHealthBadgeClass,
	getStateBadgeClass,
	splitContainerStatus,
	toTitleCase,
} from "./container-utils";
import { Meter } from "./meter";
import { Sparkline } from "./sparkline";

interface ContainerDetailHeaderProps {
	name: string;
	// Undefined while the live list loads, or for a container that no longer
	// exists and only survives as stored logs.
	container: ContainerInfo | undefined;
	isRemoved: boolean;
	isReadOnly: boolean;
	stats: ContainerStats | undefined;
	// CPU samples only; memory reads against its ceiling, not as a trend.
	history: number[];
	// Inspect carries what the container list cannot: restart counts and how
	// the last run ended.
	inspect: ContainerInspect | undefined;
	isActionPending: boolean;
	onStart: () => void;
	onStop: () => void;
	onRestart: () => void;
	onDelete: () => void;
	onOpenShell: () => void;
}

interface StatProps {
	label: string;
	value: string;
	detail?: string;
	// A sparkline for a trend, a meter for a ratio — whichever the reading is.
	trend?: React.ReactNode;
	tone?: "warn";
}

/** Label beside its reading — the same shape the dashboard header uses. */
function Stat({ label, value, detail, trend, tone }: StatProps) {
	return (
		<div className="flex min-w-0 items-center gap-2.5">
			<span className="shrink-0 font-mono text-[0.625rem] uppercase tracking-wide text-muted-foreground">
				{label}
			</span>
			<span
				className={`truncate text-sm font-medium tabular-nums ${
					tone === "warn" ? "text-amber-700 dark:text-amber-400" : ""
				}`}
				title={detail}
			>
				{value}
			</span>
			{trend}
		</div>
	);
}

function formatPorts(container: ContainerInfo): {
	value: string;
	detail: string;
} | null {
	const ports = container.ports ?? [];
	if (ports.length === 0) return null;
	return {
		value: ports.map((port) => port.publicPort).join(", "),
		detail: ports
			.map((port) => `${port.publicPort} → ${port.privatePort}/${port.type}`)
			.join("\n"),
	};
}

export function ContainerDetailHeader({
	name,
	container,
	isRemoved,
	isReadOnly,
	stats,
	history,
	inspect,
	isActionPending,
	onStart,
	onStop,
	onRestart,
	onDelete,
	onOpenShell,
}: ContainerDetailHeaderProps) {
	const state = container?.state.toLowerCase();
	const isRunning = state === "running" || state === "paused";
	const { label, duration } = container
		? splitContainerStatus(container)
		: { label: "", duration: null };
	const ports = container ? formatPorts(container) : null;

	const meta = container
		? [
				formatImageName(container.image),
				container.host,
				`created ${formatRelativeCreated(container.created)}`,
			]
		: [];

	// A running container is worth watching; a stopped one is worth explaining.
	// Same strip, different readings.
	const vitals: StatProps[] = [];
	if (container && !isRemoved) {
		if (isRunning) {
			vitals.push(
				{
					label: "CPU",
					value: formatCPUPercent(stats?.cpu_percent),
					trend: <Sparkline values={history} />,
				},
				{
					label: "Mem",
					value: stats ? formatBytes(stats.memory_used) : "—",
					trend: stats && stats.memory_limit > 0 && (
						<>
							<Meter used={stats.memory_used} limit={stats.memory_limit} />
							<span className="shrink-0 text-sm text-muted-foreground tabular-nums">
								of {formatBytes(stats.memory_limit)}
							</span>
						</>
					),
				},
			);
			if (duration) vitals.push({ label: "Up", value: duration });
		} else if (duration) {
			const sinceLabel =
				state === "exited" || state === "dead" ? "Stopped" : "Since";
			vitals.push({ label: sinceLabel, value: `${duration} ago` });
		}
		if (inspect && inspect.RestartCount > 0) {
			vitals.push({
				label: "Restarts",
				value: String(inspect.RestartCount),
				tone: "warn",
			});
		}
		if (ports) {
			vitals.push({ label: "Ports", value: ports.value, detail: ports.detail });
		}
	}

	return (
		<header>
			<Link
				to="/"
				className="inline-flex items-center gap-1.5 rounded-sm text-base text-muted-foreground hover:text-foreground sm:text-sm"
			>
				<ArrowLeftIcon className="size-4 shrink-0" />
				Containers
			</Link>

			<div className="mt-3 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between sm:gap-8">
				<div className="min-w-0">
					<div className="flex flex-wrap items-center gap-x-2.5 gap-y-1">
						<h1 className="min-w-0 truncate text-2xl font-semibold tracking-tight">
							{name}
						</h1>
						{isRemoved ? (
							<Badge className="border-transparent bg-muted text-muted-foreground">
								Removed
							</Badge>
						) : (
							container && (
								<>
									<Badge
										className={`border-transparent ${getStateBadgeClass(container.state)}`}
									>
										{label}
									</Badge>
									{container.health && (
										<Badge
											className={`border-transparent ${getHealthBadgeClass(container.health)}`}
										>
											{toTitleCase(container.health)}
										</Badge>
									)}
									{inspect?.State.OOMKilled && (
										<Badge className="border-transparent bg-rose-500/10 text-rose-700 dark:text-rose-400">
											Out of memory
										</Badge>
									)}
								</>
							)
						)}
					</div>
					<p className="mt-1 truncate text-base/6 text-muted-foreground sm:text-sm/6">
						{isRemoved
							? "This container no longer exists. Its stored logs are still readable."
							: meta.join(" · ")}
					</p>
				</div>

				{container && (
					<div className="flex flex-wrap items-center gap-2 sm:shrink-0">
						{isReadOnly && (
							<p className="text-base text-muted-foreground sm:text-sm">
								Read-only
							</p>
						)}
						{state === "running" && (
							<Button
								variant="outline"
								disabled={isReadOnly}
								onClick={onOpenShell}
							>
								<TerminalIcon className="size-4" />
								Shell
							</Button>
						)}
						<Button
							variant="outline"
							disabled={isReadOnly || isActionPending}
							onClick={onRestart}
						>
							<RotateCwIcon className="size-4" />
							Restart
						</Button>
						<Button
							variant={isRunning ? "outline" : "default"}
							disabled={isReadOnly || isActionPending}
							onClick={isRunning ? onStop : onStart}
						>
							{isActionPending ? (
								<Spinner className="size-4" />
							) : isRunning ? (
								<SquareIcon className="size-4" />
							) : (
								<PlayIcon className="size-4" />
							)}
							{isRunning ? "Stop" : "Start"}
						</Button>
						{/* Destructive, so it stays quiet until you mean it. It
						    confirms before removing anything either way. */}
						<Button
							variant="ghost"
							disabled={isReadOnly || isActionPending}
							onClick={onDelete}
							className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
						>
							<Trash2Icon className="size-4" />
							Remove
						</Button>
					</div>
				)}
			</div>

			{vitals.length > 0 && (
				<div className="mt-5 flex flex-wrap items-center gap-x-8 gap-y-3 border-t border-border/70 pt-4">
					{vitals.map((vital) => (
						<Stat key={vital.label} {...vital} />
					))}
				</div>
			)}
		</header>
	);
}
