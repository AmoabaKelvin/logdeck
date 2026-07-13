import { authenticatedFetch } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

import type { AlertRuleType } from "./get-alert-rules";

const ENDPOINT = `${API_BASE_URL}/api/v1/alerts/history`;

export interface AlertDelivery {
	status: string;
	httpStatus?: number;
	error?: string;
}

export interface AlertHistoryEntry {
	id: string;
	ruleId: string;
	ruleName: string;
	type: AlertRuleType;
	host: string;
	containerId: string;
	containerName: string;
	reason: string;
	sample?: string;
	count: number;
	suppressed: number;
	firedAt: string;
	delivery?: AlertDelivery;
}

export interface AlertHistoryResponse {
	alerts: AlertHistoryEntry[];
	count: number;
}

export async function getAlertHistory(
	limit: number,
): Promise<AlertHistoryResponse> {
	const response = await authenticatedFetch(`${ENDPOINT}?limit=${limit}`);

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to load alert history");
	}

	return (await response.json()) as AlertHistoryResponse;
}
