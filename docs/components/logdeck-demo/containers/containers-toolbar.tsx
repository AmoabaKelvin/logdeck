import type { DateRange } from "react-day-picker";
import { Button } from "@/components/logdeck-demo/ui/button";
import { Calendar } from "@/components/logdeck-demo/ui/calendar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/components/logdeck-demo/ui/dropdown-menu";
import {
  CalendarIcon,
  ChevronDownIcon,
  RefreshCcwIcon,
  SearchIcon,
} from "@/components/logdeck-demo/ui/icons";
import { Input } from "@/components/logdeck-demo/ui/input";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/logdeck-demo/ui/popover";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/logdeck-demo/ui/tooltip";

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
  dateRange: DateRange | undefined;
  onDateRangeChange: (range: DateRange | undefined) => void;
  onDateRangeClear: () => void;
  onRefresh: () => void;
  isFetching: boolean;
  stateCounts: StateCounts;
  stateFilter: string;
  onStateFilterChange: (value: string) => void;
}

const controlClass =
  "h-10 text-base sm:h-9 sm:text-sm data-[active=true]:bg-muted data-[active=true]:text-foreground";

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
          className="h-10 pl-8 sm:h-9"
        />
      </div>

      <div className="ml-auto flex min-w-0 flex-wrap items-center gap-2">
        <ContainersStateFilter
          stateCounts={stateCounts}
          stateFilter={stateFilter}
          onStateFilterChange={onStateFilterChange}
          className={controlClass}
        />

        {availableHosts.length > 1 && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="outline"
                data-active={hostFilter !== "all"}
                className={controlClass}
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
              className={controlClass}
            >
              {groupBy === "compose" ? "By project" : "No grouping"}
              <ChevronDownIcon className="size-4 shrink-0 text-muted-foreground" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuRadioGroup
              value={groupBy}
              onValueChange={(value) => onGroupByChange(value as GroupByOption)}
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
              className={controlClass}
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

        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="outline"
              size="icon-sm"
              onClick={onRefresh}
              aria-label="Refresh containers"
              className="size-10 sm:size-9"
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
