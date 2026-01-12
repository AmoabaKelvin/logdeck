import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";

import { getContainerStats } from "../api/get-container-stats";
import type { ContainerStatsMap } from "../types";

export function useContainerStats() {
  const query = useQuery({
    queryKey: ["containers", "stats"],
    queryFn: getContainerStats,
    refetchInterval: 5000, // Refresh every 5 seconds
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

  return {
    ...query,
    statsMap,
  };
}
