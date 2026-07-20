import {
	ArrowDownIcon,
	ArrowDownToLineIcon,
	ChevronLeftIcon,
	ChevronRightIcon,
	DownloadIcon,
	PauseIcon,
	PlayIcon,
	RefreshCcwIcon,
	SearchIcon,
	SquareIcon,
} from "lucide-react";
import { useId } from "react";

import { Button } from "@/components/logdeck-demo/ui/button";
import { CardTitle } from "@/components/logdeck-demo/ui/card";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuTrigger,
} from "@/components/logdeck-demo/ui/dropdown-menu";
import { Input } from "@/components/logdeck-demo/ui/input";
import { Label } from "@/components/logdeck-demo/ui/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/logdeck-demo/ui/select";
import { LevelFilterPopover } from "./level-filter-popover";
import { TimeRangeControl } from "./time-range-control";
import {
	activeToggleButtonClass,
	type LogViewerToolbarProps,
} from "./toolbar-shared";
import type { LogSource } from "./use-log-view-state";

interface PageToolbarProps extends LogViewerToolbarProps {
	totalCount: number;
	filteredCount: number;
	// Reading from the log store instead of the live container: streaming and
	// tail controls make no sense there and are hidden.
	isHistory: boolean;
	// The toggle only appears when the server persists logs and the view has a
	// live counterpart to switch back to.
	showSourceToggle: boolean;
}

const sourceToggleButtonClass =
	"h-7 rounded-sm px-2.5 text-xs shadow-none data-[active=true]:bg-muted data-[active=true]:text-foreground dark:data-[active=true]:bg-primary/15";

// Full-width toolbar with labelled controls, used on the log routes.
export function PageToolbar({
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
	totalCount,
	filteredCount,
	isHistory,
	showSourceToggle,
}: PageToolbarProps) {
	const {
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
		timeRange,
		setTimeRange,
	} = viewState;
	const logLinesInputId = useId();

	return (
		<div className="space-y-3">
			<div className="flex items-center gap-2">
				<CardTitle className="text-base shrink-0">
					Logs
					{filteredCount !== totalCount && (
						<span className="ml-2 text-xs text-muted-foreground font-normal">
							({filteredCount} of {totalCount})
						</span>
					)}
				</CardTitle>

				{showSourceToggle && (
					<div className="flex shrink-0 items-center gap-0.5 rounded-md border p-0.5">
						{(["live", "history"] as LogSource[]).map((value) => (
							<Button
								key={value}
								variant="ghost"
								size="sm"
								data-active={source === value}
								onClick={() => setSource(value)}
								aria-pressed={source === value}
								className={sourceToggleButtonClass}
							>
								{value === "live" ? "Live" : "History"}
							</Button>
						))}
					</div>
				)}

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
				{/* History searches server-side: the non-matching lines were never
				    sent, so there is nothing to step through. */}
				{!isHistory && searchText && !excludeMatches && (
					<div className="flex items-center gap-0.5 shrink-0">
						<span className="text-xs tabular-nums text-muted-foreground whitespace-nowrap px-1">
							{searchMatches.length > 0
								? `${currentMatchIndex + 1} of ${searchMatches.length}`
								: "No matches"}
						</span>
						<Button
							variant="outline"
							size="sm"
							onClick={onPreviousMatch}
							disabled={searchMatches.length === 0}
							aria-label="Previous match"
							className="h-8 w-8 p-0"
						>
							<ChevronLeftIcon className="size-3.5" />
						</Button>
						<Button
							variant="outline"
							size="sm"
							onClick={onNextMatch}
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
							onClick={() => onNavigatePins(-1)}
							aria-label="Previous pinned line"
							className="h-8 w-8 p-0"
						>
							<ChevronLeftIcon className="size-3.5" />
						</Button>
						<Button
							variant="outline"
							size="sm"
							onClick={() => onNavigatePins(1)}
							aria-label="Next pinned line"
							className="h-8 w-8 p-0"
						>
							<ChevronRightIcon className="size-3.5" />
						</Button>
					</div>
				)}

				{!isHistory && (
					<>
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
								onChange={(e) => onLogLinesChange(e.target.value)}
								disabled={isStreaming}
								className="h-8 w-20 text-xs"
							/>
						</div>
						{isReconnecting && (
							<span className="shrink-0 text-xs text-muted-foreground animate-pulse">
								Reconnecting…
							</span>
						)}
						<Button
							variant="outline"
							size="sm"
							data-active={isStreaming}
							onClick={onToggleStreaming}
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
							onClick={onTogglePause}
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
					</>
				)}
				<Button
					variant="outline"
					size="sm"
					onClick={onRefresh}
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
				pins,{" "}
				<button
					type="button"
					onClick={onShowShortcutHelp}
					className="underline underline-offset-2 hover:text-foreground"
				>
					<kbd className="font-mono">?</kbd> help
				</button>
			</p>
			<div className="flex flex-wrap items-center gap-2">
				{!isHistory && searchText && (
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

				{!isHistory && (
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
				)}

				<div className="flex-1" />

				<DropdownMenu>
					<DropdownMenuTrigger asChild>
						<Button variant="outline" size="sm" className="h-8 text-xs">
							<DownloadIcon className="mr-1.5 size-3.5" />
							Download
						</Button>
					</DropdownMenuTrigger>
					<DropdownMenuContent align="end">
						<DropdownMenuItem onClick={() => onDownload("json")}>
							Download as JSON
						</DropdownMenuItem>
						<DropdownMenuItem onClick={() => onDownload("txt")}>
							Download as TXT
						</DropdownMenuItem>
					</DropdownMenuContent>
				</DropdownMenu>
			</div>
		</div>
	);
}
