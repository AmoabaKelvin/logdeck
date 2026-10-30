import { useQuery } from "@tanstack/react-query";

import { getCollectionHealth } from "../api/get-collection-health";

import { useDocumentVisible } from "./use-document-visible";

// The server samples hosts once a minute, so polling faster shows nothing new.
// A failure (older server without the endpoint) leaves data undefined, which
// callers treat as "nothing to report".
export function useCollectionHealth() {
	const isVisible = useDocumentVisible();
	return useQuery({
		queryKey: ["health", "collection"],
		queryFn: getCollectionHealth,
		retry: false,
		refetchInterval: isVisible ? 30_000 : false,
	});
}
