import {
	ArrowDownIcon,
	ArrowDownToLineIcon,
	CheckIcon,
	ChevronLeftIcon,
	ChevronRightIcon,
	DownloadIcon,
	EllipsisVerticalIcon,
	HelpCircleIcon,
	PauseIcon,
	PlayIcon,
	RefreshCcwIcon,
	SearchIcon,
	SquareIcon,
} from "lucide-react";

import { Button } from "@/components/ui/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import {
	Tooltip,
	TooltipContent,
	TooltipTrigger,
} from "@/components/ui/tooltip";
import { LevelFilterPopover } from "./level-filter-popover";
import { TimeRangeControl } from "./time-range-control";
import {
	activeToggleButtonClass,
	type LogViewerToolbarProps,
} from "./toolbar-shared";

// Compact icon toolbar with an overflow menu, used inside the logs sheet.
export function SheetToolbar({
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
	onPreviousMatch,
	onNextMatch,
	sortedPinnedIndices,
	currentPinnedIndex,
	onNavigatePins,
	isStreaming,
	isStreamPaused,
	isReconnecting,
	isLoadingLogs,
	bufferedCount,
	onToggleStreaming,
	onTogglePause,
	onRefresh,
	onLogLinesChange,
	onDownload,
	onShowShortcutHelp,
}: LogViewerToolbarProps) {
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
		timeRange,
		setTimeRange,
	} = viewState;

	const showMatchNav = Boolean(searchText) && !excludeMatches;
	const showPinNav = sortedPinnedIndices.length > 0;

	return (
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
					{isReconnecting && (
						<span className="shrink-0 text-xs text-muted-foreground animate-pulse">
							Reconnecting…
						</span>
					)}
					<LevelFilterPopover
						selectedLevels={selectedLevels}
						setSelectedLevels={setSelectedLevels}
						availableLogLevels={availableLogLevels}
					/>

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
								onClick={onToggleStreaming}
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
									onClick={onTogglePause}
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
								onClick={onRefresh}
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
										onChange={(e) => onLogLinesChange(e.target.value)}
										disabled={isStreaming}
										className="h-6 w-16 text-xs text-center"
										onClick={(e) => e.stopPropagation()}
									/>
								</div>
							</DropdownMenuItem>
							<DropdownMenuSeparator />
							<DropdownMenuItem onClick={() => onDownload("json")}>
								<DownloadIcon className="size-3.5 mr-2" />
								Download as JSON
							</DropdownMenuItem>
							<DropdownMenuItem onClick={() => onDownload("txt")}>
								<DownloadIcon className="size-3.5 mr-2" />
								Download as TXT
							</DropdownMenuItem>
							<DropdownMenuSeparator />
							<DropdownMenuItem onClick={onShowShortcutHelp}>
								<HelpCircleIcon className="size-3.5 mr-2" />
								Keyboard shortcuts
							</DropdownMenuItem>
						</DropdownMenuContent>
					</DropdownMenu>
				</div>
			</div>

			{/* Conditional row: search match navigation + pinned navigation */}
			{((showMatchNav && searchMatches.length > 0) || showPinNav) && (
				<div className="flex items-center gap-3">
					{showMatchNav && (
						<div className="flex items-center gap-0.5">
							<span className="text-xs tabular-nums text-muted-foreground whitespace-nowrap px-0.5">
								{searchMatches.length > 0
									? `${currentMatchIndex + 1}/${searchMatches.length} matches`
									: "No matches"}
							</span>
							<Button
								variant="ghost"
								size="sm"
								onClick={onPreviousMatch}
								disabled={searchMatches.length === 0}
								aria-label="Previous match"
								className="h-7 w-7 p-0"
							>
								<ChevronLeftIcon className="size-3.5" />
							</Button>
							<Button
								variant="ghost"
								size="sm"
								onClick={onNextMatch}
								disabled={searchMatches.length === 0}
								aria-label="Next match"
								className="h-7 w-7 p-0"
							>
								<ChevronRightIcon className="size-3.5" />
							</Button>
						</div>
					)}
					{showPinNav && (
						<div className="flex items-center gap-0.5">
							<span className="text-xs tabular-nums text-muted-foreground whitespace-nowrap px-0.5">
								{`${Math.min(currentPinnedIndex + 1, sortedPinnedIndices.length)}/${sortedPinnedIndices.length} pinned`}
							</span>
							<Button
								variant="ghost"
								size="sm"
								onClick={() => onNavigatePins(-1)}
								aria-label="Previous pinned line"
								className="h-7 w-7 p-0"
							>
								<ChevronLeftIcon className="size-3.5" />
							</Button>
							<Button
								variant="ghost"
								size="sm"
								onClick={() => onNavigatePins(1)}
								aria-label="Next pinned line"
								className="h-7 w-7 p-0"
							>
								<ChevronRightIcon className="size-3.5" />
							</Button>
						</div>
					)}
				</div>
			)}
		</div>
	);
}
