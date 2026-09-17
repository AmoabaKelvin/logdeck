import type { DateRange } from "react-day-picker";
import { activeControlClass, Button } from "@/components/ui/button";
import { Calendar } from "@/components/ui/calendar";
import {
	DropdownMenu,
	DropdownMenuCheckboxItem,
	DropdownMenuContent,
	DropdownMenuRadioGroup,
	DropdownMenuRadioItem,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
	CalendarIcon,
	ChevronDownIcon,
	RefreshCcwIcon,
	SearchIcon,
} from "@/components/ui/icons";
import { Input } from "@/components/ui/input";
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "@/components/ui/popover";
import {
	Tooltip,
	TooltipContent,
	TooltipTrigger,
} from "@/components/ui/tooltip";

import type { ColumnId } from "../hooks/use-table-columns";
import { TOGGLEABLE_COLUMNS } from "../hooks/use-table-columns";
import type { DockerHost } from "../types";
import type { GroupByOption, StateCounts } from "./container-utils";
import { ContainersStateFilter } from "./containers-state-filter";

interface ContainersToolbarProps {
	searchTerm: string;
	onSearchChange: (value: string) => void;
	hostFilter: string;
	onHostFilterChange: (value: string) => void;
	availableHosts: DockerHost[];
	groupBy: GroupByOption;
	onGroupByChange: (value: GroupByOption) => void;
	hiddenColumns: ReadonlySet<ColumnId>;
	onToggleColumn: (id: ColumnId) => void;
	dateRange: DateRange | undefined;
	onDateRangeChange: (range: DateRange | undefined) => void;
	onDateRangeClear: () => void;
	onRefresh: () => void;
	isFetching: boolean;
	stateCounts: StateCounts;
	stateFilter: string;
	onStateFilterChange: (value: string) => void;
}

function formatDay(date: Date) {
	return date.toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

export function ContainersToolbar({
	searchTerm,
	onSearchChange,
	hostFilter,
	onHostFilterChange,
	availableHosts,
	groupBy,
	onGroupByChange,
	hiddenColumns,
	onToggleColumn,
	dateRange,
	onDateRangeChange,
	onDateRangeClear,
	onRefresh,
	isFetching,
	stateCounts,
	stateFilter,
	onStateFilterChange,
}: ContainersToolbarProps) {
	const renderDateRange = () => {
		if (!dateRange?.from) {
			return "Created any time";
		}
		if (dateRange.to) {
			return `${formatDay(dateRange.from)} – ${formatDay(dateRange.to)}`;
		}
		return `From ${formatDay(dateRange.from)}`;
	};

	return (
		<div className="flex flex-wrap items-center gap-3">
			<div className="relative min-w-56 flex-1 sm:max-w-sm">
				<SearchIcon className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
				<Input
					type="search"
					name="container-search"
					aria-label="Search containers by name, image, or ID"
					value={searchTerm}
					onChange={(event) => onSearchChange(event.target.value)}
					placeholder="Search name, image, or ID…"
					className="pl-8"
				/>
			</div>

			<div className="ml-auto flex flex-wrap items-center gap-2">
				<ContainersStateFilter
					stateCounts={stateCounts}
					stateFilter={stateFilter}
					onStateFilterChange={onStateFilterChange}
					className={activeControlClass}
				/>

				{availableHosts.length > 1 && (
					<DropdownMenu>
						<DropdownMenuTrigger asChild>
							<Button
								variant="outline"
								data-active={hostFilter !== "all"}
								className={activeControlClass}
							>
								{hostFilter === "all" ? "All hosts" : hostFilter}
								<ChevronDownIcon className="size-4 shrink-0 text-muted-foreground" />
							</Button>
						</DropdownMenuTrigger>
						<DropdownMenuContent align="end">
							<DropdownMenuRadioGroup
								value={hostFilter}
								onValueChange={onHostFilterChange}
							>
								<DropdownMenuRadioItem value="all">
									All hosts
								</DropdownMenuRadioItem>
								{availableHosts.map((host) => (
									<DropdownMenuRadioItem key={host.name} value={host.name}>
										{host.name}
									</DropdownMenuRadioItem>
								))}
							</DropdownMenuRadioGroup>
						</DropdownMenuContent>
					</DropdownMenu>
				)}

				<DropdownMenu>
					<DropdownMenuTrigger asChild>
						<Button
							variant="outline"
							data-active={groupBy !== "none"}
							className={activeControlClass}
						>
							{groupBy === "compose" ? "By project" : "No grouping"}
							<ChevronDownIcon className="size-4 shrink-0 text-muted-foreground" />
						</Button>
					</DropdownMenuTrigger>
					<DropdownMenuContent align="end">
						<DropdownMenuRadioGroup
							value={groupBy}
							onValueChange={(value) => {
								if (value === "none" || value === "compose") {
									onGroupByChange(value);
								}
							}}
						>
							<DropdownMenuRadioItem value="none">
								No grouping
							</DropdownMenuRadioItem>
							<DropdownMenuRadioItem value="compose">
								By compose project
							</DropdownMenuRadioItem>
						</DropdownMenuRadioGroup>
					</DropdownMenuContent>
				</DropdownMenu>

				<Popover>
					<PopoverTrigger asChild>
						<Button
							variant="outline"
							data-active={Boolean(dateRange?.from)}
							className={activeControlClass}
						>
							<CalendarIcon className="size-4 shrink-0 text-muted-foreground" />
							{renderDateRange()}
						</Button>
					</PopoverTrigger>
					<PopoverContent className="w-auto p-0" align="end">
						<Calendar
							mode="range"
							defaultMonth={dateRange?.from}
							selected={dateRange}
							onSelect={onDateRangeChange}
							numberOfMonths={2}
						/>
						{dateRange?.from && (
							<div className="border-t p-2">
								<Button
									variant="ghost"
									size="sm"
									onClick={onDateRangeClear}
									className="w-full"
								>
									Clear date range
								</Button>
							</div>
						)}
					</PopoverContent>
				</Popover>

				<DropdownMenu>
					<DropdownMenuTrigger asChild>
						<Button variant="outline">
							Columns
							<ChevronDownIcon className="size-4 shrink-0 text-muted-foreground" />
						</Button>
					</DropdownMenuTrigger>
					<DropdownMenuContent align="end">
						{TOGGLEABLE_COLUMNS.map((column) => (
							<DropdownMenuCheckboxItem
								key={column.id}
								checked={!hiddenColumns.has(column.id)}
								onCheckedChange={() => onToggleColumn(column.id)}
								// Keep the menu open.
								onSelect={(event) => event.preventDefault()}
							>
								{column.label}
							</DropdownMenuCheckboxItem>
						))}
					</DropdownMenuContent>
				</DropdownMenu>

				<Tooltip>
					<TooltipTrigger asChild>
						<Button
							variant="outline"
							size="icon-sm"
							onClick={onRefresh}
							aria-label="Refresh containers"
						>
							<RefreshCcwIcon
								className={`size-4 ${isFetching ? "animate-spin" : ""}`}
							/>
						</Button>
					</TooltipTrigger>
					<TooltipContent>Refresh</TooltipContent>
				</Tooltip>
			</div>
		</div>
	);
}
