import { describe, expect, it } from "vitest";

import type { LogEntry } from "@/features/containers/api/get-container-logs-parsed";
import { mapRawRangeToGroupedRange } from "./animated-range";

const grouped = (continuationCount?: number): LogEntry => ({
	level: "INFO",
	message: "entry",
	...(continuationCount !== undefined ? { continuationCount } : {}),
});

describe("mapRawRangeToGroupedRange", () => {
	// Grouped rows spanning raw indices [0..2], [3], [4..5].
	const rows = [grouped(2), grouped(), grouped(1)];

	it("returns null when there is no active range", () => {
		expect(mapRawRangeToGroupedRange(rows, null)).toBeNull();
	});

	it("is the identity when no rows are grouped", () => {
		const flat = [grouped(), grouped(), grouped()];
		expect(mapRawRangeToGroupedRange(flat, { start: 1, end: 2 })).toEqual({
			start: 1,
			end: 2,
		});
	});

	it("maps raw indices inside a merged row to that row", () => {
		expect(mapRawRangeToGroupedRange(rows, { start: 4, end: 5 })).toEqual({
			start: 2,
			end: 2,
		});
	});

	it("includes an existing row that absorbed a new continuation line", () => {
		// Raw entries 2 (continuation of row 0) and 3 arrived in one flush.
		expect(mapRawRangeToGroupedRange(rows, { start: 2, end: 3 })).toEqual({
			start: 0,
			end: 1,
		});
	});

	it("returns null when the raw range lies past every grouped row", () => {
		expect(mapRawRangeToGroupedRange(rows, { start: 6, end: 8 })).toBeNull();
	});
});
