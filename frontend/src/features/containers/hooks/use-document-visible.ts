import { useSyncExternalStore } from "react";

function subscribe(onStoreChange: () => void) {
  document.addEventListener("visibilitychange", onStoreChange);
  return () => document.removeEventListener("visibilitychange", onStoreChange);
}

/**
 * Tracks whether the document is currently visible (tab in the foreground).
 */
export function useDocumentVisible(): boolean {
  return useSyncExternalStore(
    subscribe,
    () => !document.hidden,
    () => true
  );
}
