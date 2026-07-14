import { authenticatedFetch } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

const ENDPOINT = `${API_BASE_URL}/api/v1/alerts/channels`;

export interface AlertTestResult {
	status: "ok" | "failed";
	httpStatus?: number;
	error?: string;
}

export async function testAlertChannel(id: string): Promise<AlertTestResult> {
	const response = await authenticatedFetch(
		`${ENDPOINT}/${encodeURIComponent(id)}/test`,
		{ method: "POST" },
	);

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to send test alert");
	}

	return (await response.json()) as AlertTestResult;
}
