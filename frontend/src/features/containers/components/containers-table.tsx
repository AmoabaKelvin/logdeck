import { Link } from "@tanstack/react-router";
import { Fragment } from "react";
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
	ArrowUpIcon,
	EllipsisVerticalIcon,
	FileTextIcon,
	PlayIcon,
	RotateCwIcon,
	SquareIcon,
	Trash2Icon,
} from "@/components/ui/icons";
import { Spinner } from "@/components/ui/spinner";
import {
	Tooltip,
	TooltipContent,
	TooltipTrigger,
} from "@/components/ui/tooltip";
import type { ComposeAction } from "../api/compose-actions";
import type { StatsHistoryMap } from "../hooks/use-stats-history";
import type { ContainerInfo, ContainerPort, ContainerStatsMap } from "../types";
import type {
	ContainerActionType,
	GroupByOption,
	GroupedContainers,
	RemovedContainerInfo,
	SortDirection,
} from "./container-utils";
import {
	formatBytes,
	formatContainerName,
	formatCPUPercent,
	formatCreatedDate,
	formatImageName,
	formatMemoryStats,
	formatRelativeCreated,
	getComposeProject,
	getContainerUrlIdentifier,
	isCoolifyManaged,
	isRemovedContainer,
	splitContainerStatus,
} from "./container-utils";
import { Sparkline } from "./sparkline";

const COLUMN_COUNT = 7;

// Flush with the page container: only the inner gutters are padded.
const cellClass =
	"h-14 px-3 align-middle whitespace-nowrap first:pl-0 last:pr-0";
const headClass =
	"h-9 px-3 text-left align-middle font-medium whitespace-nowrap text-muted-foreground first:pl-0 last:pr-0";

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

// States where the container is not doing anything. Paused and dead are left
// out on purpose: paused is a deliberate, current state, and dead is a fault
// worth noticing rather than fading.
const DORMANT_STATES = new Set(["exited", "created"]);

function percent(value: number | undefined) {
	return value != null ? `${value.toFixed(1)}%` : "—";
}

/** Matching percentages so both readings align; absolute memory is on hover. */
function MetricsCell({
	stats,
	history,
}: {
	stats: ContainerStatsMap[string] | undefined;
	history: { cpu: number; memoryPercent: number }[];
}) {
	const rows = [
		{
			label: "CPU",
			value: formatCPUPercent(stats?.cpu_percent),
			title: undefined as string | undefined,
			values: history.map((sample) => sample.cpu),
		},
		{
			label: "Mem",
			value: percent(stats?.memory_percent),
			title: stats ? formatMemoryStats(stats) : undefined,
			values: history.map((sample) => sample.memoryPercent),
		},
	];

	return (
		<div className="flex flex-col gap-1 font-mono text-xs">
			{rows.map((row) => (
				<div key={row.label} className="flex items-center gap-2">
					<span className="w-7 shrink-0 text-[0.625rem] uppercase tracking-wide text-muted-foreground">
						{row.label}
					</span>
					<span
						className="w-12 shrink-0 text-right tabular-nums"
						title={row.title}
					>
						{row.value}
					</span>
					<Sparkline values={row.values} />
				</div>
			))}
		</div>
	);
}

const MAX_VISIBLE_PORTS = 2;

function PortsCell({ ports }: { ports: ContainerPort[] }) {
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

interface ContainersTableProps {
	isLoading: boolean;
	isError: boolean;
	error: unknown;
	groupBy: GroupByOption;
	sortDirection: SortDirection;
	onSortDirectionChange: (direction: SortDirection) => void;
	emptyMessage: string;
	hasActiveFilters: boolean;
	onClearFilters: () => void;
	filteredContainers: ContainerInfo[];
	groupedItems: GroupedContainers[] | null;
	pageItems: ContainerInfo[];
	pendingActions: ReadonlyMap<string, ContainerActionType>;
	pendingComposeActions: ReadonlyMap<string, ComposeAction>;
	isReadOnly: boolean;
	statsMap: ContainerStatsMap;
	statsHistory: StatsHistoryMap;
	onStart: (container: ContainerInfo) => void;
	onStop: (container: ContainerInfo) => void;
	onRestart: (container: ContainerInfo) => void;
	onDelete: (container: ContainerInfo) => void;
	onComposeAction: (action: ComposeAction, group: GroupedContainers) => void;
	onViewLogs: (container: ContainerInfo) => void;
	onPurgeHistory: (container: RemovedContainerInfo) => void;
	onRetry: () => void;
}

export function ContainersTable({
	isLoading,
	isError,
	error,
	groupBy,
	sortDirection,
	onSortDirectionChange,
	emptyMessage,
	hasActiveFilters,
	onClearFilters,
	filteredContainers,
	groupedItems,
	pageItems,
	pendingActions,
	pendingComposeActions,
	isReadOnly,
	statsMap,
	statsHistory,
	onStart,
	onStop,
	onRestart,
	onDelete,
	onComposeAction,
	onViewLogs,
	onPurgeHistory,
	onRetry,
}: ContainersTableProps) {
	// Only real compose groups get stack actions; the "Standalone" fallback
	// group has no compose project label to act on, and removed containers have
	// nothing left to start or stop.
	const isComposeGroup = (group: GroupedContainers) =>
		group.items.some(
			(container) =>
				!isRemovedContainer(container) &&
				getComposeProject(container.labels) === group.project,
		);

	const renderRowActions = (container: ContainerInfo) => {
		const state = container.state.toLowerCase();
		const pending = pendingActions.get(container.id);
		const busy = pending !== undefined;

		if (isRemovedContainer(container)) {
			return (
				<>
					<Tooltip>
						<TooltipTrigger asChild>
							<Button variant="ghost" size="icon-sm" asChild>
								<Link
									to="/containers/$containerId/logs"
									params={{
										containerId: getContainerUrlIdentifier(container),
									}}
									aria-label={`Stored logs for ${formatContainerName(container.names)}`}
								>
									<FileTextIcon className="size-4" />
								</Link>
							</Button>
						</TooltipTrigger>
						<TooltipContent>Stored logs</TooltipContent>
					</Tooltip>
					<Tooltip>
						<TooltipTrigger asChild>
							<Button
								variant="ghost"
								size="icon-sm"
								onClick={() => onPurgeHistory(container)}
								disabled={isReadOnly}
								aria-label={`Delete stored logs for ${formatContainerName(container.names)}`}
								className="text-muted-foreground hover:text-destructive"
							>
								<Trash2Icon className="size-4" />
							</Button>
						</TooltipTrigger>
						<TooltipContent>
							{isReadOnly
								? "Delete stored logs (read-only mode)"
								: "Delete stored logs"}
						</TooltipContent>
					</Tooltip>
				</>
			);
		}

		return (
			<>
				<Tooltip>
					<TooltipTrigger asChild>
						<Button
							variant="ghost"
							size="icon-sm"
							onClick={() => onViewLogs(container)}
							aria-label={`Logs for ${formatContainerName(container.names)}`}
						>
							<FileTextIcon className="size-4" />
						</Button>
					</TooltipTrigger>
					<TooltipContent>Logs</TooltipContent>
				</Tooltip>

				<DropdownMenu>
					<DropdownMenuTrigger asChild>
						<Button
							variant="ghost"
							size="icon-sm"
							disabled={busy}
							aria-label={`Actions for ${formatContainerName(container.names)}`}
						>
							{busy ? (
								<Spinner className="size-4" />
							) : (
								<EllipsisVerticalIcon className="size-4" />
							)}
						</Button>
					</DropdownMenuTrigger>
					<DropdownMenuContent align="end" className="w-48">
						{isReadOnly && (
							<DropdownMenuLabel className="text-muted-foreground">
								Read-only mode
							</DropdownMenuLabel>
						)}
						{state === "running" ? (
							<DropdownMenuItem
								disabled={isReadOnly}
								onClick={() => onStop(container)}
							>
								<SquareIcon className="size-4" />
								Stop
							</DropdownMenuItem>
						) : (
							<DropdownMenuItem
								disabled={isReadOnly}
								onClick={() => onStart(container)}
							>
								<PlayIcon className="size-4" />
								Start
							</DropdownMenuItem>
						)}
						<DropdownMenuItem
							disabled={isReadOnly}
							onClick={() => onRestart(container)}
						>
							<RotateCwIcon className="size-4" />
							Restart
						</DropdownMenuItem>
						<DropdownMenuSeparator />
						<DropdownMenuItem
							variant="destructive"
							disabled={isReadOnly}
							onClick={() => onDelete(container)}
						>
							<Trash2Icon className="size-4" />
							Remove container
						</DropdownMenuItem>
					</DropdownMenuContent>
				</DropdownMenu>
			</>
		);
	};

	const renderContainerRow = (container: ContainerInfo) => {
		const removed = isRemovedContainer(container);
		const { label, duration, exitCode } = splitContainerStatus(container);
		const tone = statusTone(container, exitCode);
		// Health qualifies the state, so it rides inside the same label.
		const statusText =
			container.health && container.health !== "healthy"
				? `${label} · ${container.health}`
				: label;
		const name = formatContainerName(container.names);
		const shortImage = formatImageName(container.image);
		const isDormant =
			removed || DORMANT_STATES.has(container.state.toLowerCase());

		return (
			<tr
				key={container.id}
				className={`border-b border-border/60 hover:bg-muted/40 ${isDormant ? "text-muted-foreground" : ""}`}
			>
				{/* The usage cell already makes the row two lines, so the image is
				    free here. */}
				<td className={cellClass}>
					<div className="flex flex-col gap-1">
						<div className="flex items-center gap-2">
							<Link
								to="/containers/$containerId/logs"
								params={{ containerId: getContainerUrlIdentifier(container) }}
								className="truncate font-medium hover:underline"
							>
								{name}
							</Link>
							{isCoolifyManaged(container.labels) && (
								<Badge className="h-4 shrink-0 border-0 bg-purple-500/10 px-1.5 text-[0.625rem] text-purple-700 dark:text-purple-400">
									Coolify
								</Badge>
							)}
						</div>
						{shortImage === container.image ? (
							<span className="truncate text-xs text-muted-foreground">
								{shortImage}
							</span>
						) : (
							<Tooltip>
								<TooltipTrigger asChild>
									<span className="truncate text-xs text-muted-foreground">
										{shortImage}
									</span>
								</TooltipTrigger>
								<TooltipContent className="max-w-md break-all">
									{container.image}
								</TooltipContent>
							</Tooltip>
						)}
					</div>
				</td>

				<td className={cellClass}>
					{tone === "quiet" ? (
						<span className="text-muted-foreground">{statusText}</span>
					) : (
						<Badge className={TONE_BADGE_CLASS[tone]}>{statusText}</Badge>
					)}
				</td>

				<td
					className={`${cellClass} hidden text-muted-foreground md:table-cell`}
				>
					{duration ?? "—"}
				</td>

				<td
					className={`${cellClass} hidden text-muted-foreground tabular-nums lg:table-cell`}
				>
					<span title={formatCreatedDate(container.created)}>
						{formatRelativeCreated(container.created)}
					</span>
				</td>

				<td className={`${cellClass} hidden lg:table-cell`}>
					<PortsCell ports={container.ports ?? []} />
				</td>

				<td className={`${cellClass} hidden sm:table-cell`}>
					{removed ? (
						<span className="font-mono text-xs text-muted-foreground">
							{container.storedBytes > 0
								? `${formatBytes(container.storedBytes)} stored`
								: "—"}
						</span>
					) : container.state.toLowerCase() !== "running" ? (
						<span className="text-muted-foreground">—</span>
					) : (
						<MetricsCell
							stats={statsMap[container.id]}
							history={statsHistory[container.id] ?? []}
						/>
					)}
				</td>

				<td className={cellClass}>
					<div className="flex items-center justify-end gap-0.5">
						{renderRowActions(container)}
					</div>
				</td>
			</tr>
		);
	};

	const renderMessageRow = (children: React.ReactNode) => (
		<tr>
			<td colSpan={COLUMN_COUNT} className="h-40 px-0">
				{children}
			</td>
		</tr>
	);

	const renderBodyRows = () => {
		if (isLoading) {
			return renderMessageRow(
				<div className="flex items-center justify-center gap-2 text-base text-muted-foreground sm:text-sm">
					<Spinner />
					Loading containers…
				</div>,
			);
		}

		if (isError) {
			return renderMessageRow(
				<div className="flex flex-col items-center gap-3 text-center">
					<p className="text-base text-muted-foreground sm:text-sm">
						{(error instanceof Error && error.message) ||
							"Unable to load containers."}
					</p>
					<Button size="sm" variant="outline" onClick={onRetry}>
						Try again
					</Button>
				</div>,
			);
		}

		if (filteredContainers.length === 0) {
			return renderMessageRow(
				<div className="flex flex-col items-center gap-3 text-center">
					<p className="text-base text-muted-foreground sm:text-sm">
						{hasActiveFilters
							? "No containers match these filters."
							: emptyMessage}
					</p>
					{hasActiveFilters && (
						<Button size="sm" variant="outline" onClick={onClearFilters}>
							Clear filters
						</Button>
					)}
				</div>,
			);
		}

		if (groupBy === "compose" && groupedItems) {
			return groupedItems.map((group) => {
				const busy = pendingComposeActions.has(group.project);

				return (
					<Fragment key={group.project}>
						<tr className="border-b border-border/60 bg-muted/40">
							<td colSpan={COLUMN_COUNT} className="h-10 px-0">
								<div className="flex items-center justify-between gap-3">
									<div className="flex min-w-0 items-baseline gap-2">
										<span className="truncate font-medium">
											{group.project}
										</span>
										<span className="shrink-0 text-xs text-muted-foreground tabular-nums">
											{group.items.length} container
											{group.items.length === 1 ? "" : "s"}
										</span>
									</div>
									<div className="flex shrink-0 items-center gap-0.5">
										{group.project !== "Standalone" && (
											<Button variant="ghost" size="sm" asChild>
												<Link
													to="/stacks/$project/logs"
													params={{ project: group.project }}
												>
													<FileTextIcon className="size-4" />
													Stack logs
												</Link>
											</Button>
										)}
										{isComposeGroup(group) && (
											<DropdownMenu>
												<DropdownMenuTrigger asChild>
													<Button
														variant="ghost"
														size="icon-sm"
														disabled={busy}
														aria-label={`Actions for ${group.project}`}
													>
														{busy ? (
															<Spinner className="size-4" />
														) : (
															<EllipsisVerticalIcon className="size-4" />
														)}
													</Button>
												</DropdownMenuTrigger>
												<DropdownMenuContent align="end" className="w-48">
													{isReadOnly && (
														<DropdownMenuLabel className="text-muted-foreground">
															Read-only mode
														</DropdownMenuLabel>
													)}
													<DropdownMenuItem
														disabled={isReadOnly}
														onClick={() => onComposeAction("start", group)}
													>
														<PlayIcon className="size-4" />
														Start stack
													</DropdownMenuItem>
													<DropdownMenuItem
														disabled={isReadOnly}
														onClick={() => onComposeAction("stop", group)}
													>
														<SquareIcon className="size-4" />
														Stop stack
													</DropdownMenuItem>
													<DropdownMenuItem
														disabled={isReadOnly}
														onClick={() => onComposeAction("restart", group)}
													>
														<RotateCwIcon className="size-4" />
														Restart stack
													</DropdownMenuItem>
												</DropdownMenuContent>
											</DropdownMenu>
										)}
									</div>
								</div>
							</td>
						</tr>
						{group.items.map(renderContainerRow)}
					</Fragment>
				);
			});
		}

		return pageItems.map(renderContainerRow);
	};

	return (
		<div className="-mx-4 -my-2 overflow-x-auto sm:-mx-6 lg:-mx-8">
			<div className="inline-block min-w-full px-4 py-2 align-middle sm:px-6 lg:px-8">
				<table className="w-full table-fixed text-sm">
					<thead>
						<tr className="border-b">
							<th className={`${headClass} w-[30%]`}>Container</th>
							<th className={`${headClass} w-[14%]`}>Status</th>
							<th className={`${headClass} hidden w-[11%] md:table-cell`}>
								Uptime
							</th>
							<th className={`${headClass} hidden w-[10%] lg:table-cell`}>
								<button
									type="button"
									onClick={() =>
										onSortDirectionChange(
											sortDirection === "desc" ? "asc" : "desc",
										)
									}
									aria-label={`Created, sorted ${sortDirection === "desc" ? "newest" : "oldest"} first`}
									className="inline-flex items-center gap-1 rounded-sm hover:text-foreground"
								>
									Created
									<ArrowUpIcon
										className={`size-3.5 shrink-0 ${sortDirection === "desc" ? "rotate-180" : ""}`}
									/>
								</button>
							</th>
							<th className={`${headClass} hidden w-[11%] lg:table-cell`}>
								Ports
							</th>
							<th className={`${headClass} hidden w-[18%] sm:table-cell`}>
								Usage
							</th>
							<th className={`${headClass} w-[6%]`}>
								<span className="sr-only">Actions</span>
							</th>
						</tr>
					</thead>
					<tbody>{renderBodyRows()}</tbody>
				</table>
			</div>
		</div>
	);
}
