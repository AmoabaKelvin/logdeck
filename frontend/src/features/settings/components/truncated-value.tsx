import {
	Tooltip,
	TooltipContent,
	TooltipProvider,
	TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

// Socket paths and API URLs are long enough to widen a table past the settings
// page, which has no horizontal scroll. The cell shows a capped, ellipsised
// value and keeps the full one a hover away. A table cell only shrinks when its
// content carries its own max width, hence the required cap.
interface TruncatedValueProps {
	value: string;
	/** Tailwind max-width cap for the cell, e.g. "max-w-[200px]". */
	className: string;
}

export function TruncatedValue({ value, className }: TruncatedValueProps) {
	return (
		<TooltipProvider>
			<Tooltip>
				<TooltipTrigger asChild>
					<span className={cn("block cursor-help truncate", className)}>
						{value}
					</span>
				</TooltipTrigger>
				<TooltipContent className="max-w-sm break-all">{value}</TooltipContent>
			</Tooltip>
		</TooltipProvider>
	);
}
