import type { Virtualizer } from "@tanstack/react-virtual";
import type React from "react";

import { CardContent } from "@/components/ui/card";
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
	filteredLogs: LogEntry[];
	filteredToOriginalIndex: number[];
	wrapText: boolean;
	showTimestamps: boolean;
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
	filteredLogs,
	filteredToOriginalIndex,
	wrapText,
	showTimestamps,
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
	let body: React.ReactNode;
	if (isLoadingLogs && totalCount === 0) {
		body = (
			<div className="flex items-center justify-center py-8 text-muted-foreground">
				<Spinner className="mr-2 size-4" />
				Loading logs...
			</div>
		);
	} else if (totalCount === 0) {
		body = (
			<div className="flex items-center justify-center py-8 text-muted-foreground text-sm">
				No logs available
			</div>
		);
	} else if (filteredLogs.length === 0) {
		body = (
			<div className="flex items-center justify-center py-8 text-muted-foreground text-sm">
				No logs match the current filters
			</div>
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
		<CardContent className="p-0 relative">
			{/* Selection Action Bar - sticky at top of logs */}
			<SelectionActionBar
				selectedCount={selectedIndices.size}
				onCopy={onCopySelected}
				onTogglePin={onTogglePinSelected}
				pinActionLabel={allSelectedArePinned ? "unpin" : "pin"}
				onClear={onClearSelection}
			/>
			<div
				ref={parentRef}
				className={
					variant === "page"
						? "h-[calc(100vh-400px)] min-h-[400px] w-full overflow-auto"
						: "h-[400px] w-full overflow-auto"
				}
			>
				{body}
			</div>
		</CardContent>
	);
}
