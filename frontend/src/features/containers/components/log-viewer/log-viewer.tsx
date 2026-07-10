import { useVirtualizer } from "@tanstack/react-virtual";
import type React from "react";
import {
	useCallback,
	useEffect,
	useImperativeHandle,
	useMemo,
	useRef,
	useState,
} from "react";
import { toast } from "sonner";

import { Card, CardHeader } from "@/components/ui/card";
import type { AggregateLogTarget } from "@/features/containers/api/get-aggregated-logs";
import {
	getAggregatedLogs,
	streamAggregatedLogs,
} from "@/features/containers/api/get-aggregated-logs";
import type {
	ContainerLogsOptions,
	LogEntry,
} from "@/features/containers/api/get-container-logs-parsed";
import {
	getContainerLogsParsed,
	groupRelatedLogEntries,
	streamContainerLogsParsed,
} from "@/features/containers/api/get-container-logs-parsed";
import { useContainerLogStream } from "@/features/containers/hooks/use-container-log-stream";
import { mapRawRangeToGroupedRange } from "./animated-range";
import { downloadLogs, formatLogEntryLine } from "./log-export";
import { LogList } from "./log-list";
import { PageToolbar } from "./page-toolbar";
import { SheetToolbar } from "./sheet-toolbar";
import { ShortcutHelpDialog } from "./shortcut-help";
import { resolveTimeRange } from "./time-range";
import type { LogViewerToolbarProps } from "./toolbar-shared";
import { useLogFiltering } from "./use-log-filtering";
import { navigatePins, useLogPins } from "./use-log-pins";
import { useLogSearch, useSearchMatches } from "./use-log-search";
import type { LogViewState } from "./use-log-view-state";

export interface LogViewerHandle {
	// Used after a container recreate: restart the stream (or refetch) so the
	// view follows the new container.
	refreshAfterRecreate: () => Promise<void>;
}

interface LogViewerProps {
	// "page" renders the full-width toolbar with labelled controls; "sheet"
	// renders the compact icon toolbar with an overflow menu.
	variant: "page" | "sheet";
	containerId?: string;
	host?: string;
	// Raw container name (may include the leading slash); used for download
	// filenames.
	containerName?: string;
	// Host-owned view state: local in the sheet, URL-persisted on the full
	// page (see use-log-view-state.ts).
	viewState: LogViewState;
	// Aggregate mode: merge these containers' logs into one view instead of
	// containerId/host. Entries carry containerName for the per-row badge.
	targets?: AggregateLogTarget[];
	ref?: React.Ref<LogViewerHandle>;
}

export function LogViewer({
	variant,
	containerId,
	host,
	containerName,
	viewState,
	targets,
	ref,
}: LogViewerProps) {
	const {
		searchText,
		useRegex,
		selectedLevels,
		showTimestamps,
		wrapText,
		logLines,
		setLogLines,
		timeRange,
	} = viewState;

	const [excludeMatches, setExcludeMatches] = useState(false);
	const [autoScroll, setAutoScroll] = useState(true);
	const [selectedIndices, setSelectedIndices] = useState<Set<number>>(
		new Set(),
	);
	const [expandedJsonRows, setExpandedJsonRows] = useState<Set<number>>(
		new Set(),
	);
	const [lastClickedIndex, setLastClickedIndex] = useState<number | null>(null);
	const [showShortcutHelp, setShowShortcutHelp] = useState(false);
	const parentRef = useRef<HTMLDivElement>(null);
	const searchInputRef = useRef<HTMLInputElement>(null);
	const autoScrollRef = useRef(autoScroll);

	const { searchParsed, highlightSearchText } = useLogSearch(
		searchText,
		useRegex,
	);

	// Keep ref in sync with state to avoid stale closures in setTimeout
	useEffect(() => {
		autoScrollRef.current = autoScroll;
	}, [autoScroll]);

	const scrollToBottom = useCallback((behavior: ScrollBehavior = "auto") => {
		const containerEl = parentRef.current;
		if (!autoScrollRef.current || !containerEl) return;
		if (behavior === "smooth") {
			containerEl.scrollTo({ top: containerEl.scrollHeight, behavior });
			return;
		}
		containerEl.scrollTop = containerEl.scrollHeight;
	}, []);

	// Anchor relative presets once per time-range change (not per render) so
	// the fetch callback identity stays stable between changes.
	const { since, until } = useMemo(
		() => resolveTimeRange(timeRange),
		[timeRange],
	);

	// Aggregate mode reuses the single-stream hook untouched: the targets are
	// baked into the fetch/stream functions, and a targets-derived key stands
	// in for containerId/host so the hook refetches when the set changes.
	const getLogs = useMemo(
		() =>
			targets
				? (_id: string, _host: string, options: ContainerLogsOptions) =>
						getAggregatedLogs(targets, options)
				: getContainerLogsParsed,
		[targets],
	);
	const streamLogs = useMemo(
		() =>
			targets
				? (
						_id: string,
						_host: string,
						options: ContainerLogsOptions,
						signal: AbortSignal,
					) => streamAggregatedLogs(targets, options, signal)
				: streamContainerLogsParsed,
		[targets],
	);
	const streamContainerId = targets
		? targets.map((t) => t.id).join(",")
		: containerId;
	const streamHost = targets ? "aggregate" : host;

	const {
		animatedRange,
		bufferedCount,
		droppedCount,
		fetchLogs,
		isLoadingLogs,
		isReconnecting,
		isStreamPaused,
		isStreaming,
		logs: rawLogs,
		startStreaming,
		stopStreaming,
		togglePauseStreaming,
		toggleStreaming,
	} = useContainerLogStream<LogEntry>({
		containerId: streamContainerId,
		host: streamHost,
		tail: logLines,
		since,
		until,
		getLogs,
		streamLogs,
		scrollToBottom,
		onResetState: () => {
			resetPinsRef.current();
		},
		onFetchError: (error) => {
			toast.error(`Failed to fetch logs: ${error.message}`);
		},
		onStreamError: (error) => {
			toast.error(`Failed to start streaming: ${error.message}`);
		},
	});

	const logs = useMemo(() => groupRelatedLogEntries(rawLogs), [rawLogs]);

	// The hook reports newly-appended rows in raw index space; translate to
	// the grouped rows actually rendered.
	const animatedGroupedRange = useMemo(
		() => mapRawRangeToGroupedRange(logs, animatedRange),
		[logs, animatedRange],
	);

	const {
		filteredLogs,
		filteredToOriginalIndex,
		originalToFilteredIndex,
		availableLogLevels,
	} = useLogFiltering({
		logs,
		selectedLevels,
		excludeMatches,
		searchText,
		useRegex,
		searchParsed,
	});

	const {
		pinnedLogIndices,
		setPinnedLogIndices,
		currentPinnedIndex,
		setCurrentPinnedIndex,
		sortedPinnedIndices,
		pinnedFilteredIndices,
		resetPins,
	} = useLogPins({ droppedCount, filteredToOriginalIndex });

	// resetPins can only be declared after the stream hook call (it needs
	// droppedCount), so onResetState reaches it through this ref.
	const resetPinsRef = useRef(resetPins);
	useEffect(() => {
		resetPinsRef.current = resetPins;
	}, [resetPins]);

	const {
		searchMatches,
		searchMatchSet,
		currentMatchIndex,
		setCurrentMatchIndex,
	} = useSearchMatches({ filteredLogs, searchText, useRegex, searchParsed });

	const handleRefresh = () => {
		if (!isStreaming) {
			fetchLogs();
		}
	};

	useEffect(() => {
		if (!isStreaming) {
			fetchLogs();
		}
	}, [isStreaming, fetchLogs]);

	// Teardown must run on unmount only. If stopStreaming were cleanup of the
	// fetch effect above, every isStreaming transition would re-run it and
	// abort a stream right as it starts.
	const stopStreamingRef = useRef(stopStreaming);
	useEffect(() => {
		stopStreamingRef.current = stopStreaming;
	}, [stopStreaming]);
	useEffect(() => {
		return () => {
			stopStreamingRef.current();
		};
	}, []);

	useImperativeHandle(
		ref,
		() => ({
			refreshAfterRecreate: async () => {
				if (isStreaming) {
					stopStreaming();
					await new Promise((resolve) => setTimeout(resolve, 100));
					void startStreaming();
				} else {
					await fetchLogs();
				}
			},
		}),
		[isStreaming, stopStreaming, startStreaming, fetchLogs],
	);

	const handleLogLinesChange = (value: string) => {
		const num = parseInt(value, 10);
		if (!Number.isNaN(num) && num > 0) {
			setLogLines(num);
		}
	};

	const handleCopyLog = (entry: LogEntry) => {
		const text = entry.message || entry.raw || "";
		navigator.clipboard
			.writeText(text)
			.then(() => {
				toast.success("Log entry copied to clipboard");
			})
			.catch(() => {
				toast.error("Failed to copy to clipboard");
			});
	};

	const clearSelection = useCallback(() => {
		setSelectedIndices(new Set());
		setLastClickedIndex(null);
	}, []);

	const toggleJsonExpanded = useCallback((index: number) => {
		setExpandedJsonRows((prev) => {
			const next = new Set(prev);
			if (next.has(index)) next.delete(index);
			else next.add(index);
			return next;
		});
	}, []);

	const handleLogClick = useCallback(
		(index: number, event: React.MouseEvent | React.KeyboardEvent) => {
			if (event.shiftKey && lastClickedIndex !== null) {
				const start = Math.min(lastClickedIndex, index);
				const end = Math.max(lastClickedIndex, index);
				const newSelected = new Set<number>();
				for (let i = start; i <= end; i++) {
					newSelected.add(i);
				}
				setSelectedIndices(newSelected);
				return;
			}

			setSelectedIndices((prev) => {
				const newSet = new Set(prev);
				if (newSet.has(index)) {
					newSet.delete(index);
				} else {
					newSet.add(index);
				}
				return newSet;
			});
			setLastClickedIndex(index);
		},
		[lastClickedIndex],
	);

	const handleDownloadLogs = (format: "json" | "txt") => {
		if (filteredLogs.length === 0) {
			toast.error("No logs to download");
			return;
		}
		downloadLogs(filteredLogs, containerName, format);
		toast.success(`Logs downloaded as ${format.toUpperCase()}`);
	};

	const selectedOriginalIndices = useMemo(() => {
		return Array.from(selectedIndices)
			.map((index) => filteredToOriginalIndex[index] ?? -1)
			.filter((index) => index >= 0);
	}, [selectedIndices, filteredToOriginalIndex]);

	const allSelectedArePinned = useMemo(() => {
		return (
			selectedOriginalIndices.length > 0 &&
			selectedOriginalIndices.every((index) => pinnedLogIndices.has(index))
		);
	}, [selectedOriginalIndices, pinnedLogIndices]);

	const handleTogglePinSelected = useCallback(() => {
		if (selectedOriginalIndices.length === 0) return;

		setPinnedLogIndices((prev) => {
			const next = new Set(prev);
			selectedOriginalIndices.forEach((index) => {
				if (allSelectedArePinned) {
					next.delete(index);
				} else {
					next.add(index);
				}
			});
			return next;
		});

		toast.success(
			allSelectedArePinned
				? `${selectedOriginalIndices.length} ${selectedOriginalIndices.length === 1 ? "line" : "lines"} unpinned`
				: `${selectedOriginalIndices.length} ${selectedOriginalIndices.length === 1 ? "line" : "lines"} pinned`,
		);
	}, [selectedOriginalIndices, allSelectedArePinned, setPinnedLogIndices]);

	const handleCopySelected = useCallback(() => {
		if (selectedIndices.size === 0) return;

		const sortedIndices = Array.from(selectedIndices).sort((a, b) => a - b);
		const validIndices = sortedIndices.filter(
			(idx) => idx >= 0 && idx < filteredLogs.length,
		);
		if (validIndices.length === 0) {
			clearSelection();
			return;
		}

		const content = validIndices
			.map((idx) => formatLogEntryLine(filteredLogs[idx]))
			.join("\n");

		navigator.clipboard
			.writeText(content)
			.then(() => {
				toast.success(`${validIndices.length} log entries copied to clipboard`);
			})
			.catch(() => {
				toast.error("Failed to copy to clipboard");
			});
	}, [selectedIndices, filteredLogs, clearSelection]);

	// biome-ignore lint/correctness/useExhaustiveDependencies: intentionally clear selection on data changes
	useEffect(() => {
		clearSelection();
		setExpandedJsonRows(new Set());
	}, [searchText, excludeMatches, selectedLevels, useRegex]);

	// The virtualizer must be created before the navigation callbacks that
	// scroll through it.
	const rowVirtualizer = useVirtualizer({
		count: filteredLogs.length,
		getScrollElement: () => parentRef.current,
		estimateSize: () => (wrapText ? 60 : 36),
		overscan: 5,
	});

	const goToPreviousMatch = useCallback(() => {
		if (searchMatches.length === 0) return;
		const newIndex =
			currentMatchIndex > 0 ? currentMatchIndex - 1 : searchMatches.length - 1;
		setCurrentMatchIndex(newIndex);
		rowVirtualizer.scrollToIndex(searchMatches[newIndex], { align: "center" });
	}, [searchMatches, currentMatchIndex, setCurrentMatchIndex, rowVirtualizer]);

	const goToNextMatch = useCallback(() => {
		if (searchMatches.length === 0) return;
		const newIndex =
			currentMatchIndex < searchMatches.length - 1 ? currentMatchIndex + 1 : 0;
		setCurrentMatchIndex(newIndex);
		rowVirtualizer.scrollToIndex(searchMatches[newIndex], { align: "center" });
	}, [searchMatches, currentMatchIndex, setCurrentMatchIndex, rowVirtualizer]);

	const goToPinnedByOffset = useCallback(
		(offset: 1 | -1) => {
			const target = navigatePins(
				sortedPinnedIndices,
				currentPinnedIndex,
				offset,
				originalToFilteredIndex,
			);
			if (!target) return;

			setCurrentPinnedIndex(target.pinnedIndex);

			if (target.filteredIndex === -1) {
				toast.info("Pinned line is hidden by current filters");
				return;
			}

			setSelectedIndices(new Set([target.filteredIndex]));
			setLastClickedIndex(target.filteredIndex);
			rowVirtualizer.scrollToIndex(target.filteredIndex, { align: "center" });
		},
		[
			sortedPinnedIndices,
			currentPinnedIndex,
			setCurrentPinnedIndex,
			originalToFilteredIndex,
			rowVirtualizer,
		],
	);

	const goToAdjacentLogLine = useCallback(
		(direction: 1 | -1) => {
			if (filteredLogs.length === 0) return;

			let baseIndex = lastClickedIndex;
			if (baseIndex === null) {
				const selected = Array.from(selectedIndices);
				if (selected.length > 0) {
					baseIndex =
						direction > 0 ? Math.max(...selected) : Math.min(...selected);
				} else {
					// Off the ends, so the first step lands on the first/last line.
					baseIndex = direction > 0 ? -1 : filteredLogs.length;
				}
			}
			const nextIndex = Math.min(
				filteredLogs.length - 1,
				Math.max(0, baseIndex + direction),
			);
			if (nextIndex === baseIndex) return;

			setSelectedIndices(new Set([nextIndex]));
			setLastClickedIndex(nextIndex);
			rowVirtualizer.scrollToIndex(nextIndex, { align: "center" });
		},
		[filteredLogs.length, lastClickedIndex, rowVirtualizer, selectedIndices],
	);

	const extendSelectionByLine = useCallback(
		(direction: 1 | -1) => {
			if (filteredLogs.length === 0) return;

			const selected = Array.from(selectedIndices);
			const minSelected = selected.length > 0 ? Math.min(...selected) : -1;
			const maxSelected = selected.length > 0 ? Math.max(...selected) : -1;

			// The anchor is the fixed end of the range; the active end moves.
			let anchorIndex: number;
			if (lastClickedIndex !== null) {
				anchorIndex = lastClickedIndex;
			} else if (selected.length > 0) {
				anchorIndex = direction > 0 ? minSelected : maxSelected;
				setLastClickedIndex(anchorIndex);
			} else {
				anchorIndex = direction > 0 ? 0 : filteredLogs.length - 1;
				setLastClickedIndex(anchorIndex);
			}

			const activeIndex =
				selected.length > 0
					? direction > 0
						? maxSelected
						: minSelected
					: anchorIndex;
			const targetIndex = Math.min(
				filteredLogs.length - 1,
				Math.max(0, activeIndex + direction),
			);
			if (targetIndex === activeIndex) return;

			const rangeStart = Math.min(anchorIndex, targetIndex);
			const rangeEnd = Math.max(anchorIndex, targetIndex);
			const nextSelected = new Set<number>();
			for (let i = rangeStart; i <= rangeEnd; i++) {
				nextSelected.add(i);
			}
			setSelectedIndices(nextSelected);
			rowVirtualizer.scrollToIndex(targetIndex, { align: "center" });
		},
		[filteredLogs.length, lastClickedIndex, rowVirtualizer, selectedIndices],
	);

	const focusSearchInput = useCallback(() => {
		searchInputRef.current?.focus();
		searchInputRef.current?.select();
	}, []);

	// The keydown listener is attached once; the handler reads the latest
	// navigation callbacks through a ref so callback identity changes don't
	// re-subscribe the listener on every render.
	const handleKeyDownRef = useRef<(event: KeyboardEvent) => void>(() => {});
	useEffect(() => {
		handleKeyDownRef.current = (event: KeyboardEvent) => {
			if (
				event.defaultPrevented ||
				event.metaKey ||
				event.ctrlKey ||
				event.altKey
			)
				return;
			const lowerKey = event.key.toLowerCase();

			const target = event.target;
			if (
				target instanceof HTMLElement &&
				(target.isContentEditable ||
					target.closest(
						"input, textarea, select, [contenteditable='true'], [role='textbox']",
					))
			) {
				return;
			}

			if (event.key === "?") {
				event.preventDefault();
				setShowShortcutHelp((prev) => !prev);
				return;
			}

			if (event.key === "/") {
				event.preventDefault();
				focusSearchInput();
				return;
			}

			if (lowerKey === "j" || event.key === "ArrowDown") {
				event.preventDefault();
				if (event.shiftKey) {
					extendSelectionByLine(1);
				} else {
					goToAdjacentLogLine(1);
				}
				return;
			}

			if (lowerKey === "k" || event.key === "ArrowUp") {
				event.preventDefault();
				if (event.shiftKey) {
					extendSelectionByLine(-1);
				} else {
					goToAdjacentLogLine(-1);
				}
				return;
			}

			if (lowerKey === "n") {
				event.preventDefault();
				if (event.shiftKey) {
					goToPreviousMatch();
				} else {
					goToNextMatch();
				}
				return;
			}

			if (lowerKey === "p") {
				event.preventDefault();
				if (event.shiftKey) {
					goToPinnedByOffset(-1);
				} else {
					goToPinnedByOffset(1);
				}
			}
		};
	}, [
		focusSearchInput,
		goToAdjacentLogLine,
		extendSelectionByLine,
		goToNextMatch,
		goToPinnedByOffset,
		goToPreviousMatch,
	]);

	useEffect(() => {
		const listener = (event: KeyboardEvent) => {
			handleKeyDownRef.current(event);
		};
		window.addEventListener("keydown", listener);
		return () => {
			window.removeEventListener("keydown", listener);
		};
	}, []);

	const toolbarProps: LogViewerToolbarProps = {
		viewState,
		searchParsed,
		searchInputRef,
		excludeMatches,
		setExcludeMatches,
		autoScroll,
		setAutoScroll,
		availableLogLevels,
		searchMatches,
		currentMatchIndex,
		onPreviousMatch: goToPreviousMatch,
		onNextMatch: goToNextMatch,
		sortedPinnedIndices,
		currentPinnedIndex,
		onNavigatePins: goToPinnedByOffset,
		isStreaming,
		isStreamPaused,
		isReconnecting,
		isLoadingLogs,
		bufferedCount,
		onToggleStreaming: toggleStreaming,
		onTogglePause: togglePauseStreaming,
		onRefresh: handleRefresh,
		onLogLinesChange: handleLogLinesChange,
		onDownload: handleDownloadLogs,
		onShowShortcutHelp: () => setShowShortcutHelp(true),
	};

	const logList = (
		<LogList
			variant={variant}
			parentRef={parentRef}
			rowVirtualizer={rowVirtualizer}
			isLoadingLogs={isLoadingLogs}
			totalCount={logs.length}
			filteredLogs={filteredLogs}
			filteredToOriginalIndex={filteredToOriginalIndex}
			wrapText={wrapText}
			showTimestamps={showTimestamps}
			searchMatches={searchMatches}
			searchMatchSet={searchMatchSet}
			currentMatchIndex={currentMatchIndex}
			selectedIndices={selectedIndices}
			pinnedFilteredIndices={pinnedFilteredIndices}
			animatedGroupedRange={animatedGroupedRange}
			expandedJsonRows={expandedJsonRows}
			highlightSearchText={highlightSearchText}
			onLogClick={handleLogClick}
			onToggleJson={toggleJsonExpanded}
			onCopyEntry={handleCopyLog}
			allSelectedArePinned={allSelectedArePinned}
			onCopySelected={handleCopySelected}
			onTogglePinSelected={handleTogglePinSelected}
			onClearSelection={clearSelection}
		/>
	);

	const shortcutHelpDialog = (
		<ShortcutHelpDialog
			open={showShortcutHelp}
			onOpenChange={setShowShortcutHelp}
		/>
	);

	if (variant === "page") {
		return (
			<Card>
				<CardHeader>
					<PageToolbar
						{...toolbarProps}
						totalCount={logs.length}
						filteredCount={filteredLogs.length}
					/>
				</CardHeader>
				{logList}
				{shortcutHelpDialog}
			</Card>
		);
	}

	return (
		<div className="space-y-3">
			<SheetToolbar {...toolbarProps} />
			<Card>{logList}</Card>
			{shortcutHelpDialog}
		</div>
	);
}
