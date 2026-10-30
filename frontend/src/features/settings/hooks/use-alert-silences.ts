import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
	type AlertSilencePayload,
	createAlertSilence,
} from "../api/create-alert-silence";
import { deleteAlertSilence } from "../api/delete-alert-silence";
import { getAlertSilences } from "../api/get-alert-silences";

const SILENCES_KEY = ["alerts", "silences"] as const;

export function useAlertSilences() {
	return useQuery({
		queryKey: SILENCES_KEY,
		queryFn: getAlertSilences,
		// Whether a silence is muting changes with the clock, not only on edits.
		refetchInterval: 60_000,
	});
}

export function useCreateAlertSilence() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (silence: AlertSilencePayload) => createAlertSilence(silence),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: SILENCES_KEY });
		},
	});
}

export function useDeleteAlertSilence() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (id: string) => deleteAlertSilence(id),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: SILENCES_KEY });
		},
	});
}
