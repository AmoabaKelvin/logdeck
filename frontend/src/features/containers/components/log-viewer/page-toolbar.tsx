import { Button } from "@/components/ui/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuLabel,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
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
} from "@/components/ui/icons";
import { Input } from "@/components/ui/input";
import {
	Tooltip,
	TooltipContent,
	TooltipTrigger,
} from "@/components/ui/tooltip";
import { LevelFilterPopover } from "./level-filter-popover";
import { TimeRangeControl } from "./time-range-control";
import {
	type LogViewerToolbarProps,
	toolbarControlClass,
	toolbarIconButtonClass,
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

const segmentButtonClass =
	"h-8 rounded-sm px-2.5 text-sm text-muted-foreground shadow-none hover:bg-transparent data-[active=true]:bg-background data-[active=true]:text-foreground data-[active=true]:shadow-xs dark:data-[active=true]:bg-input/60";

/** "3 of 12" plus a pair of steppers — used for both matches and pins. */
function StepNav({
	label,
	disabled,
	onPrevious,
	onNext,
	previousLabel,
	nextLabel,
}: {
	label: string;
	disabled: boolean;
	onPrevious: () => void;
	onNext: () => void;
	previousLabel: string;
	nextLabel: string;
}) {
	return (
		<div className="flex shrink-0 items-center gap-0.5">
			<span className="px-1 text-sm whitespace-nowrap text-muted-foreground tabular-nums">
				{label}
			</span>
			<Button
				variant="ghost"
				size="icon-sm"
				onClick={onPrevious}
				disabled={disabled}
				aria-label={previousLabel}
			>
				<ChevronLeftIcon className="size-4" />
			</Button>
			<Button
				variant="ghost"
				size="icon-sm"
				onClick={onNext}
				disabled={disabled}
				aria-label={nextLabel}
			>
				<ChevronRightIcon className="size-4" />
			</Button>
		</div>
	);
}

/**
 * One wrapping row of controls. Reading settings that are set once and left
 * alone (timestamps, wrapping, tail length, export) live in the overflow menu;
 * only the controls a reader reaches for mid-session stay on the surface.
 */
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

	// History searches server-side: the non-matching lines were never sent, so
	// there is nothing to step through.
	const showMatchNav = !isHistory && Boolean(searchText) && !excludeMatches;

	return (
		<div className="flex flex-wrap items-center gap-2">
			{showSourceToggle && (
				<div className="flex shrink-0 items-center gap-0.5 rounded-md bg-muted p-0.5">
					{(["live", "history"] as LogSource[]).map((value) => (
						<Button
							key={value}
							variant="ghost"
							data-active={source === value}
							onClick={() => setSource(value)}
							aria-pressed={source === value}
							className={segmentButtonClass}
						>
							{value === "live" ? "Live" : "History"}
						</Button>
					))}
				</div>
			)}

			<div className="relative min-w-48 flex-1 sm:max-w-sm">
				<SearchIcon className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
				<Input
					ref={searchInputRef}
					type="search"
					name="log-search"
					aria-label="Search logs"
					placeholder={useRegex ? "Search by regex…" : "Search logs…"}
					value={searchText}
					onChange={(e) => setSearchText(e.target.value)}
					className={`h-10 pr-11 pl-8 sm:h-9 ${
						useRegex && searchParsed.error
							? "border-destructive focus-visible:ring-destructive/30"
							: ""
					}`}
				/>
				<button
					type="button"
					onClick={() => setUseRegex(!useRegex)}
					aria-label={
						useRegex ? "Switch to plain text search" : "Switch to regex search"
					}
					aria-pressed={useRegex}
					className={`absolute top-1/2 right-1.5 -translate-y-1/2 rounded px-1.5 py-1 font-mono text-xs leading-none ${
						useRegex
							? "bg-primary text-primary-foreground"
							: "text-muted-foreground hover:bg-muted hover:text-foreground"
					}`}
				>
					.*
				</button>
			</div>

			{showMatchNav && (
				<StepNav
					label={
						searchMatches.length > 0
							? `${currentMatchIndex + 1} of ${searchMatches.length}`
							: "No matches"
					}
					disabled={searchMatches.length === 0}
					onPrevious={onPreviousMatch}
					onNext={onNextMatch}
					previousLabel="Previous match"
					nextLabel="Next match"
				/>
			)}

			{sortedPinnedIndices.length > 0 && (
				<StepNav
					label={`${Math.min(currentPinnedIndex + 1, sortedPinnedIndices.length)} of ${sortedPinnedIndices.length} pinned`}
					disabled={false}
					onPrevious={() => onNavigatePins(-1)}
					onNext={() => onNavigatePins(1)}
					previousLabel="Previous pinned line"
					nextLabel="Next pinned line"
				/>
			)}

			<div className="ml-auto flex flex-wrap items-center gap-2">
				{isReconnecting && (
					<span className="animate-pulse text-sm text-muted-foreground">
						Reconnecting…
					</span>
				)}

				{filteredCount !== totalCount && (
					<span className="text-sm whitespace-nowrap text-muted-foreground tabular-nums">
						{filteredCount} of {totalCount}
					</span>
				)}

				<LevelFilterPopover
					selectedLevels={selectedLevels}
					setSelectedLevels={setSelectedLevels}
					availableLogLevels={availableLogLevels}
					className={toolbarControlClass}
				/>

				<TimeRangeControl
					timeRange={timeRange}
					setTimeRange={setTimeRange}
					disabled={isStreaming}
					className={toolbarControlClass}
				/>

				{!isHistory && (
					<>
						<Button
							variant="outline"
							data-active={isStreaming}
							onClick={onToggleStreaming}
							disabled={isLoadingLogs && !isStreaming}
							aria-pressed={isStreaming}
							className={toolbarControlClass}
						>
							{isStreaming ? (
								<SquareIcon className="size-4" />
							) : (
								<PlayIcon className="size-4" />
							)}
							{isStreaming ? "Stop" : "Stream"}
						</Button>

						{isStreaming && (
							<Button
								variant="outline"
								data-active={isStreamPaused}
								onClick={onTogglePause}
								aria-pressed={isStreamPaused}
								className={toolbarControlClass}
							>
								{isStreamPaused ? (
									<PlayIcon className="size-4" />
								) : (
									<PauseIcon className="size-4" />
								)}
								{isStreamPaused ? "Resume" : "Pause"}
								{isStreamPaused && bufferedCount > 0 && (
									<span className="text-muted-foreground tabular-nums">
										{bufferedCount}
									</span>
								)}
							</Button>
						)}

						<Tooltip>
							<TooltipTrigger asChild>
								<Button
									variant="outline"
									size="icon-sm"
									data-active={autoScroll}
									onClick={() => setAutoScroll(!autoScroll)}
									aria-label={`Auto-scroll ${autoScroll ? "on" : "off"}`}
									aria-pressed={autoScroll}
									className={`${toolbarIconButtonClass} ${toolbarControlClass}`}
								>
									{autoScroll ? (
										<ArrowDownToLineIcon className="size-4" />
									) : (
										<ArrowDownIcon className="size-4" />
									)}
								</Button>
							</TooltipTrigger>
							<TooltipContent>
								Auto-scroll {autoScroll ? "on" : "off"}
							</TooltipContent>
						</Tooltip>
					</>
				)}

				<Tooltip>
					<TooltipTrigger asChild>
						<Button
							variant="outline"
							size="icon-sm"
							onClick={onRefresh}
							disabled={isStreaming || isLoadingLogs}
							aria-label="Refresh logs"
							className={toolbarIconButtonClass}
						>
							<RefreshCcwIcon
								className={`size-4 ${isLoadingLogs ? "animate-spin" : ""}`}
							/>
						</Button>
					</TooltipTrigger>
					<TooltipContent>Refresh logs</TooltipContent>
				</Tooltip>

				<DropdownMenu>
					<DropdownMenuTrigger asChild>
						<Button
							variant="outline"
							size="icon-sm"
							aria-label="Log view options"
							className={toolbarIconButtonClass}
						>
							<EllipsisVerticalIcon className="size-4" />
						</Button>
					</DropdownMenuTrigger>
					<DropdownMenuContent align="end" className="w-56">
						<DropdownMenuLabel>View</DropdownMenuLabel>
						<DropdownMenuItem
							onClick={() => setShowTimestamps(!showTimestamps)}
						>
							<span className="flex-1">Timestamps</span>
							{showTimestamps && <CheckIcon className="size-4" />}
						</DropdownMenuItem>
						<DropdownMenuItem onClick={() => setWrapText(!wrapText)}>
							<span className="flex-1">Wrap long lines</span>
							{wrapText && <CheckIcon className="size-4" />}
						</DropdownMenuItem>
						{!isHistory && searchText && (
							<DropdownMenuItem
								onClick={() => setExcludeMatches(!excludeMatches)}
							>
								<span className="flex-1">Hide matching lines</span>
								{excludeMatches && <CheckIcon className="size-4" />}
							</DropdownMenuItem>
						)}
						{!isHistory && (
							<DropdownMenuItem
								onSelect={(event) => event.preventDefault()}
								className="justify-between gap-3"
							>
								<span>Tail</span>
								<Input
									name="log-lines"
									type="text"
									inputMode="numeric"
									pattern="[0-9]*"
									aria-label="Number of log lines to load"
									value={logLines}
									onChange={(e) => onLogLinesChange(e.target.value)}
									disabled={isStreaming}
									className="h-7 w-20 text-center tabular-nums"
								/>
							</DropdownMenuItem>
						)}
						<DropdownMenuSeparator />
						<DropdownMenuItem onClick={() => onDownload("json")}>
							<DownloadIcon className="size-4" />
							Download as JSON
						</DropdownMenuItem>
						<DropdownMenuItem onClick={() => onDownload("txt")}>
							<DownloadIcon className="size-4" />
							Download as TXT
						</DropdownMenuItem>
						<DropdownMenuSeparator />
						<DropdownMenuItem onClick={onShowShortcutHelp}>
							<HelpCircleIcon className="size-4" />
							Keyboard shortcuts
						</DropdownMenuItem>
					</DropdownMenuContent>
				</DropdownMenu>
			</div>
		</div>
	);
}
