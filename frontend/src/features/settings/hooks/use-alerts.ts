import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { clearAlertHistory } from "../api/clear-alert-history";
import {
	type AlertRulePayload,
	createAlertRule,
} from "../api/create-alert-rule";
import { deleteAlertRule } from "../api/delete-alert-rule";
import { getAlertHistory } from "../api/get-alert-history";
import { getAlertRules } from "../api/get-alert-rules";
import { getAlertWebhook } from "../api/get-alert-webhook";
import { testAlertWebhook } from "../api/test-alert-webhook";
import { updateAlertRule } from "../api/update-alert-rule";
import { updateAlertWebhook } from "../api/update-alert-webhook";

const RULES_KEY = ["alerts", "rules"] as const;
const WEBHOOK_KEY = ["alerts", "webhook"] as const;
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

export function useAlertWebhook() {
	return useQuery({
		queryKey: WEBHOOK_KEY,
		queryFn: getAlertWebhook,
		staleTime: 30_000,
	});
}

export function useUpdateAlertWebhook() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (url: string) => updateAlertWebhook(url),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: WEBHOOK_KEY });
		},
	});
}

export function useTestAlertWebhook() {
	return useMutation({
		mutationFn: () => testAlertWebhook(),
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
