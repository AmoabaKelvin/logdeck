import { authenticatedFetch } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

import type { AlertRule } from "./get-alert-rules";

const ENDPOINT = `${API_BASE_URL}/api/v1/alerts/rules`;

export type AlertRulePayload = Omit<AlertRule, "id" | "createdAt">;

export async function createAlertRule(
	rule: AlertRulePayload,
): Promise<AlertRule> {
	const response = await authenticatedFetch(ENDPOINT, {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify(rule),
	});

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to create alert rule");
	}

	return (await response.json()) as AlertRule;
}
