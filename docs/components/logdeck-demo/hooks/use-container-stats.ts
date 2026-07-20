import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";

import { getContainerStats } from "../api/get-container-stats";
import type { ContainerStatsMap } from "../types";

import { useContainerStatsHistory } from "./use-stats-history";
import { useDocumentVisible } from "./use-document-visible";

export function useContainerStats() {
	const isVisible = useDocumentVisible();
	const query = useQuery({
		queryKey: ["containers", "stats"],
		queryFn: getContainerStats,
		refetchInterval: isVisible ? 5000 : false,
		staleTime: 4000,
	});

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
