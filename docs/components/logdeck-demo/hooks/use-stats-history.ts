import { useEffect, useSyncExternalStore } from "react";

import { seedStatsHistory, seedSystemHistory } from "@/lib/logdeck-demo/store";

import type { ContainerStats } from "../types";

export const MAX_SAMPLES = 60;

export type StatsHistoryMap = Record<string, number[]>;

/**
 * Append the latest sample for each container, capping every buffer at
 * MAX_SAMPLES (oldest dropped) and evicting containers no longer present.
 */
export function appendSamples(
  history: StatsHistoryMap,
  stats: ContainerStats[],
) {
  const next: StatsHistoryMap = {};
  for (const stat of stats) {
    next[stat.id] = [...(history[stat.id] ?? []), stat.cpu_percent].slice(
      -MAX_SAMPLES,
    );
  }
  return next;
}

// Buffers live at module scope so history survives component unmounts within
// one page load. Unlike the real app they start pre-seeded: the demo should
// show trend lines immediately, not after a minute of polling.
let containerHistory: StatsHistoryMap = seedStatsHistory();
let lastContainerStats: ContainerStats[] | undefined;

// Hooks read the buffers through useSyncExternalStore; appending notifies them.
const listeners = new Set<() => void>();

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function notify() {
  for (const listener of listeners) {
    listener();
  }
}

export function useContainerStatsHistory(
  stats: ContainerStats[] | undefined,
): StatsHistoryMap {
  const history = useSyncExternalStore(
    subscribe,
    () => containerHistory,
    () => containerHistory,
  );

  useEffect(() => {
    if (stats && stats !== lastContainerStats) {
      lastContainerStats = stats;
      containerHistory = appendSamples(containerHistory, stats);
      notify();
    }
  }, [stats]);

  return history;
}

export interface SystemUsageSample {
  cpuPercent: number;
  memoryPercent: number;
}

let systemHistory: SystemUsageSample[] = seedSystemHistory();
let lastSystemSample: SystemUsageSample | undefined;

/**
 * In-memory ring buffer of system usage readings, appended whenever the
 * sample reference changes, capped at MAX_SAMPLES.
 */
export function useSystemUsageHistory(
  sample: SystemUsageSample | undefined,
): SystemUsageSample[] {
  const history = useSyncExternalStore(
    subscribe,
    () => systemHistory,
    () => systemHistory,
  );

  useEffect(() => {
    if (sample && sample !== lastSystemSample) {
      lastSystemSample = sample;
      systemHistory = [...systemHistory, sample].slice(-MAX_SAMPLES);
      notify();
    }
  }, [sample]);

  return history;
}
