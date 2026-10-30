import { authenticatedFetch, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

const ENDPOINT = `${API_BASE_URL}/api/v1/alerts/rules`;

export type AlertRuleType = "event" | "log" | "collection" | "resource";
export type AlertEventKind = "die" | "oom" | "unhealthy" | "restart";
export type AlertResourceMetric = "cpu" | "memory";

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
	metric?: AlertResourceMetric;
	/** CPU as docker stats shows it (100 is one core), or memory as a share of its limit. */
	abovePercent?: number;
	aboveBytes?: number;
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

	return readJson<AlertRulesResponse>(response);
}
