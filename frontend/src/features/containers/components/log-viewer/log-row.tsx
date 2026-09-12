import type React from "react";
import { CopyIcon } from "@/components/ui/icons";
import {
	Tooltip,
	TooltipContent,
	TooltipTrigger,
} from "@/components/ui/tooltip";
import type { LogEntry } from "@/features/containers/api/get-container-logs-parsed";
import { getLogLevelTextColor } from "@/features/containers/api/get-container-logs-parsed";
import { CollapsibleJson } from "@/features/containers/components/collapsible-json";
import { isJsonString } from "@/lib/json-format";

// Deterministic per-container colour for aggregate views.
const CONTAINER_NAME_COLORS = [
	"text-sky-600 dark:text-sky-400",
	"text-violet-600 dark:text-violet-400",
	"text-emerald-600 dark:text-emerald-400",
	"text-orange-600 dark:text-orange-400",
	"text-pink-600 dark:text-pink-400",
	"text-cyan-600 dark:text-cyan-400",
	"text-lime-600 dark:text-lime-400",
	"text-fuchsia-600 dark:text-fuchsia-400",
];

function getContainerNameColor(name: string): string {
	let hash = 0;
	for (let i = 0; i < name.length; i++) {
		hash = (hash * 31 + name.charCodeAt(i)) | 0;
	}
	return CONTAINER_NAME_COLORS[Math.abs(hash) % CONTAINER_NAME_COLORS.length];
}

function formatRowTimestamp(timestamp: string | undefined): string {
	if (!timestamp) return "—";
	const date = new Date(timestamp);
	return `${date.toLocaleDateString("en-GB", {
		day: "2-digit",
		month: "2-digit",
		year: "numeric",
	})} ${date.toLocaleTimeString("en-US", {
		hour12: false,
		hour: "2-digit",
		minute: "2-digit",
		second: "2-digit",
	})}`;
}

function rowStateClass(props: LogRowProps): string {
	const { isSelected, isPinned, isCurrentMatch, index, wrapText } = props;
	if (isSelected && isPinned) {
		return "bg-amber-200/80 dark:bg-amber-800/45 border-l-amber-500";
	}
	if (isSelected) {
		return "bg-primary/[0.08] dark:bg-primary/[0.15] border-l-primary";
	}
	if (isPinned) {
		return "bg-amber-100/80 dark:bg-amber-900/35 border-l-amber-500 hover:bg-amber-100 dark:hover:bg-amber-900/45";
	}
	if (isCurrentMatch) {
		return "bg-yellow-100 dark:bg-yellow-900/30 border-l-yellow-400 dark:border-l-yellow-600";
	}
	// Wrapped lines run to different heights, so the eye needs a band to tell
	// one entry from the next. Single-line rows read fine on alignment alone.
	if (wrapText && index % 2 === 0) {
		return "bg-muted/30 hover:bg-muted/60";
	}
	return "hover:bg-muted/60";
}

export interface LogRowProps {
	entry: LogEntry;
	// Index of this row in the filtered list (what selection and search
	// bookkeeping use).
	index: number;
	// Vertical offset assigned by the virtualizer.
	start: number;
	measureRef: (node: Element | null) => void;
	wrapText: boolean;
	showTimestamps: boolean;
	// Only meaningful in the aggregate view: a single-container view has one
	// name, and stored entries carry it too.
	showContainerName: boolean;
	isSelected: boolean;
	isPinned: boolean;
	isCurrentMatch: boolean;
	hasMatch: boolean;
	isNewRow: boolean;
	isJsonExpanded: boolean;
	highlightSearchText: (
		text: string,
		isCurrentMatch: boolean,
	) => React.ReactNode;
	onClick: (
		index: number,
		event: React.MouseEvent | React.KeyboardEvent,
	) => void;
	onToggleJson: (index: number) => void;
	onCopy: (entry: LogEntry) => void;
}

export function LogRow(props: LogRowProps) {
	const {
		entry,
		index,
		start,
		measureRef,
		wrapText,
		showTimestamps,
		showContainerName,
		isCurrentMatch,
		hasMatch,
		isNewRow,
		isJsonExpanded,
		highlightSearchText,
		onClick,
		onToggleJson,
		onCopy,
	} = props;

	const displayText = entry.message || entry.raw || "";
	if (!displayText.trim()) return null;

	// An "UNKNOWN" on every unparsed line is noise; the column still holds its
	// width so messages stay in one vertical line.
	const level = entry.level && entry.level !== "UNKNOWN" ? entry.level : null;

	return (
		// biome-ignore lint/a11y/useSemanticElements: div required for virtual scrolling absolute positioning
		<div
			data-index={index}
			ref={measureRef}
			role="button"
			tabIndex={0}
			onClick={(e) => onClick(index, e)}
			onMouseDown={(e) => {
				// Suppress native text selection on shift-click so range
				// selection doesn't highlight everything in between;
				// normal click-drag selection stays intact.
				if (e.shiftKey) e.preventDefault();
			}}
			onKeyDown={(e) => {
				if (e.key === "Enter" || e.key === " ") {
					e.preventDefault();
					onClick(index, e);
				}
			}}
			style={{
				position: "absolute",
				top: 0,
				left: 0,
				width: wrapText ? "100%" : "max-content",
				minWidth: "100%",
				transform: `translateY(${start}px)`,
				cursor: "pointer",
			}}
			className={`group flex items-start gap-x-3 border-l-2 border-transparent py-1 pr-4 pl-2 ${
				wrapText ? "" : "whitespace-nowrap"
			} ${rowStateClass(props)} ${isNewRow ? "log-stream-row-enter" : ""}`}
		>
			{showTimestamps && (
				<span className="shrink-0 text-muted-foreground tabular-nums">
					{formatRowTimestamp(entry.timestamp)}
				</span>
			)}

			<span
				className={`w-12 shrink-0 ${level ? getLogLevelTextColor(entry.level) : ""}`}
			>
				{level}
			</span>

			{showContainerName && entry.containerName && (
				<span
					title={entry.containerName}
					className={`w-40 shrink-0 truncate ${getContainerNameColor(entry.containerName)}`}
				>
					{entry.containerName}
				</span>
			)}

			<span
				className={`min-w-0 flex-1 text-foreground ${wrapText ? "break-words whitespace-pre-wrap" : "whitespace-pre"}`}
			>
				{isJsonString(displayText) ? (
					<CollapsibleJson
						text={displayText}
						isExpanded={isJsonExpanded}
						onToggle={() => onToggleJson(index)}
						isCurrentMatch={isCurrentMatch}
						highlightSearchText={hasMatch ? highlightSearchText : undefined}
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
							onCopy(entry);
						}}
						aria-label="Copy log entry"
						className="shrink-0 rounded p-1 opacity-0 group-hover:opacity-100 hover:bg-muted focus-visible:opacity-100"
					>
						<CopyIcon className="size-3 text-muted-foreground" />
					</button>
				</TooltipTrigger>
				<TooltipContent>Copy log entry</TooltipContent>
			</Tooltip>
		</div>
	);
}
