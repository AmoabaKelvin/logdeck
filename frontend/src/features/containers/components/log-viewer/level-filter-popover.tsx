import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { CheckIcon, ChevronDownIcon } from "@/components/ui/icons";
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "@/components/ui/popover";
import type { LogLevel } from "@/features/containers/api/get-container-logs-parsed";
import { getLogLevelBadgeColor } from "@/features/containers/api/get-container-logs-parsed";

interface LevelFilterPopoverProps {
	selectedLevels: Set<LogLevel>;
	setSelectedLevels: (value: Set<LogLevel>) => void;
	// Levels offered for filtering: those present in the loaded logs (live), or
	// the full set the server accepts (history, where it already filtered).
	availableLogLevels: readonly LogLevel[];
	// Sizing comes from the host toolbar so every control on a row matches.
	className?: string;
}

export function LevelFilterPopover({
	selectedLevels,
	setSelectedLevels,
	availableLogLevels,
	className,
}: LevelFilterPopoverProps) {
	const [open, setOpen] = useState(false);

	const toggleLogLevel = (level: LogLevel) => {
		const next = new Set(selectedLevels);
		if (next.has(level)) {
			next.delete(level);
		} else {
			next.add(level);
		}
		setSelectedLevels(next);
	};

	return (
		<Popover open={open} onOpenChange={setOpen}>
			<PopoverTrigger asChild>
				<Button
					variant="outline"
					data-active={selectedLevels.size > 0}
					className={className}
				>
					Log level
					{selectedLevels.size > 0 && (
						<Badge variant="secondary" className="h-5 px-1.5 tabular-nums">
							{selectedLevels.size}
						</Badge>
					)}
					<ChevronDownIcon className="size-4 shrink-0 text-muted-foreground" />
				</Button>
			</PopoverTrigger>
			<PopoverContent align="start" className="w-56">
				<div className="space-y-3">
					<div>
						<h4 className="mb-2 text-sm font-medium">Log levels</h4>
						<div className="space-y-2">
							{availableLogLevels.length === 0 ? (
								<p className="text-xs text-muted-foreground">
									No log levels available
								</p>
							) : (
								availableLogLevels.map((level) => (
									<label
										key={level}
										className="flex items-center gap-2 cursor-pointer"
									>
										<button
											type="button"
											onClick={() => toggleLogLevel(level)}
											aria-pressed={selectedLevels.has(level)}
											className={`size-4 rounded border flex items-center justify-center ${
												selectedLevels.has(level)
													? "bg-primary border-primary"
													: "border-input"
											}`}
										>
											{selectedLevels.has(level) && (
												<CheckIcon className="size-3 text-primary-foreground" />
											)}
										</button>
										<Badge
											variant="outline"
											className={`text-xs ${getLogLevelBadgeColor(level)}`}
										>
											{level}
										</Badge>
									</label>
								))
							)}
						</div>
					</div>
					{selectedLevels.size > 0 && (
						<Button
							variant="outline"
							size="sm"
							onClick={() => setSelectedLevels(new Set())}
							className="w-full"
						>
							Clear filters
						</Button>
					)}
				</div>
			</PopoverContent>
		</Popover>
	);
}
