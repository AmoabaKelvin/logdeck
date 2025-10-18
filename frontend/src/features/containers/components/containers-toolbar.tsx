import {
  CalendarIcon,
  ChevronDownIcon,
  RefreshCcwIcon,
  XIcon,
} from "lucide-react";
import type { DateRange } from "react-day-picker";

import { Button } from "@/components/ui/button";
import { Calendar } from "@/components/ui/calendar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";

import { toTitleCase } from "./container-utils";
import type { GroupByOption, SortDirection } from "./container-utils";

interface ContainersToolbarProps {
  searchTerm: string;
  onSearchChange: (value: string) => void;
  stateFilter: string;
  onStateFilterChange: (value: string) => void;
  availableStates: string[];
  sortDirection: SortDirection;
  onSortDirectionChange: (direction: SortDirection) => void;
  groupBy: GroupByOption;
  onGroupByChange: (value: GroupByOption) => void;
  dateRange: DateRange | undefined;
  onDateRangeChange: (range: DateRange | undefined) => void;
  onDateRangeClear: () => void;
  onRefresh: () => void;
  isFetching: boolean;
}

export function ContainersToolbar({
  searchTerm,
  onSearchChange,
  stateFilter,
  onStateFilterChange,
  availableStates,
  sortDirection,
  onSortDirectionChange,
  groupBy,
  onGroupByChange,
  dateRange,
  onDateRangeChange,
  onDateRangeClear,
  onRefresh,
  isFetching,
}: ContainersToolbarProps) {
  const renderDateRange = () => {
    if (!dateRange?.from) {
      return <span>Date range</span>;
    }

    if (dateRange.to) {
      const from = dateRange.from.toLocaleDateString("en-US", {
        month: "short",
        day: "numeric",
      });
      const to = dateRange.to.toLocaleDateString("en-US", {
        month: "short",
        day: "numeric",
      });
      return (
        <>
          {from} - {to}
        </>
      );
    }

    return dateRange.from.toLocaleDateString("en-US", {
      month: "short",
      day: "numeric",
    });
  };

  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <Input
        type="search"
        value={searchTerm}
        onChange={(event) => onSearchChange(event.target.value)}
        placeholder="Search containers..."
        className="sm:max-w-sm"
      />
      <div className="flex items-center gap-2">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline" size="sm" className="h-9">
              {stateFilter === "all" ? "All states" : toTitleCase(stateFilter)}
              <ChevronDownIcon className="ml-2 size-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuRadioGroup
              value={stateFilter}
              onValueChange={onStateFilterChange}
            >
              <DropdownMenuRadioItem value="all">
                All states
              </DropdownMenuRadioItem>
              {availableStates.map((state) => (
                <DropdownMenuRadioItem key={state} value={state}>
                  {toTitleCase(state)}
                </DropdownMenuRadioItem>
              ))}
            </DropdownMenuRadioGroup>
          </DropdownMenuContent>
        </DropdownMenu>

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline" size="sm" className="h-9">
              {sortDirection === "desc" ? "Newest" : "Oldest"}
              <ChevronDownIcon className="ml-2 size-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuRadioGroup
              value={sortDirection}
              onValueChange={(value) =>
                onSortDirectionChange(value as SortDirection)
              }
            >
              <DropdownMenuRadioItem value="desc">
                Newest first
              </DropdownMenuRadioItem>
              <DropdownMenuRadioItem value="asc">Oldest first</DropdownMenuRadioItem>
            </DropdownMenuRadioGroup>
          </DropdownMenuContent>
        </DropdownMenu>

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline" size="sm" className="h-9">
              {groupBy === "compose" ? "By project" : "No grouping"}
              <ChevronDownIcon className="ml-2 size-4" />
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
              variant={dateRange?.from ? "default" : "outline"}
              size="sm"
              className="h-9 justify-start text-left font-normal"
            >
              <CalendarIcon className="mr-2 size-4" />
              {renderDateRange()}
              {dateRange?.from && (
                <XIcon
                  className="ml-2 size-4 hover:text-destructive"
                  onClick={(event) => {
                    event.stopPropagation();
                    onDateRangeClear();
                  }}
                />
              )}
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
          </PopoverContent>
        </Popover>

        <Button
          variant="ghost"
          size="sm"
          onClick={onRefresh}
          className="h-9 shrink-0"
        >
          <RefreshCcwIcon
            className={`size-4 ${isFetching ? "animate-spin" : ""}`}
          />
        </Button>
      </div>
    </div>
  );
}
