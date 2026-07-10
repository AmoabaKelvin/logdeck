import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { createApiToken } from "../api/create-api-token";
import { deleteApiToken } from "../api/delete-api-token";
import { getApiTokens } from "../api/get-api-tokens";
import { getSettings } from "../api/get-settings";
import { testCoolifyHost } from "../api/test-coolify-host";
import { testDockerHost } from "../api/test-docker-host";
import { type UpdateAuthPayload, updateAuth } from "../api/update-auth";
import { updateCoolifyHosts } from "../api/update-coolify-hosts";
import { updateDockerHosts } from "../api/update-docker-hosts";
import { updateReadOnly } from "../api/update-read-only";

const SETTINGS_KEY = ["settings"] as const;
const API_TOKENS_KEY = ["settings", "api-tokens"] as const;

export function useSettings() {
	return useQuery({
		queryKey: SETTINGS_KEY,
		queryFn: getSettings,
		staleTime: 30_000,
	});
}

export function useUpdateDockerHosts() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (hosts: { name: string; host: string }[]) =>
			updateDockerHosts(hosts),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: SETTINGS_KEY });
		},
	});
}

export function useUpdateCoolifyHosts() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (
			hosts: { hostName: string; apiURL: string; apiToken: string }[],
		) => updateCoolifyHosts(hosts),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: SETTINGS_KEY });
		},
	});
}

export function useUpdateReadOnly() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (value: boolean) => updateReadOnly(value),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: SETTINGS_KEY });
		},
	});
}

export function useUpdateAuth() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (payload: UpdateAuthPayload) => updateAuth(payload),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: SETTINGS_KEY });
		},
	});
}

export function useApiTokens() {
	return useQuery({
		queryKey: API_TOKENS_KEY,
		queryFn: getApiTokens,
		staleTime: 30_000,
	});
}

export function useCreateApiToken() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (name: string) => createApiToken(name),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: API_TOKENS_KEY });
		},
	});
}

export function useDeleteApiToken() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (prefix: string) => deleteApiToken(prefix),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: API_TOKENS_KEY });
		},
	});
}

export function useTestDockerHost() {
	return useMutation({
		mutationFn: ({ name, host }: { name: string; host: string }) =>
			testDockerHost(name, host),
	});
}

export function useTestCoolifyHost() {
	return useMutation({
		mutationFn: ({
			hostName,
			apiURL,
			apiToken,
		}: {
			hostName: string;
			apiURL: string;
			apiToken: string;
		}) => testCoolifyHost(hostName, apiURL, apiToken),
	});
}
