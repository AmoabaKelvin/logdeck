import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";

import {
  getContainerStatsDemo,
  getContainersDemo,
  getSystemStatsDemo,
} from "@/lib/logdeck-demo/demo-api";

import type { ContainerStatsMap } from "./types";

export function useContainersQuery() {
  return useQuery({
    queryKey: ["containers"],
    queryFn: getContainersDemo,
    staleTime: 15_000,
  });
}

export function useSystemStats() {
  return useQuery({
    queryKey: ["system-stats"],
    queryFn: getSystemStatsDemo,
    refetchInterval: 2_000,
  });
}

export function useContainerStats() {
  const query = useQuery({
    queryKey: ["containers", "stats"],
    queryFn: getContainerStatsDemo,
    refetchInterval: 5_000,
    staleTime: 4_000,
  });

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
