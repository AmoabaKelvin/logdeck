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
		refetchInterval: isVisible ? 5000 : false,
		staleTime: 4000,
	});

	const stats = query.data?.stats;
	const statsMap = useMemo(() => {
		const map: ContainerStatsMap = {};
		for (const stat of stats ?? []) {
			map[stat.id] = stat;
		}
		return map;
	}, [stats]);

	const statsHistory = useContainerStatsHistory(stats);

	return {
		...query,
		statsMap,
		statsHistory,
	};
}
