import type { StateCounts } from "./container-utils";
import { REMOVED_STATE } from "./container-utils";

interface ContainersStateSummaryProps {
	stateCounts: StateCounts;
	stateFilter: string;
	onStateFilterChange: (value: string) => void;
}

const CHIPS: { state: string; label: string; dotClass: string }[] = [
	{ state: "running", label: "Running", dotClass: "bg-emerald-500" },
	{ state: "exited", label: "Exited", dotClass: "bg-muted" },
	{ state: "paused", label: "Paused", dotClass: "bg-amber-500" },
	{ state: "restarting", label: "Restarting", dotClass: "bg-blue-500" },
	{ state: "dead", label: "Dead", dotClass: "bg-rose-500" },
	{
		state: REMOVED_STATE,
		label: "Removed",
		dotClass: "bg-muted-foreground/50",
	},
];

export function ContainersStateSummary({
	stateCounts,
	stateFilter,
	onStateFilterChange,
}: ContainersStateSummaryProps) {
	return (
		<div className="flex flex-wrap items-center gap-3 text-sm">
			{CHIPS.map((chip) => {
				const count = stateCounts[chip.state as keyof StateCounts];
				if (count === 0) {
					return null;
				}

				const isActive = stateFilter === chip.state;

				return (
					<button
						key={chip.state}
						type="button"
						aria-pressed={isActive}
						onClick={() => onStateFilterChange(isActive ? "all" : chip.state)}
						className={`flex min-h-10 items-center gap-2 rounded-md border px-3 py-2 transition-colors hover:bg-muted/50 ${
							isActive ? "border-primary bg-muted/50" : "bg-card"
						}`}
					>
						<div className={`size-2 rounded-full ${chip.dotClass}`} />
						<span className="text-muted-foreground">{chip.label}</span>
						<span className="font-semibold">{count}</span>
					</button>
				);
			})}
		</div>
	);
}
