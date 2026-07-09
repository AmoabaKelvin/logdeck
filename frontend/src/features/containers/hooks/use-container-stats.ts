import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";

import { getContainerStats } from "../api/get-container-stats";
import type { ContainerStatsMap } from "../types";

import { useDocumentVisible } from "./use-document-visible";
import { useContainerStatsHistory } from "./use-stats-history";

export function useContainerStats() {
	const isVisible = useDocumentVisible();
	const query = useQuery({
		queryKey: ["containers", "stats"],
		queryFn: getContainerStats,
		refetchInterval: isVisible ? 5000 : false, // Refresh every 5 seconds while the page is visible
		staleTime: 4000, // Consider stale after 4 seconds
	});

	// Convert array to map for O(1) lookup by container ID
	const statsMap = useMemo<ContainerStatsMap>(() => {
		if (!query.data?.stats) return {};

		return query.data.stats.reduce((acc, stat) => {
			acc[stat.id] = stat;
			return acc;
		}, {} as ContainerStatsMap);
	}, [query.data?.stats]);

	const statsHistory = useContainerStatsHistory(query.data?.stats);

	return {
		...query,
		statsMap,
		statsHistory,
	};
}
