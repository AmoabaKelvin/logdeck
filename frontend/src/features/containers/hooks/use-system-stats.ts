import { useQuery } from "@tanstack/react-query";

import { getSystemStats } from "../api/get-system-stats";

import { useDocumentVisible } from "./use-document-visible";

export function useSystemStats() {
	const isVisible = useDocumentVisible();
	return useQuery({
		queryKey: ["system-stats"],
		queryFn: getSystemStats,
		refetchInterval: isVisible ? 2000 : false,
	});
}
