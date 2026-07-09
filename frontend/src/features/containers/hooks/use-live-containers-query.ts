import { useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useRef } from "react";

import { useContainerEvents } from "./use-container-events";
import { useContainersQuery } from "./use-containers-query";

const EVENT_INVALIDATE_DEBOUNCE_MS = 300;

/**
 * Containers query kept fresh by the container event stream: lifecycle
 * events invalidate the query (debounced so an event burst doesn't stampede
 * refetches), with polling as fallback while the stream is disconnected.
 */
export function useLiveContainersQuery() {
  const queryClient = useQueryClient();
  const invalidateTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(
    null
  );

  const handleContainerEvent = useCallback(() => {
    if (invalidateTimeoutRef.current !== null) return;
    invalidateTimeoutRef.current = setTimeout(() => {
      invalidateTimeoutRef.current = null;
      void queryClient.invalidateQueries({
        queryKey: ["containers"],
        exact: true,
      });
    }, EVENT_INVALIDATE_DEBOUNCE_MS);
  }, [queryClient]);

  useEffect(() => {
    return () => {
      if (invalidateTimeoutRef.current !== null) {
        clearTimeout(invalidateTimeoutRef.current);
      }
    };
  }, []);

  const { isConnected } = useContainerEvents(handleContainerEvent);

  return useContainersQuery({ eventsConnected: isConnected });
}
