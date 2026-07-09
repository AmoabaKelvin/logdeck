import { describe, expect, it } from "vitest";

import { navigatePins, shiftPinsForDroppedLines } from "./use-log-pins";

describe("navigatePins", () => {
	// Pins at original indices 2, 5, 9; only 2 and 9 are visible after filtering.
	const sortedPins = [2, 5, 9];
	const visibleMap = new Map<number, number>([
		[2, 0],
		[9, 4],
	]);

	it("returns null when there are no pins", () => {
		expect(navigatePins([], 0, 1, visibleMap)).toBeNull();
	});

	it("steps forward and resolves the filtered index", () => {
		expect(navigatePins(sortedPins, 0, 1, visibleMap)).toEqual({
			pinnedIndex: 1,
			filteredIndex: -1,
		});
		expect(navigatePins(sortedPins, 1, 1, visibleMap)).toEqual({
			pinnedIndex: 2,
			filteredIndex: 4,
		});
	});

	it("wraps around in both directions", () => {
		expect(navigatePins(sortedPins, 2, 1, visibleMap)).toEqual({
			pinnedIndex: 0,
			filteredIndex: 0,
		});
		expect(navigatePins(sortedPins, 0, -1, visibleMap)).toEqual({
			pinnedIndex: 2,
			filteredIndex: 4,
		});
	});

	it("reports -1 for pins hidden by the current filters", () => {
		const target = navigatePins(sortedPins, 0, 1, new Map());

		expect(target).toEqual({ pinnedIndex: 1, filteredIndex: -1 });
	});
});

describe("shiftPinsForDroppedLines", () => {
	it("shifts every pin down by the dropped amount", () => {
		const next = shiftPinsForDroppedLines(new Set([5, 10, 20]), 5);

		expect(Array.from(next).sort((a, b) => a - b)).toEqual([0, 5, 15]);
	});

	it("drops pins that shift below zero", () => {
		const next = shiftPinsForDroppedLines(new Set([2, 8]), 5);

		expect(Array.from(next)).toEqual([3]);
	});

	it("returns the same set when nothing was dropped or no pins exist", () => {
		const pins = new Set([1, 2]);

		expect(shiftPinsForDroppedLines(pins, 0)).toBe(pins);
		const empty = new Set<number>();
		expect(shiftPinsForDroppedLines(empty, 3)).toBe(empty);
	});
});
