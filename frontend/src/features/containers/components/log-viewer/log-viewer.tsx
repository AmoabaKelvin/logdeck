import { useVirtualizer } from "@tanstack/react-virtual";
import {
	ArrowDownIcon,
	ArrowDownToLineIcon,
	CheckIcon,
	ChevronDownIcon,
	ChevronLeftIcon,
	ChevronRightIcon,
	CopyIcon,
	DownloadIcon,
	EllipsisVerticalIcon,
	HelpCircleIcon,
	PauseIcon,
	PlayIcon,
	RefreshCcwIcon,
	SearchIcon,
	SquareIcon,
} from "lucide-react";
import type React from "react";
import {
	useCallback,
	useEffect,
	useId,
	useImperativeHandle,
	useMemo,
	useRef,
	useState,
} from "react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "@/components/ui/popover";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import {
	Tooltip,
	TooltipContent,
	TooltipTrigger,
} from "@/components/ui/tooltip";
import type {
	LogEntry,
	LogLevel,
} from "@/features/containers/api/get-container-logs-parsed";
import {
	getContainerLogsParsed,
	getLogLevelBadgeColor,
	groupRelatedLogEntries,
	streamContainerLogsParsed,
} from "@/features/containers/api/get-container-logs-parsed";
import { CollapsibleJson } from "@/features/containers/components/collapsible-json";
import { SelectionActionBar } from "@/features/containers/components/selection-action-bar";
import { useContainerLogStream } from "@/features/containers/hooks/use-container-log-stream";
import { isJsonString } from "@/lib/json-format";
import { resolveTimeRange } from "./time-range";
import { TimeRangeControl } from "./time-range-control";
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
	ref?: React.Ref<LogViewerHandle>;
}

const activeToggleButtonClass =
	"h-8 text-xs data-[active=true]:bg-muted data-[active=true]:text-foreground data-[active=true]:border-border data-[active=true]:ring-1 data-[active=true]:ring-primary/30 dark:data-[active=true]:bg-primary/15 dark:data-[active=true]:ring-primary/50 dark:data-[active=true]:border-primary/30";

function formatLogEntryLine(entry: LogEntry): string {
	const timestamp = entry.timestamp
		? new Date(entry.timestamp).toISOString()
		: "";
	const level = entry.level || "UNKNOWN";
	const message = entry.message || entry.raw || "";
	return `[${timestamp}] [${level}] ${message}`;
}

export function LogViewer({
	variant,
	containerId,
	host,
	containerName,
	viewState,
	ref,
}: LogViewerProps) {
	const {
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
	} = viewState;

	const [excludeMatches, setExcludeMatches] = useState(false);
	const [autoScroll, setAutoScroll] = useState(true);
	const [showFilters, setShowFilters] = useState(false);
	const [selectedIndices, setSelectedIndices] = useState<Set<number>>(
		new Set(),
	);
	const [expandedJsonRows, setExpandedJsonRows] = useState<Set<number>>(
		new Set(),
	);
	const [lastClickedIndex, setLastClickedIndex] = useState<number | null>(null);
	const parentRef = useRef<HTMLDivElement>(null);
	const searchInputRef = useRef<HTMLInputElement>(null);
	const autoScrollRef = useRef(autoScroll);
	const logLinesInputId = useId();

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
		if (autoScrollRef.current && containerEl) {
			if (behavior === "smooth") {
				containerEl.scrollTo({
					top: containerEl.scrollHeight,
					behavior: "smooth",
				});
				return;
			}
			containerEl.scrollTop = containerEl.scrollHeight;
		}
	}, []);

	// Anchor relative presets once per time-range change (not per render) so
	// the fetch callback identity stays stable between changes.
	const { since, until } = useMemo(
		() => resolveTimeRange(timeRange),
		[timeRange],
	);

	const {
		animatedRange,
		bufferedCount,
		droppedCount,
		fetchLogs,
		isLoadingLogs,
		isStreamPaused,
		isStreaming,
		logs: rawLogs,
		startStreaming,
		stopStreaming,
		togglePauseStreaming,
		toggleStreaming,
	} = useContainerLogStream<LogEntry>({
		containerId,
		host,
		tail: logLines,
		since,
		until,
		getLogs: getContainerLogsParsed,
		streamLogs: streamContainerLogsParsed,
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

	// The stream hook captures onResetState once; route it through a ref so it
	// always calls the current resetPins.
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
		return () => {
			stopStreaming();
		};
	}, [isStreaming, fetchLogs, stopStreaming]);

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

	const toggleLogLevel = (level: LogLevel) => {
		const newSet = new Set(selectedLevels);
		if (newSet.has(level)) {
			newSet.delete(level);
		} else {
			newSet.add(level);
		}
		setSelectedLevels(newSet);
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
		(index: number, event: React.MouseEvent) => {
			if (event.shiftKey && lastClickedIndex !== null) {
				// Shift-click: range selection
				const start = Math.min(lastClickedIndex, index);
				const end = Math.max(lastClickedIndex, index);
				const newSelected = new Set<number>();
				for (let i = start; i <= end; i++) {
					newSelected.add(i);
				}
				setSelectedIndices(newSelected);
			} else {
				// Regular click: toggle single selection and set anchor
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
			}
		},
		[lastClickedIndex],
	);

	const handleDownloadLogs = (format: "json" | "txt") => {
		if (filteredLogs.length === 0) {
			toast.error("No logs to download");
			return;
		}

		const cleanContainerName = (containerName || "container")
			.replace(/^\//, "")
			.replace(/[/\\:*?"<>|]/g, "-");
		const timestamp = new Date()
			.toISOString()
			.replace(/:/g, "-")
			.replace(/\..+/, "");
		const filename = `${cleanContainerName}-logs-${timestamp}.${format}`;
		let content: string;
		let mimeType: string;

		if (format === "json") {
			content = JSON.stringify(filteredLogs, null, 2);
			mimeType = "application/json";
		} else {
			content = filteredLogs.map(formatLogEntryLine).join("\n");
			mimeType = "text/plain";
		}

		const blob = new Blob([content], { type: mimeType });
		const url = URL.createObjectURL(blob);
		const a = document.createElement("a");
		a.href = url;
		a.download = filename;
		document.body.appendChild(a);
		a.click();
		document.body.removeChild(a);
		URL.revokeObjectURL(url);
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
			if (allSelectedArePinned) {
				selectedOriginalIndices.forEach((index) => {
					next.delete(index);
				});
			} else {
				selectedOriginalIndices.forEach((index) => {
					next.add(index);
				});
			}
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
		// Filter out any invalid indices as a safety measure
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

	// Clear selection when filters or search settings change
	// biome-ignore lint/correctness/useExhaustiveDependencies: intentionally clear selection on data changes
	useEffect(() => {
		clearSelection();
		setExpandedJsonRows(new Set());
	}, [searchText, excludeMatches, selectedLevels, useRegex]);

	// Virtualization setup (must be before navigation functions)
	const rowVirtualizer = useVirtualizer({
		count: filteredLogs.length,
		getScrollElement: () => parentRef.current,
		estimateSize: () => (wrapText ? 60 : 36),
		overscan: 5,
	});

	// Navigate to previous match
	const goToPreviousMatch = useCallback(() => {
		if (searchMatches.length === 0) return;
		const newIndex =
			currentMatchIndex > 0 ? currentMatchIndex - 1 : searchMatches.length - 1;
		setCurrentMatchIndex(newIndex);
		rowVirtualizer.scrollToIndex(searchMatches[newIndex], { align: "center" });
	}, [searchMatches, currentMatchIndex, setCurrentMatchIndex, rowVirtualizer]);

	// Navigate to next match
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
			const selected = Array.from(selectedIndices);
			const fallbackIndex =
				selected.length > 0
					? direction > 0
						? Math.max(...selected)
						: Math.min(...selected)
					: direction > 0
						? -1
						: filteredLogs.length;
			const baseIndex = lastClickedIndex ?? fallbackIndex;
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
			const minSelected = selected.length > 0 ? Math.min(...selected) : null;
			const maxSelected = selected.length > 0 ? Math.max(...selected) : null;

			let anchorIndex: number;
			if (lastClickedIndex !== null) {
				anchorIndex = lastClickedIndex;
			} else {
				if (selected.length > 0) {
					anchorIndex =
						direction > 0 ? (minSelected as number) : (maxSelected as number);
				} else {
					anchorIndex = direction > 0 ? 0 : filteredLogs.length - 1;
				}
				setLastClickedIndex(anchorIndex);
			}

			const activeIndex =
				selected.length > 0
					? direction > 0
						? (maxSelected as number)
						: (minSelected as number)
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

	const levelFilterPopover = (
		<Popover open={showFilters} onOpenChange={setShowFilters}>
			<PopoverTrigger asChild>
				<Button variant="outline" size="sm" className="h-8 text-xs">
					Log level
					{selectedLevels.size > 0 && (
						<Badge
							variant="secondary"
							className="ml-1.5 px-1 py-0 h-4 text-[10px] leading-none"
						>
							{selectedLevels.size}
						</Badge>
					)}
					<ChevronDownIcon className="ml-1 size-3.5 opacity-50" />
				</Button>
			</PopoverTrigger>
			<PopoverContent align="start" className="w-56">
				<div className="space-y-3">
					<div>
						<h4 className="text-sm font-medium mb-2">Log Levels</h4>
						<div className="space-y-2">
							{availableLogLevels.length === 0 ? (
								<p className="text-xs text-muted-foreground">
									No log levels available
								</p>
							) : (
								availableLogLevels.map((level) => (
									<label
										key={level}
										className="flex items-center gap-2 cursor-pointer"
									>
										<button
											type="button"
											onClick={() => toggleLogLevel(level)}
											aria-pressed={selectedLevels.has(level)}
											className={`size-4 rounded border flex items-center justify-center ${
												selectedLevels.has(level)
													? "bg-primary border-primary"
													: "border-input"
											}`}
										>
											{selectedLevels.has(level) && (
												<CheckIcon className="size-3 text-primary-foreground" />
											)}
										</button>
										<Badge
											variant="outline"
											className={`text-xs ${getLogLevelBadgeColor(level)}`}
										>
											{level}
										</Badge>
									</label>
								))
							)}
						</div>
					</div>
					{selectedLevels.size > 0 && (
						<Button
							variant="outline"
							size="sm"
							onClick={() => setSelectedLevels(new Set())}
							className="w-full"
						>
							Clear Filters
						</Button>
					)}
				</div>
			</PopoverContent>
		</Popover>
	);

	const pageToolbar = (
		<div className="space-y-3">
			{/* Row 1: Search + Lines + Stream/Refresh */}
			<div className="flex items-center gap-2">
				<CardTitle className="text-base shrink-0">
					Logs
					{filteredLogs.length !== logs.length && (
						<span className="ml-2 text-xs text-muted-foreground font-normal">
							({filteredLogs.length} of {logs.length})
						</span>
					)}
				</CardTitle>

				<div className="relative flex-1 min-w-[140px]">
					<SearchIcon className="absolute left-2.5 top-1/2 -translate-y-1/2 size-3.5 text-muted-foreground" />
					<Input
						ref={searchInputRef}
						placeholder="Search logs..."
						value={searchText}
						onChange={(e) => setSearchText(e.target.value)}
						className={`pl-8 h-8 text-xs ${useRegex && searchParsed.error ? "border-red-500 focus-visible:ring-red-500" : ""}`}
					/>
				</div>
				<Button
					variant={useRegex ? "secondary" : "ghost"}
					size="sm"
					onClick={() => setUseRegex(!useRegex)}
					aria-label={
						useRegex ? "Switch to plain text search" : "Switch to regex search"
					}
					aria-pressed={useRegex}
					className="h-8 w-8 p-0 font-mono text-xs shrink-0"
					title={
						useRegex ? "Switch to plain text search" : "Switch to regex search"
					}
				>
					.*
				</Button>
				{searchText && !excludeMatches && (
					<div className="flex items-center gap-0.5 shrink-0">
						<span className="text-xs tabular-nums text-muted-foreground whitespace-nowrap px-1">
							{searchMatches.length > 0
								? `${currentMatchIndex + 1} of ${searchMatches.length}`
								: "No matches"}
						</span>
						<Button
							variant="outline"
							size="sm"
							onClick={goToPreviousMatch}
							disabled={searchMatches.length === 0}
							aria-label="Previous match"
							className="h-8 w-8 p-0"
						>
							<ChevronLeftIcon className="size-3.5" />
						</Button>
						<Button
							variant="outline"
							size="sm"
							onClick={goToNextMatch}
							disabled={searchMatches.length === 0}
							aria-label="Next match"
							className="h-8 w-8 p-0"
						>
							<ChevronRightIcon className="size-3.5" />
						</Button>
					</div>
				)}

				{sortedPinnedIndices.length > 0 && (
					<div className="flex items-center gap-0.5 shrink-0">
						<span className="text-xs tabular-nums text-muted-foreground whitespace-nowrap px-1">
							{`${Math.min(currentPinnedIndex + 1, sortedPinnedIndices.length)} of ${sortedPinnedIndices.length} pinned`}
						</span>
						<Button
							variant="outline"
							size="sm"
							onClick={() => goToPinnedByOffset(-1)}
							aria-label="Previous pinned line"
							className="h-8 w-8 p-0"
						>
							<ChevronLeftIcon className="size-3.5" />
						</Button>
						<Button
							variant="outline"
							size="sm"
							onClick={() => goToPinnedByOffset(1)}
							aria-label="Next pinned line"
							className="h-8 w-8 p-0"
						>
							<ChevronRightIcon className="size-3.5" />
						</Button>
					</div>
				)}

				<div className="flex items-center gap-2 shrink-0">
					<Label
						htmlFor={logLinesInputId}
						className="text-xs text-muted-foreground"
					>
						Lines
					</Label>
					<Input
						id={logLinesInputId}
						type="text"
						inputMode="numeric"
						pattern="[0-9]*"
						value={logLines}
						onChange={(e) => handleLogLinesChange(e.target.value)}
						disabled={isStreaming}
						className="h-8 w-20 text-xs"
					/>
				</div>
				<Button
					variant="outline"
					size="sm"
					data-active={isStreaming}
					onClick={toggleStreaming}
					disabled={isLoadingLogs && !isStreaming}
					aria-pressed={isStreaming}
					className={`shrink-0 ${activeToggleButtonClass}`}
				>
					{isStreaming ? (
						<>
							<SquareIcon className="mr-2 size-4" />
							Stop
						</>
					) : (
						<>
							<PlayIcon className="mr-2 size-4" />
							Stream
						</>
					)}
				</Button>
				<Button
					variant="outline"
					size="sm"
					data-active={isStreamPaused}
					onClick={togglePauseStreaming}
					disabled={!isStreaming}
					aria-pressed={isStreamPaused}
					className={`shrink-0 ${activeToggleButtonClass}`}
				>
					{isStreamPaused ? (
						<>
							<PlayIcon className="mr-2 size-4" />
							Resume
							{bufferedCount > 0 && (
								<span className="ml-1 text-[10px] tabular-nums">
									({bufferedCount})
								</span>
							)}
						</>
					) : (
						<>
							<PauseIcon className="mr-2 size-4" />
							Pause
						</>
					)}
				</Button>
				<Button
					variant="outline"
					size="sm"
					onClick={handleRefresh}
					disabled={isStreaming || isLoadingLogs}
					aria-label="Refresh logs"
					className="shrink-0"
				>
					<RefreshCcwIcon className="size-4" />
				</Button>
			</div>
			<p className="text-[11px] text-muted-foreground">
				Shortcuts: <kbd className="font-mono">/</kbd> search,{" "}
				<kbd className="font-mono">j</kbd>/<kbd className="font-mono">k</kbd>{" "}
				lines, <kbd className="font-mono">n</kbd>/
				<kbd className="font-mono">N</kbd> matches,{" "}
				<kbd className="font-mono">p</kbd>/<kbd className="font-mono">P</kbd>{" "}
				pins
			</p>
			{/* Row 2: Options bar */}
			<div className="flex flex-wrap items-center gap-2">
				{searchText && (
					<Select
						value={excludeMatches ? "exclude" : "highlight"}
						onValueChange={(v) => setExcludeMatches(v === "exclude")}
					>
						<SelectTrigger size="sm" className="w-[160px] text-xs">
							<SelectValue />
						</SelectTrigger>
						<SelectContent>
							<SelectItem value="highlight">Highlight matches</SelectItem>
							<SelectItem value="exclude">Exclude matches</SelectItem>
						</SelectContent>
					</Select>
				)}

				{levelFilterPopover}

				<TimeRangeControl
					timeRange={timeRange}
					setTimeRange={setTimeRange}
					disabled={isStreaming}
				/>

				<Button
					variant="outline"
					size="sm"
					data-active={showTimestamps}
					onClick={() => setShowTimestamps(!showTimestamps)}
					aria-pressed={showTimestamps}
					className={activeToggleButtonClass}
				>
					Timestamps
				</Button>

				<Button
					variant="outline"
					size="sm"
					data-active={wrapText}
					onClick={() => setWrapText(!wrapText)}
					aria-pressed={wrapText}
					className={activeToggleButtonClass}
				>
					Wrap
				</Button>

				<Button
					variant="outline"
					size="sm"
					data-active={autoScroll}
					onClick={() => setAutoScroll(!autoScroll)}
					aria-pressed={autoScroll}
					className={activeToggleButtonClass}
				>
					{autoScroll ? (
						<ArrowDownToLineIcon className="mr-1.5 size-3.5" />
					) : (
						<ArrowDownIcon className="mr-1.5 size-3.5" />
					)}
					Auto-scroll
				</Button>

				<div className="flex-1" />

				<DropdownMenu>
					<DropdownMenuTrigger asChild>
						<Button variant="outline" size="sm" className="h-8 text-xs">
							<DownloadIcon className="mr-1.5 size-3.5" />
							Download
						</Button>
					</DropdownMenuTrigger>
					<DropdownMenuContent align="end">
						<DropdownMenuItem onClick={() => handleDownloadLogs("json")}>
							Download as JSON
						</DropdownMenuItem>
						<DropdownMenuItem onClick={() => handleDownloadLogs("txt")}>
							Download as TXT
						</DropdownMenuItem>
					</DropdownMenuContent>
				</DropdownMenu>
			</div>
		</div>
	);

	const sheetToolbar = (
		<div className="space-y-2">
			{/* Row 1: Search + Stream controls */}
			<div className="flex items-center gap-1.5">
				{/* Search input with inset regex toggle */}
				<div className="relative flex-1 min-w-[120px]">
					<SearchIcon className="absolute left-2.5 top-1/2 -translate-y-1/2 size-3.5 text-muted-foreground" />
					<Input
						ref={searchInputRef}
						placeholder={useRegex ? "Regex search..." : "Search logs..."}
						value={searchText}
						onChange={(e) => setSearchText(e.target.value)}
						className={`pl-8 pr-9 h-8 text-xs ${useRegex && searchParsed.error ? "border-red-500 focus-visible:ring-red-500" : ""}`}
					/>
					<button
						type="button"
						onClick={() => setUseRegex(!useRegex)}
						aria-label={
							useRegex
								? "Switch to plain text search"
								: "Switch to regex search"
						}
						aria-pressed={useRegex}
						title={useRegex ? "Switch to plain text" : "Switch to regex"}
						className={`absolute right-1.5 top-1/2 -translate-y-1/2 px-1 py-0.5 rounded text-[10px] font-mono leading-none transition-colors ${
							useRegex
								? "bg-primary text-primary-foreground"
								: "text-muted-foreground hover:text-foreground hover:bg-muted"
						}`}
					>
						.*
					</button>
				</div>

				{/* Controls: log level, time range, stream, auto-scroll, overflow */}
				<div className="flex items-center gap-1 shrink-0">
					{levelFilterPopover}

					<TimeRangeControl
						timeRange={timeRange}
						setTimeRange={setTimeRange}
						disabled={isStreaming}
					/>

					<Tooltip>
						<TooltipTrigger asChild>
							<Button
								variant="outline"
								size="sm"
								data-active={isStreaming}
								onClick={toggleStreaming}
								disabled={isLoadingLogs && !isStreaming}
								aria-label={isStreaming ? "Stop streaming" : "Start streaming"}
								aria-pressed={isStreaming}
								className={`h-8 w-8 p-0 ${activeToggleButtonClass}`}
							>
								{isStreaming ? (
									<SquareIcon className="size-3.5" />
								) : (
									<PlayIcon className="size-3.5" />
								)}
							</Button>
						</TooltipTrigger>
						<TooltipContent>
							{isStreaming ? "Stop streaming" : "Start streaming"}
						</TooltipContent>
					</Tooltip>
					{isStreaming && (
						<Tooltip>
							<TooltipTrigger asChild>
								<Button
									variant="outline"
									size="sm"
									data-active={isStreamPaused}
									onClick={togglePauseStreaming}
									aria-label={
										isStreamPaused ? "Resume streaming" : "Pause streaming"
									}
									aria-pressed={isStreamPaused}
									className={`h-8 w-8 p-0 ${activeToggleButtonClass}`}
								>
									{isStreamPaused ? (
										<PlayIcon className="size-3.5" />
									) : (
										<PauseIcon className="size-3.5" />
									)}
								</Button>
							</TooltipTrigger>
							<TooltipContent>
								{isStreamPaused
									? `Resume${bufferedCount > 0 ? ` (${bufferedCount} buffered)` : ""}`
									: "Pause streaming"}
							</TooltipContent>
						</Tooltip>
					)}
					<Tooltip>
						<TooltipTrigger asChild>
							<Button
								variant="outline"
								size="sm"
								onClick={handleRefresh}
								disabled={isStreaming || isLoadingLogs}
								aria-label="Refresh logs"
								className="h-8 w-8 p-0"
							>
								<RefreshCcwIcon className="size-3.5" />
							</Button>
						</TooltipTrigger>
						<TooltipContent>Refresh logs</TooltipContent>
					</Tooltip>
					<Tooltip>
						<TooltipTrigger asChild>
							<Button
								variant="outline"
								size="sm"
								data-active={autoScroll}
								onClick={() => setAutoScroll(!autoScroll)}
								aria-label={`Auto-scroll ${autoScroll ? "on" : "off"}`}
								aria-pressed={autoScroll}
								className={`h-8 w-8 p-0 ${activeToggleButtonClass}`}
							>
								{autoScroll ? (
									<ArrowDownToLineIcon className="size-3.5" />
								) : (
									<ArrowDownIcon className="size-3.5" />
								)}
							</Button>
						</TooltipTrigger>
						<TooltipContent>
							Auto-scroll {autoScroll ? "on" : "off"}
						</TooltipContent>
					</Tooltip>

					{/* Overflow menu: view options + download */}
					<DropdownMenu>
						<DropdownMenuTrigger asChild>
							<Button
								variant="ghost"
								size="sm"
								aria-label="More options"
								className="h-8 w-8 p-0"
							>
								<EllipsisVerticalIcon className="size-3.5" />
							</Button>
						</DropdownMenuTrigger>
						<DropdownMenuContent align="end" className="w-48">
							{searchText && (
								<>
									<DropdownMenuItem
										onClick={() => setExcludeMatches(!excludeMatches)}
									>
										<span className="flex-1">
											{excludeMatches ? "Highlight matches" : "Exclude matches"}
										</span>
									</DropdownMenuItem>
									<DropdownMenuSeparator />
								</>
							)}
							<DropdownMenuItem
								onClick={() => setShowTimestamps(!showTimestamps)}
							>
								<span className="flex-1">Timestamps</span>
								{showTimestamps && <CheckIcon className="size-3.5" />}
							</DropdownMenuItem>
							<DropdownMenuItem onClick={() => setWrapText(!wrapText)}>
								<span className="flex-1">Wrap lines</span>
								{wrapText && <CheckIcon className="size-3.5" />}
							</DropdownMenuItem>
							<DropdownMenuItem asChild>
								<div className="flex items-center gap-2">
									<span className="flex-1">Lines</span>
									<Input
										type="text"
										inputMode="numeric"
										pattern="[0-9]*"
										value={logLines}
										onChange={(e) => handleLogLinesChange(e.target.value)}
										disabled={isStreaming}
										className="h-6 w-16 text-xs text-center"
										onClick={(e) => e.stopPropagation()}
									/>
								</div>
							</DropdownMenuItem>
							<DropdownMenuSeparator />
							<DropdownMenuItem onClick={() => handleDownloadLogs("json")}>
								<DownloadIcon className="size-3.5 mr-2" />
								Download as JSON
							</DropdownMenuItem>
							<DropdownMenuItem onClick={() => handleDownloadLogs("txt")}>
								<DownloadIcon className="size-3.5 mr-2" />
								Download as TXT
							</DropdownMenuItem>
							<DropdownMenuSeparator />
							<DropdownMenuItem
								disabled
								className="text-[11px] text-muted-foreground"
							>
								<HelpCircleIcon className="size-3 mr-2" />/ search, j/k lines,
								n/N matches
							</DropdownMenuItem>
						</DropdownMenuContent>
					</DropdownMenu>
				</div>
			</div>

			{/* Conditional row: search match navigation + pinned navigation */}
			{(searchText && !excludeMatches && searchMatches.length > 0) ||
			sortedPinnedIndices.length > 0 ? (
				<div className="flex items-center gap-3">
					{searchText && !excludeMatches && (
						<div className="flex items-center gap-0.5">
							<span className="text-xs tabular-nums text-muted-foreground whitespace-nowrap px-0.5">
								{searchMatches.length > 0
									? `${currentMatchIndex + 1}/${searchMatches.length} matches`
									: "No matches"}
							</span>
							<Button
								variant="ghost"
								size="sm"
								onClick={goToPreviousMatch}
								disabled={searchMatches.length === 0}
								aria-label="Previous match"
								className="h-7 w-7 p-0"
							>
								<ChevronLeftIcon className="size-3.5" />
							</Button>
							<Button
								variant="ghost"
								size="sm"
								onClick={goToNextMatch}
								disabled={searchMatches.length === 0}
								aria-label="Next match"
								className="h-7 w-7 p-0"
							>
								<ChevronRightIcon className="size-3.5" />
							</Button>
						</div>
					)}
					{sortedPinnedIndices.length > 0 && (
						<div className="flex items-center gap-0.5">
							<span className="text-xs tabular-nums text-muted-foreground whitespace-nowrap px-0.5">
								{`${Math.min(currentPinnedIndex + 1, sortedPinnedIndices.length)}/${sortedPinnedIndices.length} pinned`}
							</span>
							<Button
								variant="ghost"
								size="sm"
								onClick={() => goToPinnedByOffset(-1)}
								aria-label="Previous pinned line"
								className="h-7 w-7 p-0"
							>
								<ChevronLeftIcon className="size-3.5" />
							</Button>
							<Button
								variant="ghost"
								size="sm"
								onClick={() => goToPinnedByOffset(1)}
								aria-label="Next pinned line"
								className="h-7 w-7 p-0"
							>
								<ChevronRightIcon className="size-3.5" />
							</Button>
						</div>
					)}
				</div>
			) : null}
		</div>
	);

	const logList = (
		<CardContent className="p-0 relative">
			{/* Selection Action Bar - sticky at top of logs */}
			<SelectionActionBar
				selectedCount={selectedIndices.size}
				onCopy={handleCopySelected}
				onTogglePin={handleTogglePinSelected}
				pinActionLabel={allSelectedArePinned ? "unpin" : "pin"}
				onClear={clearSelection}
			/>
			<div
				ref={parentRef}
				className={
					variant === "page"
						? "h-[calc(100vh-400px)] min-h-[400px] w-full overflow-auto"
						: "h-[400px] w-full overflow-auto"
				}
			>
				{isLoadingLogs && logs.length === 0 ? (
					<div className="flex items-center justify-center py-8 text-muted-foreground">
						<Spinner className="mr-2 size-4" />
						Loading logs...
					</div>
				) : logs.length === 0 ? (
					<div className="flex items-center justify-center py-8 text-muted-foreground text-sm">
						No logs available
					</div>
				) : filteredLogs.length === 0 ? (
					<div className="flex items-center justify-center py-8 text-muted-foreground text-sm">
						No logs match the current filters
					</div>
				) : (
					<div
						style={{
							height: `${rowVirtualizer.getTotalSize()}px`,
							width: "100%",
							position: "relative",
						}}
						className={`font-mono text-xs ${wrapText ? "" : "w-fit min-w-full"}`}
					>
						{rowVirtualizer.getVirtualItems().map((virtualRow) => {
							const entry = filteredLogs[virtualRow.index];
							const displayText = entry.message || entry.raw || "";
							if (!displayText.trim()) return null;

							const timestamp = entry.timestamp
								? new Date(entry.timestamp)
								: null;
							const dateLabel = timestamp
								? `${timestamp.toLocaleDateString("en-GB", {
										day: "2-digit",
										month: "2-digit",
										year: "numeric",
									})} ${timestamp.toLocaleTimeString("en-US", {
										hour12: false,
										hour: "2-digit",
										minute: "2-digit",
										second: "2-digit",
									})}`
								: "—";

							// Check if this row is the current search match
							const isCurrentMatch =
								searchMatches.length > 0 &&
								searchMatches[currentMatchIndex] === virtualRow.index;
							const hasMatch = searchMatchSet.has(virtualRow.index);
							const isSelected = selectedIndices.has(virtualRow.index);
							const isPinned = pinnedFilteredIndices.has(virtualRow.index);

							return (
								// biome-ignore lint/a11y/useSemanticElements: div required for virtual scrolling absolute positioning
								<div
									key={virtualRow.key}
									data-index={virtualRow.index}
									ref={rowVirtualizer.measureElement}
									role="button"
									tabIndex={0}
									onClick={(e) => handleLogClick(virtualRow.index, e)}
									onKeyDown={(e) => {
										if (e.key === "Enter" || e.key === " ") {
											e.preventDefault();
											handleLogClick(
												virtualRow.index,
												e as unknown as React.MouseEvent,
											);
										}
									}}
									style={{
										position: "absolute",
										top: 0,
										left: 0,
										width: wrapText ? "100%" : "max-content",
										minWidth: "100%",
										transform: `translateY(${virtualRow.start}px)`,
										cursor: "pointer",
									}}
									className={`group flex items-start gap-3 px-4 py-1.5 transition-all duration-150 ease-out ${
										wrapText ? "" : "whitespace-nowrap"
									} ${
										isSelected && isPinned
											? "bg-amber-200/80 dark:bg-amber-800/45 border-l-[3px] border-amber-500 shadow-[inset_0_0_0_1px_rgba(245,158,11,0.35)]"
											: isSelected
												? "bg-primary/[0.08] dark:bg-primary/[0.15] border-l-[3px] border-primary shadow-[inset_0_0_0_1px_rgba(var(--primary),0.1)]"
												: isPinned
													? "bg-amber-100/80 dark:bg-amber-900/35 border-l-[3px] border-amber-500 hover:bg-amber-100 dark:hover:bg-amber-900/45"
													: isCurrentMatch
														? "bg-yellow-100 dark:bg-yellow-900/30 border-y-2 border-yellow-400 dark:border-yellow-600"
														: virtualRow.index % 2 === 0
															? "bg-muted/30 border-l-[3px] border-transparent hover:bg-muted/50"
															: "border-l-[3px] border-transparent hover:bg-muted/50"
									} ${
										animatedRange &&
										virtualRow.index >= animatedRange.start &&
										virtualRow.index <= animatedRange.end
											? "log-stream-row-enter"
											: ""
									}`}
								>
									{showTimestamps && (
										<span className="text-muted-foreground shrink-0 text-[11px]">
											{dateLabel}
										</span>
									)}
									<Badge
										variant="outline"
										className={`shrink-0 text-xs px-1.5 py-0 h-5 ${getLogLevelBadgeColor(entry.level ?? "UNKNOWN")}`}
									>
										{entry.level ?? "UNKNOWN"}
									</Badge>
									<span
										className={`text-foreground flex-1 ${wrapText ? "whitespace-pre-wrap break-words" : "whitespace-pre"}`}
									>
										{isJsonString(displayText) ? (
											<CollapsibleJson
												text={displayText}
												isExpanded={expandedJsonRows.has(virtualRow.index)}
												onToggle={() => toggleJsonExpanded(virtualRow.index)}
												isCurrentMatch={isCurrentMatch}
												highlightSearchText={
													hasMatch ? highlightSearchText : undefined
												}
											/>
										) : hasMatch ? (
											highlightSearchText(displayText, isCurrentMatch)
										) : (
											displayText
										)}
									</span>
									<Tooltip>
										<TooltipTrigger asChild>
											<button
												type="button"
												onClick={(e) => {
													e.stopPropagation();
													handleCopyLog(entry);
												}}
												aria-label="Copy log entry"
												className="shrink-0 opacity-0 group-hover:opacity-100 transition-opacity p-1 hover:bg-muted rounded"
											>
												<CopyIcon className="size-3 text-muted-foreground" />
											</button>
										</TooltipTrigger>
										<TooltipContent>Copy log entry</TooltipContent>
									</Tooltip>
								</div>
							);
						})}
					</div>
				)}
			</div>
		</CardContent>
	);

	if (variant === "page") {
		return (
			<Card>
				<CardHeader>{pageToolbar}</CardHeader>
				{logList}
			</Card>
		);
	}

	return (
		<div className="space-y-3">
			{sheetToolbar}
			<Card>{logList}</Card>
		</div>
	);
}
