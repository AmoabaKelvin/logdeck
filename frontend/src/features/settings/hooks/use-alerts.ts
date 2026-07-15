import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { clearAlertHistory } from "../api/clear-alert-history";
import {
	type AlertChannelPayload,
	createAlertChannel,
} from "../api/create-alert-channel";
import {
	type AlertRulePayload,
	createAlertRule,
} from "../api/create-alert-rule";
import { deleteAlertChannel } from "../api/delete-alert-channel";
import { deleteAlertRule } from "../api/delete-alert-rule";
import { getAlertChannels } from "../api/get-alert-channels";
import { getAlertHistory } from "../api/get-alert-history";
import { getAlertRules } from "../api/get-alert-rules";
import { testAlertChannel } from "../api/test-alert-channel";
import { updateAlertChannel } from "../api/update-alert-channel";
import { updateAlertRule } from "../api/update-alert-rule";

const RULES_KEY = ["alerts", "rules"] as const;
const CHANNELS_KEY = ["alerts", "channels"] as const;
const HISTORY_KEY = ["alerts", "history"] as const;

export function useAlertRules() {
	return useQuery({
		queryKey: RULES_KEY,
		queryFn: getAlertRules,
		staleTime: 30_000,
	});
}

export function useCreateAlertRule() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (rule: AlertRulePayload) => createAlertRule(rule),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: RULES_KEY });
		},
	});
}

export function useUpdateAlertRule() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: ({ id, rule }: { id: string; rule: AlertRulePayload }) =>
			updateAlertRule(id, rule),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: RULES_KEY });
		},
	});
}

export function useDeleteAlertRule() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (id: string) => deleteAlertRule(id),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: RULES_KEY });
		},
	});
}

export function useAlertChannels() {
	return useQuery({
		queryKey: CHANNELS_KEY,
		queryFn: getAlertChannels,
		staleTime: 30_000,
	});
}

export function useCreateAlertChannel() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (channel: AlertChannelPayload) => createAlertChannel(channel),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: CHANNELS_KEY });
		},
	});
}

export function useUpdateAlertChannel() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: ({
			id,
			channel,
		}: {
			id: string;
			channel: AlertChannelPayload;
		}) => updateAlertChannel(id, channel),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: CHANNELS_KEY });
		},
	});
}

export function useDeleteAlertChannel() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (id: string) => deleteAlertChannel(id),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: CHANNELS_KEY });
		},
	});
}

export function useTestAlertChannel() {
	return useMutation({
		mutationFn: (id: string) => testAlertChannel(id),
	});
}

export function useAlertHistory(limit: number) {
	return useQuery({
		queryKey: [...HISTORY_KEY, limit],
		queryFn: () => getAlertHistory(limit),
		refetchInterval: 30_000,
		refetchOnWindowFocus: true,
	});
}

export function useClearAlertHistory() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: () => clearAlertHistory(),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: HISTORY_KEY });
		},
	});
}
