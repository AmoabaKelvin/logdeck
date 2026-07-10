import { CopyIcon } from "lucide-react";
import type React from "react";

import { Badge } from "@/components/ui/badge";
import {
	Tooltip,
	TooltipContent,
	TooltipTrigger,
} from "@/components/ui/tooltip";
import type { LogEntry } from "@/features/containers/api/get-container-logs-parsed";
import { getLogLevelBadgeColor } from "@/features/containers/api/get-container-logs-parsed";
import { CollapsibleJson } from "@/features/containers/components/collapsible-json";
import { isJsonString } from "@/lib/json-format";

// Deterministic per-container badge color for aggregate views.
const CONTAINER_NAME_BADGE_COLORS = [
	"bg-sky-100 text-sky-700 dark:bg-sky-900 dark:text-sky-300",
	"bg-violet-100 text-violet-700 dark:bg-violet-900 dark:text-violet-300",
	"bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300",
	"bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300",
	"bg-pink-100 text-pink-700 dark:bg-pink-900 dark:text-pink-300",
	"bg-cyan-100 text-cyan-700 dark:bg-cyan-900 dark:text-cyan-300",
	"bg-lime-100 text-lime-700 dark:bg-lime-900 dark:text-lime-300",
	"bg-fuchsia-100 text-fuchsia-700 dark:bg-fuchsia-900 dark:text-fuchsia-300",
];

function getContainerNameBadgeColor(name: string): string {
	let hash = 0;
	for (let i = 0; i < name.length; i++) {
		hash = (hash * 31 + name.charCodeAt(i)) | 0;
	}
	return CONTAINER_NAME_BADGE_COLORS[
		Math.abs(hash) % CONTAINER_NAME_BADGE_COLORS.length
	];
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
	const { isSelected, isPinned, isCurrentMatch, index } = props;
	if (isSelected && isPinned) {
		return "bg-amber-200/80 dark:bg-amber-800/45 border-l-[3px] border-amber-500 shadow-[inset_0_0_0_1px_rgba(245,158,11,0.35)]";
	}
	if (isSelected) {
		return "bg-primary/[0.08] dark:bg-primary/[0.15] border-l-[3px] border-primary shadow-[inset_0_0_0_1px_rgba(var(--primary),0.1)]";
	}
	if (isPinned) {
		return "bg-amber-100/80 dark:bg-amber-900/35 border-l-[3px] border-amber-500 hover:bg-amber-100 dark:hover:bg-amber-900/45";
	}
	if (isCurrentMatch) {
		return "bg-yellow-100 dark:bg-yellow-900/30 border-y-2 border-yellow-400 dark:border-yellow-600";
	}
	if (index % 2 === 0) {
		return "bg-muted/30 border-l-[3px] border-transparent hover:bg-muted/50";
	}
	return "border-l-[3px] border-transparent hover:bg-muted/50";
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
			className={`group flex items-start gap-3 px-4 py-1.5 transition-all duration-150 ease-out ${
				wrapText ? "" : "whitespace-nowrap"
			} ${rowStateClass(props)} ${isNewRow ? "log-stream-row-enter" : ""}`}
		>
			{showTimestamps && (
				<span className="text-muted-foreground shrink-0 text-[11px]">
					{formatRowTimestamp(entry.timestamp)}
				</span>
			)}
			<Badge
				variant="outline"
				className={`shrink-0 text-xs px-1.5 py-0 h-5 ${getLogLevelBadgeColor(entry.level ?? "UNKNOWN")}`}
			>
				{entry.level ?? "UNKNOWN"}
			</Badge>
			{entry.containerName && (
				<Badge
					variant="outline"
					className={`shrink-0 text-xs px-1.5 py-0 h-5 ${getContainerNameBadgeColor(entry.containerName)}`}
				>
					{entry.containerName}
				</Badge>
			)}
			<span
				className={`text-foreground flex-1 ${wrapText ? "whitespace-pre-wrap break-words" : "whitespace-pre"}`}
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
						className="shrink-0 opacity-0 group-hover:opacity-100 transition-opacity p-1 hover:bg-muted rounded"
					>
						<CopyIcon className="size-3 text-muted-foreground" />
					</button>
				</TooltipTrigger>
				<TooltipContent>Copy log entry</TooltipContent>
			</Tooltip>
		</div>
	);
}
