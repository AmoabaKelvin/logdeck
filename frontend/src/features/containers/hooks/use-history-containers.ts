import { keepPreviousData, useQuery } from "@tanstack/react-query";

import {
	getHistoryContainers,
	getStoredContainersPage,
	type StoredContainersPageParams,
} from "../api/get-history";

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

// One page of stored containers, filtered and sorted by the server. The
// previous page stays on screen while the next one loads.
export function useStoredContainersPage(
	params: StoredContainersPageParams,
	enabled = true,
) {
	return useQuery({
		queryKey: ["history", "containers", params],
		queryFn: () => getStoredContainersPage(params),
		enabled,
		retry: false,
		staleTime: 30_000,
		placeholderData: keepPreviousData,
	});
}
