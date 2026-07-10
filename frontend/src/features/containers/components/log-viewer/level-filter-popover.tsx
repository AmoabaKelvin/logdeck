import { CheckIcon, ChevronDownIcon } from "lucide-react";
import { useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
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
	// Levels actually present in the loaded logs.
	availableLogLevels: LogLevel[];
}

export function LevelFilterPopover({
	selectedLevels,
	setSelectedLevels,
	availableLogLevels,
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
				<Button variant="outline" size="sm" className="h-8 text-xs">
					Log level
					{selectedLevels.size > 0 && (
						<Badge
							variant="secondary"
							className="ml-1.5 px-1 py-0 h-4 text-[10px] leading-none"
						>
							{selectedLevels.size}
						</Badge>
					)}
					<ChevronDownIcon className="ml-1 size-3.5 opacity-50" />
				</Button>
			</PopoverTrigger>
			<PopoverContent align="start" className="w-56">
				<div className="space-y-3">
					<div>
						<h4 className="text-sm font-medium mb-2">Log Levels</h4>
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
							Clear Filters
						</Button>
					)}
				</div>
			</PopoverContent>
		</Popover>
	);
}
