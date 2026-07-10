import type React from "react";

import type { LogLevel } from "@/features/containers/api/get-container-logs-parsed";
import type { SearchParsed } from "./use-log-search";
import type { LogViewState } from "./use-log-view-state";

export const activeToggleButtonClass =
	"h-8 text-xs data-[active=true]:bg-muted data-[active=true]:text-foreground data-[active=true]:border-border data-[active=true]:ring-1 data-[active=true]:ring-primary/30 dark:data-[active=true]:bg-primary/15 dark:data-[active=true]:ring-primary/50 dark:data-[active=true]:border-primary/30";

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
	availableLogLevels: LogLevel[];
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
