import { useMemo } from "react";

import type {
	LogEntry,
	LogLevel,
} from "@/features/containers/api/get-container-logs-parsed";
import type { SearchParsed } from "./use-log-search";

export interface LogFilterOptions {
	selectedLevels: Set<LogLevel>;
	excludeMatches: boolean;
	searchText: string;
	useRegex: boolean;
	searchParsed: SearchParsed;
}

export interface FilteredLogItem {
	entry: LogEntry;
	originalIndex: number;
}

// Filter logs by level and optionally exclude search matches.
export function computeFilteredLogItems(
	logs: LogEntry[],
	{
		selectedLevels,
		excludeMatches,
		searchText,
		useRegex,
		searchParsed,
	}: LogFilterOptions,
): FilteredLogItem[] {
	const searchLower = searchText.toLowerCase();

	return logs
		.map((entry, originalIndex) => ({ entry, originalIndex }))
		.filter(({ entry }) => {
			if (selectedLevels.size > 0 && entry.level) {
				if (!selectedLevels.has(entry.level)) {
					return false;
				}
			}
			if (excludeMatches && searchText) {
				const message = entry.message || entry.raw || "";
				if (useRegex) {
					// An invalid regex excludes nothing rather than hiding every row.
					if (searchParsed.regex) {
						searchParsed.regex.lastIndex = 0;
						if (searchParsed.regex.test(message)) return false;
					}
				} else if (message.toLowerCase().includes(searchLower)) {
					return false;
				}
			}
			return true;
		});
}

export function useLogFiltering({
	logs,
	selectedLevels,
	excludeMatches,
	searchText,
	useRegex,
	searchParsed,
}: LogFilterOptions & { logs: LogEntry[] }) {
	const filteredLogItems = useMemo(
		() =>
			computeFilteredLogItems(logs, {
				selectedLevels,
				excludeMatches,
				searchText,
				useRegex,
				searchParsed,
			}),
		[logs, selectedLevels, excludeMatches, searchText, useRegex, searchParsed],
	);

	const filteredLogs = useMemo(
		() => filteredLogItems.map((item) => item.entry),
		[filteredLogItems],
	);

	const filteredToOriginalIndex = useMemo(
		() => filteredLogItems.map((item) => item.originalIndex),
		[filteredLogItems],
	);

	const originalToFilteredIndex = useMemo(() => {
		const map = new Map<number, number>();
		filteredToOriginalIndex.forEach((originalIndex, filteredIndex) => {
			map.set(originalIndex, filteredIndex);
		});
		return map;
	}, [filteredToOriginalIndex]);

	const availableLogLevels = useMemo(() => {
		const levels = new Set<LogLevel>();
		logs.forEach((entry) => {
			if (entry.level) {
				levels.add(entry.level);
			}
		});
		return Array.from(levels).sort();
	}, [logs]);

	return {
		filteredLogs,
		filteredToOriginalIndex,
		originalToFilteredIndex,
		availableLogLevels,
	};
}
