import { ChevronDownIcon, ChevronRightIcon } from "lucide-react";
import type React from "react";
import { formatJson } from "@/lib/json-format";

interface CollapsibleJsonProps {
	text: string;
	isExpanded: boolean;
	onToggle: () => void;
	isCurrentMatch?: boolean;
	highlightSearchText?: (
		text: string,
		isCurrentMatch: boolean,
	) => React.ReactNode;
}

function colorizeJson(jsonStr: string): React.ReactNode {
	const lines = jsonStr.split("\n");
	return lines.map((line, lineIdx) => {
		const parts: React.ReactNode[] = [];
		const remaining = line;
		let partIdx = 0;

		const tokenRegex =
			/("(?:\\.|[^"\\])*")\s*:|("(?:\\.|[^"\\])*")|([-+]?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)|(\btrue\b|\bfalse\b|\bnull\b)/g;
		let lastIndex = 0;

		for (const match of remaining.matchAll(tokenRegex)) {
			if (match.index > lastIndex) {
				parts.push(
		<span key={partIdx++}>{remaining.slice(lastIndex, match.index)}</span>,
				);
			}

			if (match[1]) {
				parts.push(
		<span key={partIdx++} className="text-blue-600 dark:text-blue-400">
						{match[1]}
					</span>,
				);
parts.push(<span key={partIdx++}>:</span>);
			} else if (match[2]) {
				parts.push(
		<span key={partIdx++} className="text-green-600 dark:text-green-400">
						{match[2]}
					</span>,
				);
			} else if (match[3]) {
				parts.push(
		<span key={partIdx++} className="text-orange-600 dark:text-orange-400">
						{match[3]}
					</span>,
				);
			} else if (match[4]) {
				parts.push(
		<span key={partIdx++} className="text-purple-600 dark:text-purple-400">
						{match[4]}
					</span>,
				);
			}

			lastIndex = match.index + match[0].length;
		}

		if (lastIndex < remaining.length) {
			parts.push(<span key={partIdx++}>{remaining.slice(lastIndex)}</span>);
		}

		return (
			// biome-ignore lint/suspicious/noArrayIndexKey: lines from JSON.stringify have stable order
			<span key={lineIdx}>
				{parts}
				{lineIdx < lines.length - 1 ? "\n" : ""}
			</span>
		);
	});
}

export function CollapsibleJson({
	text,
	isExpanded,
	onToggle,
	isCurrentMatch = false,
	highlightSearchText,
}: CollapsibleJsonProps) {
	if (!isExpanded) {
		return (
			<span className="inline">
				<button
					type="button"
					onClick={(event) => {
						event.stopPropagation();
						onToggle();
					}}
					onKeyDown={(event) => {
						event.stopPropagation();
					}}
					className="inline-flex items-center justify-center shrink-0 p-0.5 rounded hover:bg-muted/50 transition-colors"
					aria-label="Expand JSON"
				>
					<ChevronRightIcon className="size-3" />
				</button>
				{highlightSearchText
					? highlightSearchText(text, isCurrentMatch)
					: text}
			</span>
		);
	}

	const { formatted: prettyJson } = formatJson(text);

	return (
		<div>
			<button
				type="button"
				onClick={(event) => {
					event.stopPropagation();
					onToggle();
				}}
				onKeyDown={(event) => {
					event.stopPropagation();
				}}
				className="inline-flex items-center justify-center shrink-0 p-0.5 rounded hover:bg-muted/50 transition-colors"
				aria-label="Collapse JSON"
			>
				<ChevronDownIcon className="size-3" />
			</button>
			<pre className="whitespace-pre font-mono text-xs leading-relaxed inline">
				{highlightSearchText
					? highlightSearchText(prettyJson, isCurrentMatch)
					: colorizeJson(prettyJson)}
			</pre>
		</div>
	);
}
