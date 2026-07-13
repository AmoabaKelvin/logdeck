import { useQuery } from "@tanstack/react-query";

import { getHistoryStatus } from "../api/get-history";

// Whether the server persists logs at all. The answer only changes on a server
// restart, so it is fetched once and cached for the session. A failure (older
// server without the endpoint) is treated as "not enabled" by the callers,
// which read `data?.enabled`.
export function useHistoryStatus(enabled = true) {
	return useQuery({
		queryKey: ["history", "status"],
		queryFn: getHistoryStatus,
		enabled,
		retry: false,
		staleTime: Number.POSITIVE_INFINITY,
		gcTime: Number.POSITIVE_INFINITY,
		refetchOnWindowFocus: false,
	});
}
