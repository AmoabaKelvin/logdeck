import { Button } from "@/components/logdeck-demo/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/logdeck-demo/ui/dropdown-menu";
import { ChevronDownIcon } from "@/components/logdeck-demo/ui/icons";

import type { StateCounts } from "./container-utils";
import { REMOVED_STATE, toTitleCase } from "./container-utils";

interface ContainersStateFilterProps {
  stateCounts: StateCounts;
  stateFilter: string;
  onStateFilterChange: (value: string) => void;
  className?: string;
}

const STATES: { state: string; label: string; countClass?: string }[] = [
  { state: "running", label: "Running" },
  { state: "exited", label: "Exited" },
  { state: "paused", label: "Paused" },
  {
    state: "restarting",
    label: "Restarting",
    countClass: "text-amber-600 dark:text-amber-400",
  },
  {
    state: "dead",
    label: "Dead",
    countClass: "text-rose-600 dark:text-rose-400",
  },
  { state: REMOVED_STATE, label: "Removed" },
];

export function ContainersStateFilter({
  stateCounts,
  stateFilter,
  onStateFilterChange,
  className,
}: ContainersStateFilterProps) {
  const total = Object.values(stateCounts).reduce(
    (sum, count) => sum + count,
    0,
  );
  const isAll = stateFilter === "all";
  const activeCount = isAll
    ? total
    : (stateCounts[stateFilter as keyof StateCounts] ?? 0);

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" data-active={!isAll} className={className}>
          {isAll ? "All states" : toTitleCase(stateFilter)}
          <span className="tabular-nums text-muted-foreground">
            {activeCount}
          </span>
          <ChevronDownIcon className="size-4 shrink-0 text-muted-foreground" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-52">
        <DropdownMenuRadioGroup
          value={stateFilter}
          onValueChange={onStateFilterChange}
        >
          <DropdownMenuRadioItem value="all">
            <span className="flex-1">All states</span>
            <span className="tabular-nums text-muted-foreground">{total}</span>
          </DropdownMenuRadioItem>

          <DropdownMenuSeparator />

          {STATES.map((entry) => {
            const count = stateCounts[entry.state as keyof StateCounts];
            // An empty state is not worth an entry, unless it is the one
            // currently filtered on.
            if (count === 0 && stateFilter !== entry.state) {
              return null;
            }

            return (
              <DropdownMenuRadioItem key={entry.state} value={entry.state}>
                <span className="flex-1">{entry.label}</span>
                <span
                  className={`tabular-nums ${entry.countClass ?? "text-muted-foreground"}`}
                >
                  {count}
                </span>
              </DropdownMenuRadioItem>
            );
          })}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
