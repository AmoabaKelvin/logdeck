import type React from "react";
import { ChevronDownIcon, ChevronRightIcon } from "@/components/ui/icons";
import { FIELD_KEY_REGEX } from "@/lib/field-filter";
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
	// Called with the dot path and value of a clicked primitive value.
	onFieldClick?: (key: string, value: string) => void;
}

// A nested key that joins a dot path unambiguously.
const NESTED_KEY_REGEX = /^[A-Za-z_@][A-Za-z0-9_@-]*$/;
const KEYED_LINE_REGEX = /^("(?:\\.|[^"\\])*"): (.*)$/;

// For each line of two-space pretty-printed JSON, the filter key of the
// primitive value on it, or null. Values inside arrays have none.
function fieldKeys(lines: string[]): (string | null)[] {
	// The key path of each open object; null where values are not addressable.
	const stack: (string[] | null)[] = [];
	return lines.map((line) => {
		const trimmed = line.trim().replace(/,$/, "");
		const keyed = KEYED_LINE_REGEX.exec(trimmed);
		const parent = stack.at(-1);
		let path: string[] | null = null;
		let value = trimmed;
		if (keyed) {
			value = keyed[2];
			// SAFETY: the key is a JSON string token written by JSON.stringify.
			const key = JSON.parse(keyed[1]) as string;
			const valid =
				parent?.length === 0
					? FIELD_KEY_REGEX.test(key)
					: NESTED_KEY_REGEX.test(key);
			if (parent && valid) path = [...parent, key];
		}
		if (value === "{") {
			stack.push(stack.length === 0 ? [] : path);
			return null;
		}
		if (value === "[") {
			stack.push(null);
			return null;
		}
		if (value === "}" || value === "]") {
			stack.pop();
			return null;
		}
		if (value === "{}" || value === "[]") return null;
		return path ? path.join(".") : null;
	});
}

function colorizeJson(
	jsonStr: string,
	onFieldClick?: (key: string, value: string) => void,
): React.ReactNode {
	const lines = jsonStr.split("\n");
	const keys = onFieldClick ? fieldKeys(lines) : [];
	return lines.map((line, lineIdx) => {
		const fieldKey = keys[lineIdx] ?? null;
		const parts: React.ReactNode[] = [];
		const remaining = line;
		let partIdx = 0;

		const tokenRegex =
			/("(?:\\.|[^"\\])*")\s*:|("(?:\\.|[^"\\])*")|([-+]?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)|(\btrue\b|\bfalse\b|\bnull\b)/g;
		let lastIndex = 0;

		for (const match of remaining.matchAll(tokenRegex)) {
			if (match.index > lastIndex) {
				parts.push(
					<span key={partIdx++}>
						{remaining.slice(lastIndex, match.index)}
					</span>,
				);
			}

			if (match[1]) {
				parts.push(
					<span key={partIdx++} className="text-blue-600 dark:text-blue-400">
						{match[1]}
					</span>,
				);
				parts.push(<span key={partIdx++}>:</span>);
			} else {
				const token = match[2] ?? match[3] ?? match[4];
				const className = match[2]
					? "text-green-600 dark:text-green-400"
					: match[3]
						? "text-orange-600 dark:text-orange-400"
						: "text-purple-600 dark:text-purple-400";
				if (fieldKey && onFieldClick) {
					// SAFETY: a string token is JSON written by JSON.stringify.
					const value = match[2] ? (JSON.parse(token) as string) : token;
					parts.push(
						<button
							key={partIdx++}
							type="button"
							title={`Filter by ${fieldKey}`}
							onClick={(event) => {
								event.stopPropagation();
								onFieldClick(fieldKey, value);
							}}
							onKeyDown={(event) => {
								event.stopPropagation();
							}}
							className={`${className} cursor-pointer rounded-sm hover:underline`}
						>
							{token}
						</button>,
					);
				} else {
					parts.push(
						<span key={partIdx++} className={className}>
							{token}
						</span>,
					);
				}
			}

			lastIndex = match.index + match[0].length;
		}

		if (lastIndex < remaining.length) {
			parts.push(<span key={partIdx++}>{remaining.slice(lastIndex)}</span>);
		}

		return (
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
	onFieldClick,
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
				{highlightSearchText ? highlightSearchText(text, isCurrentMatch) : text}
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
			<pre className="whitespace-pre font-mono leading-relaxed inline">
				{highlightSearchText
					? highlightSearchText(prettyJson, isCurrentMatch)
					: colorizeJson(prettyJson, onFieldClick)}
			</pre>
		</div>
	);
}
