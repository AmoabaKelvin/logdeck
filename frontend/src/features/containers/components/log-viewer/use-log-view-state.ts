import {
	createParser,
	parseAsBoolean,
	parseAsInteger,
	parseAsIsoDateTime,
	parseAsString,
	parseAsStringLiteral,
	useQueryStates,
} from "nuqs";
import { useCallback, useMemo, useState } from "react";

import type { LogLevel } from "@/features/containers/api/get-container-logs-parsed";
import {
	ALL_TIME_RANGE,
	TIME_RANGE_PRESETS,
	type TimeRange,
} from "./time-range";

// Where the viewer reads logs from: the live container ("live") or the log
// store ("history"). Only the page variant can switch; the sheet stays live.
export const LOG_SOURCES = ["live", "history"] as const;
export type LogSource = (typeof LOG_SOURCES)[number];

// The view state LogViewer needs from its host. The sheet keeps it in local
// component state; the full-page route persists it to the URL.
export interface LogViewState {
	source: LogSource;
	setSource: (value: LogSource) => void;
	searchText: string;
	setSearchText: (value: string) => void;
	useRegex: boolean;
	setUseRegex: (value: boolean) => void;
	selectedLevels: Set<LogLevel>;
	setSelectedLevels: (value: Set<LogLevel>) => void;
	showTimestamps: boolean;
	setShowTimestamps: (value: boolean) => void;
	wrapText: boolean;
	setWrapText: (value: boolean) => void;
	logLines: number;
	setLogLines: (value: number) => void;
	timeRange: TimeRange;
	setTimeRange: (value: TimeRange) => void;
}

export function useLocalLogViewState(): LogViewState {
	const [source, setSource] = useState<LogSource>("live");
	const [searchText, setSearchText] = useState("");
	const [useRegex, setUseRegex] = useState(false);
	const [selectedLevels, setSelectedLevels] = useState<Set<LogLevel>>(
		new Set(),
	);
	const [showTimestamps, setShowTimestamps] = useState(true);
	const [wrapText, setWrapText] = useState(false);
	const [logLines, setLogLines] = useState(100);
	const [timeRange, setTimeRange] = useState<TimeRange>(ALL_TIME_RANGE);

	return {
		source,
		setSource,
		searchText,
		setSearchText,
		useRegex,
		setUseRegex,
		selectedLevels,
		setSelectedLevels,
		showTimestamps,
		setShowTimestamps,
		wrapText,
		setWrapText,
		logLines,
		setLogLines,
		timeRange,
		setTimeRange,
	};
}

// The levels the wire actually carries: the log parser normalises "WARNING" to
// "WARN", and the history endpoint rejects anything outside this set.
export const LOG_LEVELS: readonly LogLevel[] = [
	"TRACE",
	"DEBUG",
	"INFO",
	"WARN",
	"ERROR",
	"FATAL",
	"PANIC",
	"UNKNOWN",
];

// Comma-separated log levels, e.g. ?levels=ERROR,WARN
const parseAsLogLevels = createParser<Set<LogLevel>>({
	parse: (value) => {
		const levels = value
			.split(",")
			.filter((level): level is LogLevel =>
				LOG_LEVELS.includes(level as LogLevel),
			);
		return new Set(levels);
	},
	serialize: (value) => Array.from(value).sort().join(","),
	eq: (a, b) =>
		a.size === b.size && Array.from(a).every((level) => b.has(level)),
});

const logViewSearchParams = {
	source: parseAsStringLiteral(LOG_SOURCES).withDefault("live"),
	search: parseAsString.withDefault(""),
	regex: parseAsBoolean.withDefault(false),
	levels: parseAsLogLevels.withDefault(new Set<LogLevel>()),
	timestamps: parseAsBoolean.withDefault(true),
	wrap: parseAsBoolean.withDefault(false),
	lines: parseAsInteger.withDefault(100),
	range: parseAsStringLiteral(TIME_RANGE_PRESETS).withDefault("all"),
	since: parseAsIsoDateTime,
	until: parseAsIsoDateTime,
};

export function useUrlLogViewState(): LogViewState {
	const [params, setParams] = useQueryStates(logViewSearchParams, {
		history: "replace",
	});

	const setSource = useCallback(
		(value: LogSource) => {
			setParams({ source: value });
		},
		[setParams],
	);

	const setSearchText = useCallback(
		(value: string) => {
			setParams({ search: value });
		},
		[setParams],
	);

	const setUseRegex = useCallback(
		(value: boolean) => {
			setParams({ regex: value });
		},
		[setParams],
	);

	const setSelectedLevels = useCallback(
		(value: Set<LogLevel>) => {
			setParams({ levels: value });
		},
		[setParams],
	);

	const setShowTimestamps = useCallback(
		(value: boolean) => {
			setParams({ timestamps: value });
		},
		[setParams],
	);

	const setWrapText = useCallback(
		(value: boolean) => {
			setParams({ wrap: value });
		},
		[setParams],
	);

	const setLogLines = useCallback(
		(value: number) => {
			setParams({ lines: value });
		},
		[setParams],
	);

	const timeRange = useMemo<TimeRange>(
		() => ({
			preset: params.range,
			since: params.since ? params.since.toISOString() : null,
			until: params.until ? params.until.toISOString() : null,
		}),
		[params.range, params.since, params.until],
	);

	const setTimeRange = useCallback(
		(value: TimeRange) => {
			const isCustom = value.preset === "custom";
			setParams({
				range: value.preset,
				since: isCustom && value.since ? new Date(value.since) : null,
				until: isCustom && value.until ? new Date(value.until) : null,
			});
		},
		[setParams],
	);

	return {
		source: params.source,
		setSource,
		searchText: params.search,
		setSearchText,
		useRegex: params.regex,
		setUseRegex,
		selectedLevels: params.levels,
		setSelectedLevels,
		showTimestamps: params.timestamps,
		setShowTimestamps,
		wrapText: params.wrap,
		setWrapText,
		logLines: params.lines,
		setLogLines,
		timeRange,
		setTimeRange,
	};
}
