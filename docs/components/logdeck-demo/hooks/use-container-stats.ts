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

  const stats = query.data?.stats;
  const statsMap = useMemo<ContainerStatsMap>(() => {
    if (!stats) return {};

    return stats.reduce<ContainerStatsMap>((acc, stat) => {
      acc[stat.id] = stat;
      return acc;
    }, {});
  }, [stats]);

  const statsHistory = useContainerStatsHistory(stats);

  return {
    ...query,
    statsMap,
    statsHistory,
  };
}
