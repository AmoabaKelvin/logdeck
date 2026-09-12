import { Link } from "@tanstack/react-router";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuLabel,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
	ArrowLeftIcon,
	EllipsisVerticalIcon,
	PlayIcon,
	RotateCwIcon,
	SquareIcon,
	TerminalIcon,
	Trash2Icon,
} from "@/components/ui/icons";
import { Spinner } from "@/components/ui/spinner";
import type { ContainerInspect } from "../api/get-container-inspect";
import type { StatsSample } from "../hooks/use-stats-history";
import type { ContainerInfo, ContainerStats } from "../types";
import {
	formatCPUPercent,
	formatImageName,
	formatMemoryStats,
	formatRelativeCreated,
	getHealthBadgeClass,
	getStateBadgeClass,
	splitContainerStatus,
	toTitleCase,
} from "./container-utils";
import { Sparkline } from "./sparkline";

// Action buttons match the dashboard toolbar's control height so the app has
// one button size, not four.
const actionButtonClass = "h-10 py-2 pr-3 pl-2 text-base sm:h-9 sm:text-sm";

interface ContainerDetailHeaderProps {
	name: string;
	// Undefined while the live list loads, or for a container that no longer
	// exists and only survives as stored logs.
	container: ContainerInfo | undefined;
	isRemoved: boolean;
	isReadOnly: boolean;
	stats: ContainerStats | undefined;
	history: StatsSample[];
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
	history?: number[];
	tone?: "warn";
}

/** Label beside its reading — the same shape the dashboard header uses. */
function Stat({ label, value, detail, history, tone }: StatProps) {
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
			{history && <Sparkline values={history} />}
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
					history: history.map((sample) => sample.cpu),
				},
				{
					label: "Mem",
					value:
						stats?.memory_percent != null
							? `${stats.memory_percent.toFixed(1)}%`
							: "—",
					detail: formatMemoryStats(stats),
					history: history.map((sample) => sample.memoryPercent),
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
					<div className="flex shrink-0 items-center gap-2">
						<Button
							variant={isRunning ? "outline" : "default"}
							disabled={isReadOnly || isActionPending}
							onClick={isRunning ? onStop : onStart}
							className={actionButtonClass}
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
						<Button
							variant="outline"
							disabled={isReadOnly || isActionPending}
							onClick={onRestart}
							className={actionButtonClass}
						>
							<RotateCwIcon className="size-4" />
							Restart
						</Button>
						<DropdownMenu>
							<DropdownMenuTrigger asChild>
								<Button
									variant="ghost"
									size="icon-sm"
									aria-label={`More actions for ${name}`}
									className="size-10 sm:size-9"
								>
									<EllipsisVerticalIcon className="size-4" />
								</Button>
							</DropdownMenuTrigger>
							<DropdownMenuContent align="end" className="w-48">
								{isReadOnly && (
									<DropdownMenuLabel className="text-muted-foreground">
										Read-only mode
									</DropdownMenuLabel>
								)}
								{state === "running" && (
									<>
										<DropdownMenuItem
											disabled={isReadOnly}
											onClick={onOpenShell}
										>
											<TerminalIcon className="size-4" />
											Open shell
										</DropdownMenuItem>
										<DropdownMenuSeparator />
									</>
								)}
								<DropdownMenuItem
									variant="destructive"
									disabled={isReadOnly}
									onClick={onDelete}
								>
									<Trash2Icon className="size-4" />
									Remove container
								</DropdownMenuItem>
							</DropdownMenuContent>
						</DropdownMenu>
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
