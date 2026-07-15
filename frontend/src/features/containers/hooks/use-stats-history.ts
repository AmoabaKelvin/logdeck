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

// Buffers live at module scope so history survives route changes (component
// unmounts) within one page load; only a reload resets them. The last-appended
// reference guards against double-appending when several mounted components
// receive the same react-query data.
let containerHistory: StatsHistoryMap = {};
let lastContainerStats: ContainerStats[] | undefined;

export function useContainerStatsHistory(
	stats: ContainerStats[] | undefined,
): StatsHistoryMap {
	const [history, setHistory] = useState<StatsHistoryMap>(
		() => containerHistory,
	);

	useEffect(() => {
		if (stats && stats !== lastContainerStats) {
			lastContainerStats = stats;
			containerHistory = appendSamples(containerHistory, stats);
		}
		setHistory(containerHistory);
	}, [stats]);

	return history;
}

export interface SystemUsageSample {
	cpuPercent: number;
	memoryPercent: number;
}

let systemHistory: SystemUsageSample[] = [];
let lastSystemSample: SystemUsageSample | undefined;

/**
 * In-memory ring buffer of system usage readings, appended whenever the
 * sample reference changes, capped at MAX_SAMPLES.
 */
export function useSystemUsageHistory(
	sample: SystemUsageSample | undefined,
): SystemUsageSample[] {
	const [history, setHistory] = useState<SystemUsageSample[]>(
		() => systemHistory,
	);

	useEffect(() => {
		if (sample && sample !== lastSystemSample) {
			lastSystemSample = sample;
			systemHistory = [...systemHistory, sample].slice(-MAX_SAMPLES);
		}
		setHistory(systemHistory);
	}, [sample]);

	return history;
}
