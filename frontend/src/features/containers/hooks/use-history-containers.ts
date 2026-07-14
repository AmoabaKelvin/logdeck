import { useQuery } from "@tanstack/react-query";

import { getHistoryContainers } from "../api/get-history";

// Containers the log store has data for, including ones that no longer exist
// on the host.
export function useHistoryContainers(enabled = true) {
	return useQuery({
		queryKey: ["history", "containers"],
		queryFn: getHistoryContainers,
		enabled,
		retry: false,
		staleTime: 30_000,
	});
}
