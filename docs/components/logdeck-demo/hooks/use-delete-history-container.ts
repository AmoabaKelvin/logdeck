import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { deleteHistoryContainer } from "../api/get-history";

export interface DeleteHistoryTarget {
	name: string;
	host: string;
}

/**
 * Permanently deletes a container's stored logs. Invalidating the history and
 * container queries drops the removed row from the dashboard and updates the
 * "Removed" chip count.
 */
export function useDeleteHistoryContainer() {
	const queryClient = useQueryClient();

	return useMutation({
		mutationFn: ({ name, host }: DeleteHistoryTarget) =>
			deleteHistoryContainer(name, host),
		onSuccess: (result) => {
			toast.success(result.message, {
				description:
					result.linesDeleted > 0
						? `${result.linesDeleted.toLocaleString()} log lines deleted.`
						: undefined,
			});
			queryClient.invalidateQueries({ queryKey: ["history", "containers"] });
			queryClient.invalidateQueries({ queryKey: ["history", "status"] });
			queryClient.invalidateQueries({ queryKey: ["containers"] });
		},
		onError: (error: Error) => toast.error(error.message),
	});
}
