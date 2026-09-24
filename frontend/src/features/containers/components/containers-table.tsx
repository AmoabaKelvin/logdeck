import { Link } from "@tanstack/react-router";
import { Fragment } from "react";
import { Button } from "@/components/ui/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuLabel,
	DropdownMenuRadioGroup,
	DropdownMenuRadioItem,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
	ChevronRightIcon,
	EllipsisVerticalIcon,
	FileTextIcon,
	PlayIcon,
	RotateCwIcon,
	SquareIcon,
} from "@/components/ui/icons";
import { Spinner } from "@/components/ui/spinner";
import type { ComposeAction } from "../api/compose-actions";
import type { StatsHistoryMap } from "../hooks/use-stats-history";
import type { ColumnId } from "../hooks/use-table-columns";
import type { ContainerInfo, ContainerStatsMap } from "../types";
import type {
	ContainerActionType,
	GroupByOption,
	GroupedContainers,
	SortDirection,
	SortKey,
} from "./container-utils";
import {
	getComposeProject,
	getSystemdUnit,
	isRemovedContainer,
} from "./container-utils";
import { headClass, SortButton } from "./containers-table-cells";
import type { ContainerRowCallbacks } from "./containers-table-row";
import { ContainerRow } from "./containers-table-row";

// Relative widths, rescaled over the visible columns.
const COLUMN_WEIGHTS = {
	container: 29,
	host: 10,
	status: 13,
	uptime: 10,
	created: 10,
	ports: 10,
	usage: 22,
	actions: 6,
} satisfies Record<ColumnId | "container" | "actions", number>;
const ALL_COLUMNS_WEIGHT = Object.values(COLUMN_WEIGHTS).reduce(
	(sum, weight) => sum + weight,
	0,
);

interface ContainersTableProps extends ContainerRowCallbacks {
	isLoading: boolean;
	isError: boolean;
	error: unknown;
	groupBy: GroupByOption;
	sortKey: SortKey;
	sortDirection: SortDirection;
	onSortChange: (key: SortKey, direction: SortDirection) => void;
	hiddenColumns: ReadonlySet<ColumnId>;
	collapsedGroups: ReadonlySet<string>;
	onToggleGroup: (project: string) => void;
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
	onComposeAction: (action: ComposeAction, group: GroupedContainers) => void;
	onRetry: () => void;
}

export function ContainersTable({
	isLoading,
	isError,
	error,
	groupBy,
	sortKey,
	sortDirection,
	onSortChange,
	hiddenColumns,
	collapsedGroups,
	onToggleGroup,
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
	onComposeAction,
	onRetry,
	...rowCallbacks
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

	const isSystemdGroup = (group: GroupedContainers) =>
		group.items.every(
			(container) =>
				isRemovedContainer(container) || getSystemdUnit(container.labels),
		);

	const columnCount = Object.keys(COLUMN_WEIGHTS).length - hiddenColumns.size;
	let visibleWeight = ALL_COLUMNS_WEIGHT;
	for (const id of hiddenColumns) visibleWeight -= COLUMN_WEIGHTS[id];
	const columnWidth = (id: keyof typeof COLUMN_WEIGHTS) => ({
		width: `${(COLUMN_WEIGHTS[id] / visibleWeight) * 100}%`,
	});

	const ariaSort = (...keys: SortKey[]) => {
		if (!keys.includes(sortKey)) return undefined;
		return sortDirection === "asc" ? "ascending" : "descending";
	};
	const usageKey = sortKey === "memory" ? "memory" : "cpu";
	const sortButton = (label: string, key: SortKey) => (
		<SortButton
			label={label}
			sortKey={key}
			activeKey={sortKey}
			direction={sortDirection}
			onSort={onSortChange}
		/>
	);

	const renderRow = (container: ContainerInfo, indented = false) => (
		<ContainerRow
			key={container.id}
			container={container}
			stats={statsMap[container.id]}
			history={statsHistory[container.id] ?? []}
			busy={pendingActions.has(container.id)}
			isReadOnly={isReadOnly}
			hiddenColumns={hiddenColumns}
			indented={indented}
			{...rowCallbacks}
		/>
	);

	const renderMessageRow = (children: React.ReactNode) => (
		<tr>
			<td colSpan={columnCount} className="h-40 px-0">
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
				const systemdGroup = isSystemdGroup(group);
				const collapsed = collapsedGroups.has(group.project);
				const running = group.items.filter(
					(container) => container.state.toLowerCase() === "running",
				).length;

				return (
					<Fragment key={group.project}>
						<tr className="border-b border-border/60 bg-muted/40">
							{/* oxlint-disable-next-line jsx-a11y/control-has-associated-label -- a plain group header cell, not a control; its text sits deeper than the rule looks */}
							<td colSpan={columnCount} className="h-10 px-0">
								<div className="flex items-center justify-between gap-3">
									<button
										type="button"
										onClick={() => onToggleGroup(group.project)}
										aria-expanded={!collapsed}
										className="flex min-w-0 items-center gap-2 rounded-sm text-left"
									>
										<ChevronRightIcon
											className={`size-4 shrink-0 text-muted-foreground ${collapsed ? "" : "rotate-90"}`}
										/>
										<span className="truncate font-semibold">
											{group.project}
										</span>
										<span className="shrink-0 text-xs text-muted-foreground tabular-nums">
											{group.items.length} container
											{group.items.length === 1 ? "" : "s"} · {running} running
										</span>
									</button>
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
													{isReadOnly ? (
														<DropdownMenuLabel className="text-muted-foreground">
															Read-only mode
														</DropdownMenuLabel>
													) : (
														systemdGroup && (
															<DropdownMenuLabel className="font-normal text-muted-foreground">
																Managed by systemd
															</DropdownMenuLabel>
														)
													)}
													<DropdownMenuItem
														disabled={isReadOnly}
														onClick={() => onComposeAction("start", group)}
													>
														<PlayIcon className="size-4" />
														Start stack
													</DropdownMenuItem>
													<DropdownMenuItem
														disabled={isReadOnly || systemdGroup}
														onClick={() => onComposeAction("stop", group)}
													>
														<SquareIcon className="size-4" />
														Stop stack
													</DropdownMenuItem>
													<DropdownMenuItem
														disabled={isReadOnly || systemdGroup}
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
						{!collapsed &&
							group.items.map((container) => renderRow(container, true))}
					</Fragment>
				);
			});
		}

		return pageItems.map((container) => renderRow(container));
	};

	return (
		<div className="-mx-4 -my-2 overflow-x-auto sm:-mx-6 lg:-mx-8">
			<div className="inline-block min-w-full px-4 py-2 align-middle sm:px-6 lg:px-8">
				<table className="w-full table-fixed text-sm">
					<thead>
						<tr className="border-b">
							<th
								className={headClass}
								style={columnWidth("container")}
								aria-sort={ariaSort("name")}
							>
								{sortButton("Container", "name")}
							</th>
							{!hiddenColumns.has("host") && (
								<th
									className={`${headClass} hidden md:table-cell`}
									style={columnWidth("host")}
									aria-sort={ariaSort("host")}
								>
									{sortButton("Host", "host")}
								</th>
							)}
							{!hiddenColumns.has("status") && (
								<th
									className={headClass}
									style={columnWidth("status")}
									aria-sort={ariaSort("status")}
								>
									{sortButton("Status", "status")}
								</th>
							)}
							{!hiddenColumns.has("uptime") && (
								<th
									className={`${headClass} hidden md:table-cell`}
									style={columnWidth("uptime")}
									aria-sort={ariaSort("uptime")}
								>
									{sortButton("Uptime", "uptime")}
								</th>
							)}
							{!hiddenColumns.has("created") && (
								<th
									className={`${headClass} hidden lg:table-cell`}
									style={columnWidth("created")}
									aria-sort={ariaSort("created")}
								>
									{sortButton("Created", "created")}
								</th>
							)}
							{!hiddenColumns.has("ports") && (
								<th
									className={`${headClass} hidden lg:table-cell`}
									style={columnWidth("ports")}
									aria-sort={ariaSort("ports")}
								>
									{sortButton("Ports", "ports")}
								</th>
							)}
							{!hiddenColumns.has("usage") && (
								<th
									className={`${headClass} hidden sm:table-cell`}
									style={columnWidth("usage")}
									aria-sort={ariaSort("cpu", "memory")}
								>
									<div className="flex items-center gap-1">
										{sortButton("Usage", usageKey)}
										<DropdownMenu>
											<DropdownMenuTrigger
												aria-label="Choose the usage measure to sort by"
												className="inline-flex items-center gap-1.5 rounded-sm font-normal hover:text-foreground"
											>
												<span
													aria-hidden="true"
													className="size-1 shrink-0 rounded-full bg-current"
												/>
												{usageKey === "cpu" ? "CPU" : "Memory"}
											</DropdownMenuTrigger>
											<DropdownMenuContent align="start">
												<DropdownMenuLabel className="text-muted-foreground">
													Sort usage by
												</DropdownMenuLabel>
												<DropdownMenuRadioGroup
													value={usageKey}
													onValueChange={(value) => {
														if (value === "cpu" || value === "memory") {
															onSortChange(
																value,
																sortKey === usageKey ? sortDirection : "desc",
															);
														}
													}}
												>
													<DropdownMenuRadioItem value="cpu">
														CPU
													</DropdownMenuRadioItem>
													<DropdownMenuRadioItem value="memory">
														Memory
													</DropdownMenuRadioItem>
												</DropdownMenuRadioGroup>
											</DropdownMenuContent>
										</DropdownMenu>
									</div>
								</th>
							)}
							<th className={headClass} style={columnWidth("actions")}>
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
