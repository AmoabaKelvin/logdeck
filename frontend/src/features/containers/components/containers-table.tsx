import { Link } from "@tanstack/react-router";
import { Fragment } from "react";
import { Button } from "@/components/ui/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuLabel,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
	ArrowUpIcon,
	EllipsisVerticalIcon,
	FileTextIcon,
	PlayIcon,
	RotateCwIcon,
	SquareIcon,
} from "@/components/ui/icons";
import { Spinner } from "@/components/ui/spinner";
import type { ComposeAction } from "../api/compose-actions";
import type { StatsHistoryMap } from "../hooks/use-stats-history";
import type { ContainerInfo, ContainerStatsMap } from "../types";
import type {
	ContainerActionType,
	GroupByOption,
	GroupedContainers,
	SortDirection,
} from "./container-utils";
import { getComposeProject, isRemovedContainer } from "./container-utils";
import { headClass } from "./containers-table-cells";
import type { ContainerRowCallbacks } from "./containers-table-row";
import { ContainerRow } from "./containers-table-row";

const COLUMN_COUNT = 7;

interface ContainersTableProps extends ContainerRowCallbacks {
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
	onComposeAction: (action: ComposeAction, group: GroupedContainers) => void;
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

	const renderRow = (container: ContainerInfo) => (
		<ContainerRow
			key={container.id}
			container={container}
			stats={statsMap[container.id]}
			history={statsHistory[container.id] ?? []}
			busy={pendingActions.has(container.id)}
			isReadOnly={isReadOnly}
			{...rowCallbacks}
		/>
	);

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
						{group.items.map(renderRow)}
					</Fragment>
				);
			});
		}

		return pageItems.map(renderRow);
	};

	return (
		<div className="-mx-4 -my-2 overflow-x-auto sm:-mx-6 lg:-mx-8">
			<div className="inline-block min-w-full px-4 py-2 align-middle sm:px-6 lg:px-8">
				<table className="w-full table-fixed text-sm">
					<thead>
						<tr className="border-b">
							<th className={`${headClass} w-[29%]`}>Container</th>
							<th className={`${headClass} w-[13%]`}>Status</th>
							<th className={`${headClass} hidden w-[10%] md:table-cell`}>
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
							<th className={`${headClass} hidden w-[10%] lg:table-cell`}>
								Ports
							</th>
							<th className={`${headClass} hidden w-[22%] sm:table-cell`}>
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
