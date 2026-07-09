import { useQuery } from "@tanstack/react-query";

import { getContainers } from "../api/get-containers";

interface UseContainersQueryOptions {
  eventsConnected?: boolean;
}

export function useContainersQuery({
  eventsConnected = false,
}: UseContainersQueryOptions = {}) {
  return useQuery({
    queryKey: ["containers"],
    queryFn: getContainers,
    staleTime: 30_000,
    // With the event stream connected, updates are pushed; polling is only a
    // slow safety net.
    refetchInterval: eventsConnected ? 60_000 : undefined,
  });
}
