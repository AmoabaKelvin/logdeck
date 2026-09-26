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
import {
	type UpdateLogStoragePayload,
	updateLogStorage,
} from "../api/update-log-storage";
import { updateReadOnly } from "../api/update-read-only";
import type { APITokenScope } from "../types";

const SETTINGS_KEY = ["settings"] as const;
const API_TOKENS_KEY = ["settings", "api-tokens"] as const;
const HISTORY_STATUS_KEY = ["history", "status"] as const;
// The container list carries the host set and the read-only flag.
const CONTAINERS_KEY = ["containers"] as const;

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
		mutationFn: (input: {
			hosts: { name: string; host: string }[];
			revision?: string;
		}) => updateDockerHosts(input.hosts, input.revision),
		// Settled, not success: a 409 means the list changed under the user, and
		// refetching remounts the section with the current hosts and revision.
		onSettled: () => {
			queryClient.invalidateQueries({ queryKey: SETTINGS_KEY });
			queryClient.invalidateQueries({ queryKey: CONTAINERS_KEY });
		},
	});
}

export function useUpdateCoolifyHosts() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (input: {
			hosts: { hostName: string; apiURL: string; apiToken: string }[];
			revision?: string;
		}) => updateCoolifyHosts(input.hosts, input.revision),
		// Settled, not success: a 409 means the list changed under the user, and
		// refetching remounts the section with the current hosts and revision.
		onSettled: () => {
			queryClient.invalidateQueries({ queryKey: SETTINGS_KEY });
			queryClient.invalidateQueries({ queryKey: CONTAINERS_KEY });
		},
	});
}

export function useUpdateReadOnly() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (value: boolean) => updateReadOnly(value),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: SETTINGS_KEY });
			queryClient.invalidateQueries({ queryKey: CONTAINERS_KEY });
		},
	});
}

// The usage bar and the per-container cap text read their limits from
// /history/status, so that query is invalidated alongside the settings.
export function useUpdateLogStorage() {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (payload: UpdateLogStoragePayload) => updateLogStorage(payload),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: SETTINGS_KEY });
			queryClient.invalidateQueries({ queryKey: HISTORY_STATUS_KEY });
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
		mutationFn: ({ name, scope }: { name: string; scope: APITokenScope }) =>
			createApiToken(name, scope),
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
