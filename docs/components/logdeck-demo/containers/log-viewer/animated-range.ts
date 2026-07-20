import type { LogEntry } from "@/components/logdeck-demo/api/get-container-logs-parsed";

export interface IndexRange {
	start: number;
	end: number;
}

// The stream hook reports newly-appended rows as a range of RAW log indices,
// but the viewer renders grouped rows (continuations merged by
// groupRelatedLogEntries). A grouped row spans 1 + continuationCount raw
// entries, so a rendered row should animate iff its raw span overlaps the
// new-raw range — including an existing row that just absorbed a new
// continuation line.
export function mapRawRangeToGroupedRange(
	groupedEntries: LogEntry[],
	rawRange: IndexRange | null,
): IndexRange | null {
	if (!rawRange) return null;

	let rawIndex = 0;
	let start = -1;
	let end = -1;

	for (let i = 0; i < groupedEntries.length; i++) {
		const span = 1 + (groupedEntries[i].continuationCount ?? 0);
		const rowStart = rawIndex;
		const rowEnd = rawIndex + span - 1;
		if (rowEnd >= rawRange.start && rowStart <= rawRange.end) {
			if (start === -1) start = i;
			end = i;
		}
		rawIndex += span;
	}

	return start === -1 ? null : { start, end };
}
