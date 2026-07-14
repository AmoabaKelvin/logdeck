import { useInfiniteQuery } from "@tanstack/react-query";
import { useMemo } from "react";

import type { LogEntry, LogLevel } from "../api/get-container-logs-parsed";
import { getHistoryLogs, type HistoryLogsPage } from "../api/get-history";

export const HISTORY_PAGE_SIZE = 500;

// Pages arrive newest-first (page 0 is the newest window, each further page is
// older), while entries inside a page are ascending. Rendering wants one
// ascending list, so older pages go in front.
export function flattenHistoryPages(pages: HistoryLogsPage[]): LogEntry[] {
	const ordered: LogEntry[] = [];
	for (let i = pages.length - 1; i >= 0; i--) {
		ordered.push(...pages[i].logs);
	}
	return ordered;
}

interface UseHistoryLogsOptions {
	enabled: boolean;
	container: string;
	host?: string;
	since?: string;
	until?: string;
	// Sent to the server: history is filtered in the store, not in the browser.
	levels: Set<LogLevel>;
	search: string;
	regex: boolean;
}

export function useHistoryLogs({
	enabled,
	container,
	host,
	since,
	until,
	levels,
	search,
	regex,
}: UseHistoryLogsOptions) {
	const levelList = useMemo(() => Array.from(levels).sort(), [levels]);

	const query = useInfiniteQuery({
		queryKey: [
			"history",
			"logs",
			container,
			host ?? "",
			since ?? "",
			until ?? "",
			levelList.join(","),
			search,
			regex,
		],
		queryFn: ({ pageParam }) =>
			getHistoryLogs({
				container,
				host,
				since,
				until,
				levels: levelList,
				search: search || undefined,
				regex,
				limit: HISTORY_PAGE_SIZE,
				cursor: pageParam || undefined,
			}),
		initialPageParam: "",
		getNextPageParam: (lastPage) => lastPage.nextCursor || undefined,
		enabled: enabled && container.length > 0,
		retry: false,
		staleTime: 30_000,
	});

	const logs = useMemo(
		() => flattenHistoryPages(query.data?.pages ?? []),
		[query.data],
	);

	return {
		logs,
		error: query.error,
		isLoading: query.isLoading,
		isFetchingOlder: query.isFetchingNextPage,
		hasOlder: query.hasNextPage,
		fetchOlder: query.fetchNextPage,
		refetch: query.refetch,
	};
}
