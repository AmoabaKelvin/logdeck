import { useEffect, useState } from "react";

import type { ContainerStats } from "../types";

export const MAX_SAMPLES = 60;

export interface StatsSample {
	cpu: number;
	memoryPercent: number;
}

export type StatsHistoryMap = Record<string, StatsSample[]>;

/**
 * Append the latest sample for each container, capping every buffer at
 * MAX_SAMPLES (oldest dropped) and evicting containers no longer present.
 */
export function appendSamples(
	history: StatsHistoryMap,
	stats: ContainerStats[],
): StatsHistoryMap {
	const next: StatsHistoryMap = {};
	for (const stat of stats) {
		next[stat.id] = [
			...(history[stat.id] ?? []),
			{ cpu: stat.cpu_percent, memoryPercent: stat.memory_percent },
		].slice(-MAX_SAMPLES);
	}
	return next;
}

/**
 * In-memory ring buffer of recent stats per container, fed from the stats
 * query data. History resets on page reload; that's fine.
 */
export function useContainerStatsHistory(
	stats: ContainerStats[] | undefined,
): StatsHistoryMap {
	const [history, setHistory] = useState<StatsHistoryMap>({});

	useEffect(() => {
		if (stats) setHistory((prev) => appendSamples(prev, stats));
	}, [stats]);

	return history;
}

/**
 * In-memory ring buffer of a single sampled reading (e.g. system usage).
 * Appends whenever the sample reference changes, capped at MAX_SAMPLES.
 */
export function useSampleHistory<T>(sample: T | undefined): T[] {
	const [history, setHistory] = useState<T[]>([]);

	useEffect(() => {
		if (sample !== undefined) {
			setHistory((prev) => [...prev, sample].slice(-MAX_SAMPLES));
		}
	}, [sample]);

	return history;
}
