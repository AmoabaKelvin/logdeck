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
import type { ContainerInfo, ContainerStatsMap } from "../types";
import type { RemovedContainerInfo } from "./container-utils";
import {
	formatBytes,
	formatContainerName,
	formatCreatedDate,
	formatImageName,
	formatRelativeCreated,
	getContainerUrlIdentifier,
	isCoolifyManaged,
	isRemovedContainer,
	splitContainerStatus,
} from "./container-utils";
import {
	cellClass,
	MetricsCell,
	PortsCell,
	StatusCell,
} from "./containers-table-cells";

export interface ContainerRowCallbacks {
	onStart: (container: ContainerInfo) => void;
	onStop: (container: ContainerInfo) => void;
	onRestart: (container: ContainerInfo) => void;
	onDelete: (container: ContainerInfo) => void;
	onViewLogs: (container: ContainerInfo) => void;
	onPurgeHistory: (container: RemovedContainerInfo) => void;
}

// States where the container is not doing anything. Paused and dead are left
// out on purpose: paused is a deliberate, current state, and dead is a fault
// worth noticing rather than fading.
const DORMANT_STATES = new Set(["exited", "created"]);

function RowActions({
	container,
	busy,
	isReadOnly,
	onStart,
	onStop,
	onRestart,
	onDelete,
	onViewLogs,
	onPurgeHistory,
}: ContainerRowCallbacks & {
	container: ContainerInfo;
	busy: boolean;
	isReadOnly: boolean;
}) {
	const state = container.state.toLowerCase();

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
}

export function ContainerRow({
	container,
	stats,
	history,
	busy,
	isReadOnly,
	...callbacks
}: ContainerRowCallbacks & {
	container: ContainerInfo;
	stats: ContainerStatsMap[string] | undefined;
	history: number[];
	busy: boolean;
	isReadOnly: boolean;
}) {
	const removed = isRemovedContainer(container);
	const { label, duration, exitCode } = splitContainerStatus(container);
	const name = formatContainerName(container.names);
	const shortImage = formatImageName(container.image);
	const isDormant =
		removed || DORMANT_STATES.has(container.state.toLowerCase());

	return (
		<tr
			className={`border-b border-border/60 hover:bg-muted/40 ${isDormant ? "text-muted-foreground" : ""}`}
		>
			{/* The usage cell already makes the row two lines, so the image is
			    free here. */}
			<td className={cellClass}>
				<div className="flex min-w-0 flex-col gap-1">
					<div className="flex min-w-0 items-center gap-2">
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
				<StatusCell container={container} label={label} exitCode={exitCode} />
			</td>

			<td className={`${cellClass} hidden text-muted-foreground md:table-cell`}>
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
					<MetricsCell stats={stats} history={history} />
				)}
			</td>

			<td className={cellClass}>
				<div className="flex items-center justify-end gap-0.5">
					<RowActions
						container={container}
						busy={busy}
						isReadOnly={isReadOnly}
						{...callbacks}
					/>
				</div>
			</td>
		</tr>
	);
}
