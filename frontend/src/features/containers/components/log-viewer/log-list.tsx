import type { Virtualizer } from "@tanstack/react-virtual";
import type React from "react";

import { Spinner } from "@/components/ui/spinner";
import type { LogEntry } from "@/features/containers/api/get-container-logs-parsed";
import { SelectionActionBar } from "@/features/containers/components/selection-action-bar";
import type { IndexRange } from "./animated-range";
import { LogRow } from "./log-row";

interface LogListProps {
	variant: "page" | "sheet";
	parentRef: React.RefObject<HTMLDivElement | null>;
	rowVirtualizer: Virtualizer<HTMLDivElement, Element>;
	isLoadingLogs: boolean;
	// Grouped rows before filtering; distinguishes "no logs" from "all
	// filtered out".
	totalCount: number;
	// Shown when nothing was returned at all. Defaults to the live wording.
	emptyMessage?: string;
	// Rendered above the scroll area (history mode uses it for "Load older").
	topSlot?: React.ReactNode;
	filteredLogs: LogEntry[];
	filteredToOriginalIndex: number[];
	wrapText: boolean;
	showTimestamps: boolean;
	// Per-row container badge; only the aggregate view mixes containers.
	showContainerName: boolean;
	searchMatches: number[];
	searchMatchSet: Set<number>;
	currentMatchIndex: number;
	selectedIndices: Set<number>;
	pinnedFilteredIndices: Set<number>;
	// Rows to play the "new line" animation for, in grouped index space.
	animatedGroupedRange: IndexRange | null;
	expandedJsonRows: Set<number>;
	highlightSearchText: (
		text: string,
		isCurrentMatch: boolean,
	) => React.ReactNode;
	onLogClick: (
		index: number,
		event: React.MouseEvent | React.KeyboardEvent,
	) => void;
	onToggleJson: (index: number) => void;
	onCopyEntry: (entry: LogEntry) => void;
	allSelectedArePinned: boolean;
	onCopySelected: () => void;
	onTogglePinSelected: () => void;
	onClearSelection: () => void;
}

export function LogList({
	variant,
	parentRef,
	rowVirtualizer,
	isLoadingLogs,
	totalCount,
	emptyMessage = "No logs available",
	topSlot,
	filteredLogs,
	filteredToOriginalIndex,
	wrapText,
	showTimestamps,
	showContainerName,
	searchMatches,
	searchMatchSet,
	currentMatchIndex,
	selectedIndices,
	pinnedFilteredIndices,
	animatedGroupedRange,
	expandedJsonRows,
	highlightSearchText,
	onLogClick,
	onToggleJson,
	onCopyEntry,
	allSelectedArePinned,
	onCopySelected,
	onTogglePinSelected,
	onClearSelection,
}: LogListProps) {
	// Empty and loading states centre in the whole scroll area, which on the log
	// page is most of the viewport; pinned to the top they read as a broken load.
	const messageClass =
		"flex h-full min-h-40 items-center justify-center gap-2 px-4 text-center text-base text-muted-foreground sm:text-sm";

	let body: React.ReactNode;
	if (isLoadingLogs && totalCount === 0) {
		body = (
			<div className={messageClass}>
				<Spinner className="size-4" />
				Loading logs…
			</div>
		);
	} else if (totalCount === 0) {
		body = <div className={messageClass}>{emptyMessage}</div>;
	} else if (filteredLogs.length === 0) {
		body = (
			<div className={messageClass}>No logs match the current filters.</div>
		);
	} else {
		body = (
			<div
				style={{
					height: `${rowVirtualizer.getTotalSize()}px`,
					width: "100%",
					position: "relative",
				}}
				className={`font-mono text-xs ${wrapText ? "" : "w-fit min-w-full"}`}
			>
				{rowVirtualizer.getVirtualItems().map((virtualRow) => {
					const isCurrentMatch =
						searchMatches.length > 0 &&
						searchMatches[currentMatchIndex] === virtualRow.index;
					// Animation range is in grouped index space; compare against
					// this row's grouped index, not its filtered position.
					const groupedIndex = filteredToOriginalIndex[virtualRow.index];
					const isNewRow =
						animatedGroupedRange !== null &&
						groupedIndex >= animatedGroupedRange.start &&
						groupedIndex <= animatedGroupedRange.end;

					return (
						<LogRow
							key={virtualRow.key}
							entry={filteredLogs[virtualRow.index]}
							index={virtualRow.index}
							start={virtualRow.start}
							measureRef={rowVirtualizer.measureElement}
							wrapText={wrapText}
							showTimestamps={showTimestamps}
							showContainerName={showContainerName}
							isSelected={selectedIndices.has(virtualRow.index)}
							isPinned={pinnedFilteredIndices.has(virtualRow.index)}
							isCurrentMatch={isCurrentMatch}
							hasMatch={searchMatchSet.has(virtualRow.index)}
							isNewRow={isNewRow}
							isJsonExpanded={expandedJsonRows.has(virtualRow.index)}
							highlightSearchText={highlightSearchText}
							onClick={onLogClick}
							onToggleJson={onToggleJson}
							onCopy={onCopyEntry}
						/>
					);
				})}
			</div>
		);
	}

	return (
		// The page variant fills whatever height the route's flex column leaves
		// it, down to a floor that keeps the list usable when a detail panel is
		// open; the sheet gets a fixed window.
		<div
			className={
				variant === "page"
					? "relative flex flex-col lg:min-h-0 lg:flex-1"
					: "relative"
			}
		>
			<SelectionActionBar
				selectedCount={selectedIndices.size}
				onCopy={onCopySelected}
				onTogglePin={onTogglePinSelected}
				pinActionLabel={allSelectedArePinned ? "unpin" : "pin"}
				onClear={onClearSelection}
			/>
			{topSlot}
			<div
				ref={parentRef}
				className={
					variant === "page"
						? "h-[60dvh] w-full overflow-auto lg:h-auto lg:min-h-[26rem] lg:flex-1"
						: "h-[400px] w-full overflow-auto"
				}
			>
				{body}
			</div>
		</div>
	);
}
