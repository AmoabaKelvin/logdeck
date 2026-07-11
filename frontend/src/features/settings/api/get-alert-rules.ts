import { authenticatedFetch } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

const ENDPOINT = `${API_BASE_URL}/api/v1/alerts/rules`;

export type AlertRuleType = "event" | "log";
export type AlertEventKind = "die" | "oom";

export interface AlertRule {
	id: string;
	name: string;
	enabled: boolean;
	type: AlertRuleType;
	hosts?: string[];
	containers?: string[];
	projects?: string[];
	events?: AlertEventKind[];
	minLevel?: string;
	pattern?: string;
	threshold: number;
	windowSeconds?: number;
	cooldownSeconds?: number;
	createdAt: string;
}

export interface AlertRulesResponse {
	rules: AlertRule[];
}

export async function getAlertRules(): Promise<AlertRulesResponse> {
	const response = await authenticatedFetch(ENDPOINT);

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to load alert rules");
	}

	return (await response.json()) as AlertRulesResponse;
}
