import { ClockIcon } from "lucide-react";
import { useId, useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "@/components/ui/popover";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import {
	TIME_RANGE_PRESET_LABELS,
	TIME_RANGE_PRESETS,
	type TimeRange,
	type TimeRangePreset,
} from "./time-range";

interface TimeRangeControlProps {
	timeRange: TimeRange;
	setTimeRange: (value: TimeRange) => void;
	// Time bounds only apply to historical fetches, so the control is disabled
	// while live-streaming.
	disabled?: boolean;
}

// ISO timestamp <-> datetime-local input value (local time, minute precision).
function toDateTimeLocalValue(iso: string | null): string {
	if (!iso) return "";
	const date = new Date(iso);
	if (Number.isNaN(date.getTime())) return "";
	const pad = (value: number) => String(value).padStart(2, "0");
	return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function fromDateTimeLocalValue(value: string): string | null {
	if (!value) return null;
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) return null;
	return date.toISOString();
}

export function TimeRangeControl({
	timeRange,
	setTimeRange,
	disabled = false,
}: TimeRangeControlProps) {
	const [isCustomOpen, setIsCustomOpen] = useState(false);
	const [sinceDraft, setSinceDraft] = useState("");
	const [untilDraft, setUntilDraft] = useState("");
	const sinceInputId = useId();
	const untilInputId = useId();

	const openCustomEditor = () => {
		setSinceDraft(toDateTimeLocalValue(timeRange.since));
		setUntilDraft(toDateTimeLocalValue(timeRange.until));
		setIsCustomOpen(true);
	};

	const handlePresetChange = (value: string) => {
		const preset = value as TimeRangePreset;
		if (preset === "custom") {
			setTimeRange({ ...timeRange, preset: "custom" });
			openCustomEditor();
			return;
		}
		setTimeRange({ preset, since: null, until: null });
	};

	const applyCustomRange = () => {
		setTimeRange({
			preset: "custom",
			since: fromDateTimeLocalValue(sinceDraft),
			until: fromDateTimeLocalValue(untilDraft),
		});
		setIsCustomOpen(false);
	};

	return (
		<div className="flex items-center gap-1">
			<Select
				value={timeRange.preset}
				onValueChange={handlePresetChange}
				disabled={disabled}
			>
				<SelectTrigger size="sm" className="text-xs" aria-label="Time range">
					<ClockIcon className="size-3.5 text-muted-foreground" />
					<SelectValue />
				</SelectTrigger>
				<SelectContent>
					{TIME_RANGE_PRESETS.map((preset) => (
						<SelectItem key={preset} value={preset}>
							{TIME_RANGE_PRESET_LABELS[preset]}
						</SelectItem>
					))}
				</SelectContent>
			</Select>
			{timeRange.preset === "custom" && (
				<Popover open={isCustomOpen} onOpenChange={setIsCustomOpen}>
					<PopoverTrigger asChild>
						<Button
							variant="outline"
							size="sm"
							disabled={disabled}
							onClick={openCustomEditor}
							className="h-8 text-xs"
						>
							Edit
						</Button>
					</PopoverTrigger>
					<PopoverContent align="start" className="w-72 space-y-3">
						<div className="space-y-1.5">
							<Label htmlFor={sinceInputId} className="text-xs">
								From
							</Label>
							<Input
								id={sinceInputId}
								type="datetime-local"
								value={sinceDraft}
								onChange={(e) => setSinceDraft(e.target.value)}
								className="h-8 text-xs"
							/>
						</div>
						<div className="space-y-1.5">
							<Label htmlFor={untilInputId} className="text-xs">
								To
							</Label>
							<Input
								id={untilInputId}
								type="datetime-local"
								value={untilDraft}
								onChange={(e) => setUntilDraft(e.target.value)}
								className="h-8 text-xs"
							/>
						</div>
						<Button size="sm" onClick={applyCustomRange} className="w-full">
							Apply
						</Button>
					</PopoverContent>
				</Popover>
			)}
		</div>
	);
}
