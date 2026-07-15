import { useCallback, useEffect, useMemo, useState } from "react";

import type { LogEntry } from "@/features/containers/api/get-container-logs-parsed";
import { escapeRegExp } from "@/lib/utils";

export interface SearchParsed {
	regex: RegExp | null;
	error: string | null;
}

export const EMPTY_SEARCH: SearchParsed = { regex: null, error: null };

export function parseSearch(
	searchText: string,
	useRegex: boolean,
): SearchParsed {
	if (!searchText || !useRegex) return EMPTY_SEARCH;
	try {
		return { regex: new RegExp(searchText, "gi"), error: null };
	} catch {
		return { regex: null, error: "Invalid regex" };
	}
}

export function computeSearchMatches(
	entries: LogEntry[],
	searchText: string,
	useRegex: boolean,
	searchParsed: SearchParsed,
): number[] {
	if (!searchText) return [];
	if (useRegex && searchParsed.error) return [];
	const searchLower = searchText.toLowerCase();
	const matches: number[] = [];
	entries.forEach((entry, index) => {
		const message = entry.message || entry.raw || "";
		if (useRegex && searchParsed.regex) {
			searchParsed.regex.lastIndex = 0;
			if (searchParsed.regex.test(message)) {
				matches.push(index);
			}
		} else {
			if (message.toLowerCase().includes(searchLower)) {
				matches.push(index);
			}
		}
	});
	return matches;
}

export function useLogSearch(searchText: string, useRegex: boolean) {
	const searchParsed = useMemo(
		() => parseSearch(searchText, useRegex),
		[searchText, useRegex],
	);

	// Pre-compiled regex for plain-text highlighting (avoids per-row compilation)
	const plainTextSplitRegex = useMemo(() => {
		if (!searchText || useRegex) return null;
		return new RegExp(`(${escapeRegExp(searchText)})`, "gi");
	}, [searchText, useRegex]);

	const highlightSearchText = useCallback(
		(text: string, isCurrentMatch: boolean): React.ReactNode => {
			if (!searchText || !text) return text;

			if (useRegex && searchParsed.regex) {
				// Use matchAll for safe regex highlighting (handles zero-length matches)
				searchParsed.regex.lastIndex = 0;
				const parts: React.ReactNode[] = [];
				let lastIndex = 0;
				let key = 0;
				for (const match of text.matchAll(searchParsed.regex)) {
					if (match.index === undefined) continue;
					// Zero-length matches would render empty <mark> elements.
					if (match[0].length === 0) continue;
					if (match.index > lastIndex) {
						parts.push(
							<span key={key++}>{text.slice(lastIndex, match.index)}</span>,
						);
					}
					parts.push(
						<mark
							key={key++}
							className={`px-0.5 rounded ${isCurrentMatch ? "bg-yellow-400 dark:bg-yellow-500" : "bg-yellow-200 dark:bg-yellow-700"}`}
						>
							{match[0]}
						</mark>,
					);
					lastIndex = match.index + match[0].length;
				}
				if (lastIndex < text.length) {
					parts.push(<span key={key++}>{text.slice(lastIndex)}</span>);
				}
				return parts.length > 0 ? parts : text;
			}

			if (!plainTextSplitRegex) return text;
			const splitParts = text.split(plainTextSplitRegex);
			return splitParts.map((part, i) =>
				part.toLowerCase() === searchText.toLowerCase() ? (
					<mark
						key={`${i}-${part}`}
						className={`px-0.5 rounded ${isCurrentMatch ? "bg-yellow-400 dark:bg-yellow-500" : "bg-yellow-200 dark:bg-yellow-700"}`}
					>
						{part}
					</mark>
				) : (
					<span key={`${i}-${part}`}>{part}</span>
				),
			);
		},
		[searchText, useRegex, searchParsed, plainTextSplitRegex],
	);

	return { searchParsed, highlightSearchText };
}

export function useSearchMatches({
	filteredLogs,
	searchText,
	useRegex,
	searchParsed,
}: {
	filteredLogs: LogEntry[];
	searchText: string;
	useRegex: boolean;
	searchParsed: SearchParsed;
}) {
	const [currentMatchIndex, setCurrentMatchIndex] = useState(0);

	const searchMatches = useMemo(
		() =>
			computeSearchMatches(filteredLogs, searchText, useRegex, searchParsed),
		[filteredLogs, searchText, useRegex, searchParsed],
	);

	// Set view of searchMatches for O(1) per-row lookups during rendering
	const searchMatchSet = useMemo(() => new Set(searchMatches), [searchMatches]);

	// biome-ignore lint/correctness/useExhaustiveDependencies: intentionally reset when searchText or useRegex changes
	useEffect(() => {
		setCurrentMatchIndex(0);
	}, [searchText, useRegex]);

	return {
		searchMatches,
		searchMatchSet,
		currentMatchIndex,
		setCurrentMatchIndex,
	};
}
