export const TIME_RANGE_PRESETS = [
  "all",
  "15m",
  "1h",
  "6h",
  "24h",
  "custom",
] as const;

export type TimeRangePreset = (typeof TIME_RANGE_PRESETS)[number];

export interface TimeRange {
  preset: TimeRangePreset;
  // ISO timestamps; only meaningful when preset is "custom".
  since: string | null;
  until: string | null;
}

export const ALL_TIME_RANGE: TimeRange = {
  preset: "all",
  since: null,
  until: null,
};

export const TIME_RANGE_PRESET_LABELS = {
  all: "All time",
  "15m": "Last 15 min",
  "1h": "Last hour",
  "6h": "Last 6 hours",
  "24h": "Last 24 hours",
  custom: "Custom range",
} satisfies Record<TimeRangePreset, string>;

const PRESET_DURATIONS_MS = {
  "15m": 15 * 60 * 1000,
  "1h": 60 * 60 * 1000,
  "6h": 6 * 60 * 60 * 1000,
  "24h": 24 * 60 * 60 * 1000,
} satisfies Record<Exclude<TimeRangePreset, "all" | "custom">, number>;

export interface ResolvedTimeRange {
  since?: string;
  until?: string;
}

// Resolve a time range into the since/until values the logs API accepts
// (RFC3339 timestamps). Relative presets are anchored to `now`.
export function resolveTimeRange(
  timeRange: TimeRange,
  now: number = Date.now(),
): ResolvedTimeRange {
  if (timeRange.preset === "all") {
    return {};
  }
  if (timeRange.preset === "custom") {
    return {
      since: timeRange.since ?? undefined,
      until: timeRange.until ?? undefined,
    };
  }
  return {
    since: new Date(now - PRESET_DURATIONS_MS[timeRange.preset]).toISOString(),
  };
}
