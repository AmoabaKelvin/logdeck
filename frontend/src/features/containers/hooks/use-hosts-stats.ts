import { useQuery } from "@tanstack/react-query";

import { getHostsStats } from "../api/get-hosts-stats";

import { useDocumentVisible } from "./use-document-visible";

export function useHostsStats(enabled: boolean) {
	const isVisible = useDocumentVisible();
	return useQuery({
		queryKey: ["hosts-stats"],
		queryFn: getHostsStats,
		enabled,
		refetchInterval: isVisible ? 10000 : false, // Refresh every 10 seconds while the page is visible
	});
}
