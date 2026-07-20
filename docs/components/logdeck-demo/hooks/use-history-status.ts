import { useQuery } from "@tanstack/react-query";

import { getHistoryStatus } from "../api/get-history";

// Whether the server persists logs at all. Always true in the demo, but the
// query shape matches the real app so ported components work unchanged.
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
