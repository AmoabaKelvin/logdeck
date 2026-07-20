import { useSyncExternalStore } from "react";

function subscribe(onStoreChange: () => void) {
	document.addEventListener("visibilitychange", onStoreChange);
	return () => document.removeEventListener("visibilitychange", onStoreChange);
}

export function useDocumentVisible(): boolean {
	return useSyncExternalStore(
		subscribe,
		() => !document.hidden,
		() => true,
	);
}
