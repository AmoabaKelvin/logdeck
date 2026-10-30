import { authenticatedFetch, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

const ENDPOINT = `${API_BASE_URL}/api/v1/alerts/silences`;

export type SilenceDay = "mon" | "tue" | "wed" | "thu" | "fri" | "sat" | "sun";

/** A weekly maintenance window: from start on each day, for durationMinutes. */
export interface SilenceSchedule {
	/** Empty means every day. */
	days?: SilenceDay[];
	/** Time of day, "15:04". */
	start: string;
	durationMinutes: number;
	/** IANA name; empty means UTC. */
	timezone?: string;
}

export interface AlertSilence {
	id: string;
	comment?: string;
	ruleId?: string;
	host?: string;
	container?: string;
	project?: string;
	startsAt?: string;
	endsAt?: string;
	schedule?: SilenceSchedule;
	createdAt: string;
	/** Whether it is muting right now. */
	active: boolean;
	/** When the current muting ends; absent while active means until deleted. */
	until?: string;
}

export interface AlertSilencesResponse {
	silences: AlertSilence[];
}

export async function getAlertSilences(): Promise<AlertSilencesResponse> {
	const response = await authenticatedFetch(ENDPOINT);

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to load silences");
	}

	return readJson<AlertSilencesResponse>(response);
}
