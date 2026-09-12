import { useQuery } from "@tanstack/react-query";

import { getContainerInspect } from "../api/get-container-inspect";
import { useDocumentVisible } from "./use-document-visible";

/**
 * The container's full inspect payload. Restart counts and health probes move
 * on their own schedule, so this polls while the tab is visible rather than
 * waiting for a container event.
 */
export function useContainerInspect(
	id: string | undefined,
	host: string | undefined,
) {
	const isVisible = useDocumentVisible();

	return useQuery({
		queryKey: ["containers", id, "inspect", host],
		queryFn: () => getContainerInspect(id ?? "", host ?? ""),
		enabled: Boolean(id && host),
		refetchInterval: isVisible ? 10000 : false,
		staleTime: 8000,
	});
}
