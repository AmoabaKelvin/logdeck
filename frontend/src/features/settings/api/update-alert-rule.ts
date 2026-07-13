import { authenticatedFetch } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

import type { AlertRulePayload } from "./create-alert-rule";
import type { AlertRule } from "./get-alert-rules";

const ENDPOINT = `${API_BASE_URL}/api/v1/alerts/rules`;

export async function updateAlertRule(
	id: string,
	rule: AlertRulePayload,
): Promise<AlertRule> {
	const response = await authenticatedFetch(
		`${ENDPOINT}/${encodeURIComponent(id)}`,
		{
			method: "PUT",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify(rule),
		},
	);

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to update alert rule");
	}

	return (await response.json()) as AlertRule;
}
