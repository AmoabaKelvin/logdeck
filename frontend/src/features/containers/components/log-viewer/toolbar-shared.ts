import type React from "react";

import type { LogLevel } from "@/features/containers/api/get-container-logs-parsed";
import type { SearchParsed } from "./use-log-search";
import type { LogViewState } from "./use-log-view-state";

// Toolbar controls match the dashboard's: bigger on mobile, and an active
// toggle reads as a filled chip rather than a second border treatment.
export const toolbarControlClass =
	"h-10 text-base sm:h-9 sm:text-sm data-[active=true]:bg-muted data-[active=true]:text-foreground";

export const toolbarIconButtonClass = "size-10 sm:size-9";

// Props shared by the page and sheet toolbar variants; LogViewer supplies
// them from its state and the stream hook.
export interface LogViewerToolbarProps {
	viewState: LogViewState;
	searchParsed: SearchParsed;
	searchInputRef: React.RefObject<HTMLInputElement | null>;
	excludeMatches: boolean;
	setExcludeMatches: (value: boolean) => void;
	autoScroll: boolean;
	setAutoScroll: (value: boolean) => void;
	availableLogLevels: readonly LogLevel[];
	searchMatches: number[];
	currentMatchIndex: number;
	onPreviousMatch: () => void;
	onNextMatch: () => void;
	sortedPinnedIndices: number[];
	currentPinnedIndex: number;
	onNavigatePins: (offset: 1 | -1) => void;
	isStreaming: boolean;
	isStreamPaused: boolean;
	isReconnecting: boolean;
	isLoadingLogs: boolean;
	bufferedCount: number;
	onToggleStreaming: () => void;
	onTogglePause: () => void;
	onRefresh: () => void;
	onLogLinesChange: (value: string) => void;
	onDownload: (format: "json" | "txt") => void;
	onShowShortcutHelp: () => void;
}
