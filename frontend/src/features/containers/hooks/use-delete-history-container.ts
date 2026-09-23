import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import {
	deleteAllHistory,
	deleteHistoryContainer,
	deleteRemovedHistory,
} from "../api/get-history";

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

/** Purges every removed container's stored logs in one go. */
export function useDeleteRemovedHistory() {
	const queryClient = useQueryClient();

	return useMutation({
		mutationFn: deleteRemovedHistory,
		onSuccess: (result) => {
			toast.success(
				`Deleted stored logs for ${result.containersDeleted} removed ${
					result.containersDeleted === 1 ? "container" : "containers"
				}`,
				{
					description:
						result.linesDeleted > 0
							? `${result.linesDeleted.toLocaleString()} log lines deleted.`
							: undefined,
				},
			);
			queryClient.invalidateQueries({ queryKey: ["history", "containers"] });
			queryClient.invalidateQueries({ queryKey: ["history", "status"] });
			queryClient.invalidateQueries({ queryKey: ["containers"] });
		},
		onError: (error: Error) => toast.error(error.message),
	});
}

export function useDeleteAllHistory() {
	const queryClient = useQueryClient();

	return useMutation({
		mutationFn: deleteAllHistory,
		onSuccess: (result) => {
			toast.success(
				`Deleted stored logs for ${result.containersDeleted} ${
					result.containersDeleted === 1 ? "container" : "containers"
				}`,
				{
					description:
						result.linesDeleted > 0
							? `${result.linesDeleted.toLocaleString()} log lines deleted.`
							: undefined,
				},
			);
			queryClient.invalidateQueries({ queryKey: ["history"] });
			queryClient.invalidateQueries({ queryKey: ["containers"] });
		},
		onError: (error: Error) => toast.error(error.message),
	});
}
