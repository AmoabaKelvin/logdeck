import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { type CreateBackupPayload, createBackup } from "../api/create-backup";
import { deleteBackup } from "../api/delete-backup";
import { getBackups } from "../api/get-backups";
import {
	cancelRestore,
	checkRestore,
	confirmRestore,
	type RestoreSource,
} from "../api/restore-backup";
import { updateBackupSchedule } from "../api/update-backup-schedule";
import type { BackupSchedule } from "../types";

const BACKUPS_KEY = ["settings", "backups"] as const;

/** Polls while a backup runs, so the list fills in when it finishes. */
export function useBackups() {
	return useQuery({
		queryKey: BACKUPS_KEY,
		queryFn: getBackups,
		staleTime: 30_000,
		refetchInterval: (query) => (query.state.data?.running ? 2_000 : false),
	});
}

export function useCreateBackup() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (payload: CreateBackupPayload) => createBackup(payload),
		onSettled: () => {
			queryClient.invalidateQueries({ queryKey: BACKUPS_KEY });
		},
	});
}

export function useDeleteBackup() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (name: string) => deleteBackup(name),
		onSettled: () => {
			queryClient.invalidateQueries({ queryKey: BACKUPS_KEY });
		},
	});
}

export function useUpdateBackupSchedule() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (schedule: BackupSchedule) => updateBackupSchedule(schedule),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: BACKUPS_KEY });
		},
	});
}

export function useCheckRestore() {
	return useMutation({
		mutationFn: (input: { source: RestoreSource; passphrase: string }) =>
			checkRestore(input.source, input.passphrase),
	});
}

export function useConfirmRestore() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: confirmRestore,
		onSettled: () => {
			queryClient.invalidateQueries({ queryKey: BACKUPS_KEY });
		},
	});
}

export function useCancelRestore() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: cancelRestore,
		onSettled: () => {
			queryClient.invalidateQueries({ queryKey: BACKUPS_KEY });
		},
	});
}
