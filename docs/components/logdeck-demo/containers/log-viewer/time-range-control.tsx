import { format } from "date-fns";
import { ClockIcon } from "lucide-react";
import { useId, useState } from "react";
import type { DateRange } from "react-day-picker";

import { Button } from "@/components/logdeck-demo/ui/button";
import { Calendar } from "@/components/logdeck-demo/ui/calendar";
import { Input } from "@/components/logdeck-demo/ui/input";
import { Label } from "@/components/logdeck-demo/ui/label";
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "@/components/logdeck-demo/ui/popover";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/logdeck-demo/ui/select";
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

function combineDateAndTime(day: Date, time: string): string {
	const [hours = 0, minutes = 0] = time.split(":").map(Number);
	const date = new Date(day);
	date.setHours(hours, minutes, 0, 0);
	return date.toISOString();
}

function toTimeValue(iso: string | null, fallback: string): string {
	if (!iso) return fallback;
	const date = new Date(iso);
	if (Number.isNaN(date.getTime())) return fallback;
	return format(date, "HH:mm");
}

function customRangeLabel(timeRange: TimeRange): string {
	if (!timeRange.since && !timeRange.until) return "Pick range";
	const fmt = (iso: string | null) =>
		iso ? format(new Date(iso), "MMM d, HH:mm") : "now";
	if (!timeRange.since) return `Until ${fmt(timeRange.until)}`;
	return `${fmt(timeRange.since)} – ${fmt(timeRange.until)}`;
}

export function TimeRangeControl({
	timeRange,
	setTimeRange,
	disabled = false,
}: TimeRangeControlProps) {
	const [isCustomOpen, setIsCustomOpen] = useState(false);
	const [draftRange, setDraftRange] = useState<DateRange | undefined>();
	const [sinceTime, setSinceTime] = useState("00:00");
	const [untilTime, setUntilTime] = useState("23:59");
	const sinceTimeId = useId();
	const untilTimeId = useId();

	const openCustomEditor = () => {
		const since = timeRange.since ? new Date(timeRange.since) : undefined;
		const until = timeRange.until ? new Date(timeRange.until) : undefined;
		setDraftRange(since || until ? { from: since, to: until } : undefined);
		setSinceTime(toTimeValue(timeRange.since, "00:00"));
		setUntilTime(toTimeValue(timeRange.until, "23:59"));
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
			since: draftRange?.from
				? combineDateAndTime(draftRange.from, sinceTime)
				: null,
			until: draftRange?.to
				? combineDateAndTime(draftRange.to, untilTime)
				: null,
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
							{customRangeLabel(timeRange)}
						</Button>
					</PopoverTrigger>
					<PopoverContent align="start" className="w-auto p-3">
						<Calendar
							mode="range"
							defaultMonth={draftRange?.from}
							selected={draftRange}
							onSelect={setDraftRange}
						/>
						<div className="mt-3 grid grid-cols-2 gap-3 border-t pt-3">
							<div className="space-y-1.5">
								<Label htmlFor={sinceTimeId} className="text-xs">
									From time
								</Label>
								<Input
									id={sinceTimeId}
									type="time"
									value={sinceTime}
									onChange={(e) => setSinceTime(e.target.value)}
									className="h-8 text-xs"
								/>
							</div>
							<div className="space-y-1.5">
								<Label htmlFor={untilTimeId} className="text-xs">
									To time
								</Label>
								<Input
									id={untilTimeId}
									type="time"
									value={untilTime}
									onChange={(e) => setUntilTime(e.target.value)}
									className="h-8 text-xs"
								/>
							</div>
						</div>
						<Button
							size="sm"
							onClick={applyCustomRange}
							className="mt-3 w-full"
						>
							Apply
						</Button>
					</PopoverContent>
				</Popover>
			)}
		</div>
	);
}
