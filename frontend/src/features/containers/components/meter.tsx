import { TREND_WIDTH_CLASS } from "./sparkline";

const METER_TONE = {
	normal: { fill: "bg-foreground/50", track: "bg-foreground/10" },
	warn: { fill: "bg-amber-500", track: "bg-amber-500/15" },
	fail: { fill: "bg-rose-500", track: "bg-rose-500/15" },
} as const;

export function meterGeometry(used: number, limit: number) {
	const ratio = limit > 0 ? Math.min(used / limit, 1) : 0;

	return {
		tone: ratio >= 0.9 ? "fail" : ratio >= 0.75 ? "warn" : "normal",
		// 140KB of 512MB is 0.03%, a sub-pixel fill. The floor keeps it visible
		// without ever applying at zero, where none would read as some.
		width: ratio === 0 ? "0" : `max(2px, ${(ratio * 100).toFixed(1)}%)`,
	} as const;
}

/**
 * How close a reading is to its ceiling, as a length. Shared so the dashboard
 * row, the container header and the limits editor all colour the same usage
 * the same way.
 */
export function Meter({
	used,
	limit,
	className = TREND_WIDTH_CLASS,
}: {
	used: number;
	limit: number;
	className?: string;
}) {
	const { tone, width } = meterGeometry(used, limit);
	const { fill, track } = METER_TONE[tone];

	return (
		<div
			className={`h-1.5 shrink-0 overflow-hidden rounded-full ${track} ${className}`}
			aria-hidden="true"
		>
			<div
				className={`h-full rounded-full duration-600 ease-out motion-safe:transition-[width] ${fill}`}
				style={{ width }}
			/>
		</div>
	);
}
