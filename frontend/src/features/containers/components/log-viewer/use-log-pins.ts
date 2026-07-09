import { useCallback, useEffect, useMemo, useRef, useState } from "react";

export interface PinNavigationTarget {
	pinnedIndex: number;
	filteredIndex: number;
}

// Step through the sorted pins by offset, wrapping around. Returns the new
// position in the pin list plus the filtered row to scroll to (-1 when the
// pinned line is hidden by the current filters).
export function navigatePins(
	sortedPinnedIndices: number[],
	currentPinnedIndex: number,
	offset: 1 | -1,
	originalToFilteredIndex: Map<number, number>,
): PinNavigationTarget | null {
	if (sortedPinnedIndices.length === 0) return null;

	const pinnedIndex =
		(currentPinnedIndex + offset + sortedPinnedIndices.length) %
		sortedPinnedIndices.length;
	const filteredIndex =
		originalToFilteredIndex.get(sortedPinnedIndices[pinnedIndex]) ?? -1;

	return { pinnedIndex, filteredIndex };
}

// Pins are stored as indices into the log buffer; when the ring buffer drops
// the oldest entries those indices shift down by the dropped amount. (After
// grouping the shift is approximate, since dropped raw entries may have been
// merged into fewer grouped rows.)
export function shiftPinsForDroppedLines(
	pins: Set<number>,
	delta: number,
): Set<number> {
	if (delta <= 0 || pins.size === 0) return pins;
	const next = new Set<number>();
	pins.forEach((index) => {
		if (index - delta >= 0) {
			next.add(index - delta);
		}
	});
	return next;
}

export function useLogPins({
	droppedCount,
	filteredToOriginalIndex,
}: {
	droppedCount: number;
	filteredToOriginalIndex: number[];
}) {
	const [pinnedLogIndices, setPinnedLogIndices] = useState<Set<number>>(
		new Set(),
	);
	const [currentPinnedIndex, setCurrentPinnedIndex] = useState(0);

	const prevDroppedCountRef = useRef(0);
	useEffect(() => {
		const delta = droppedCount - prevDroppedCountRef.current;
		prevDroppedCountRef.current = droppedCount;
		if (delta <= 0) return;
		setPinnedLogIndices((prev) => shiftPinsForDroppedLines(prev, delta));
	}, [droppedCount]);

	const sortedPinnedIndices = useMemo(() => {
		return Array.from(pinnedLogIndices).sort((a, b) => a - b);
	}, [pinnedLogIndices]);

	const pinnedFilteredIndices = useMemo(() => {
		const next = new Set<number>();
		filteredToOriginalIndex.forEach((originalIndex, filteredIndex) => {
			if (pinnedLogIndices.has(originalIndex)) {
				next.add(filteredIndex);
			}
		});
		return next;
	}, [filteredToOriginalIndex, pinnedLogIndices]);

	useEffect(() => {
		if (sortedPinnedIndices.length === 0) {
			setCurrentPinnedIndex(0);
		} else if (currentPinnedIndex >= sortedPinnedIndices.length) {
			setCurrentPinnedIndex(sortedPinnedIndices.length - 1);
		}
	}, [sortedPinnedIndices, currentPinnedIndex]);

	const resetPins = useCallback(() => {
		setPinnedLogIndices(new Set());
		setCurrentPinnedIndex(0);
	}, []);

	return {
		pinnedLogIndices,
		setPinnedLogIndices,
		currentPinnedIndex,
		setCurrentPinnedIndex,
		sortedPinnedIndices,
		pinnedFilteredIndices,
		resetPins,
	};
}
