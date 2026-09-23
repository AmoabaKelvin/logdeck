import { useInfiniteQuery } from "@tanstack/react-query";
import { useMemo } from "react";

import type { LogEntry, LogLevel } from "../api/get-container-logs-parsed";
import {
	getHistoryLogs,
	type HistoryLogsPage,
	type HistoryScope,
} from "../api/get-history";
import { stripProjectPrefix } from "../components/container-utils";

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

// Stack rows drop the "<project>-" prefix, as in the live stack view. Done
// once per page as it arrives, not per render.
export function stripPageProjectPrefix(
	page: HistoryLogsPage,
	project: string,
): HistoryLogsPage {
	return {
		...page,
		logs: page.logs.map((log) =>
			log.containerName
				? {
						...log,
						containerName: stripProjectPrefix(log.containerName, project),
					}
				: log,
		),
	};
}

interface UseHistoryLogsOptions {
	enabled: boolean;
	scope: HistoryScope;
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
	scope,
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
			scope.container ?? "",
			scope.project ?? "",
			host ?? "",
			since ?? "",
			until ?? "",
			levelList.join(","),
			search,
			regex,
		],
		queryFn: async ({ pageParam }) => {
			const page = await getHistoryLogs({
				...scope,
				host,
				since,
				until,
				levels: levelList,
				search: search || undefined,
				regex,
				limit: HISTORY_PAGE_SIZE,
				cursor: pageParam || undefined,
			});
			return scope.project ? stripPageProjectPrefix(page, scope.project) : page;
		},
		initialPageParam: "",
		getNextPageParam: (lastPage) => lastPage.nextCursor || undefined,
		enabled,
		retry: false,
		staleTime: 30_000,
	});

	const logs = useMemo(
		() => flattenHistoryPages(query.data?.pages ?? []),
		[query.data],
	);

	return {
		logs,
		// How far back the oldest loaded page searched, when the server's scan
		// budget stopped it early.
		scannedTo: query.data?.pages.at(-1)?.scannedTo,
		error: query.error,
		isLoading: query.isLoading,
		isFetchingOlder: query.isFetchingNextPage,
		hasOlder: query.hasNextPage,
		fetchOlder: query.fetchNextPage,
		refetch: query.refetch,
	};
}
