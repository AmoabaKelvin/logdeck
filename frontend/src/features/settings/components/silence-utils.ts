import type { AlertSilencePayload } from "../api/create-alert-silence";
import type { AlertRule } from "../api/get-alert-rules";
import type { AlertSilence, SilenceDay } from "../api/get-alert-silences";

export const SILENCE_DAYS: { value: SilenceDay; label: string }[] = [
	{ value: "mon", label: "Mon" },
	{ value: "tue", label: "Tue" },
	{ value: "wed", label: "Wed" },
	{ value: "thu", label: "Thu" },
	{ value: "fri", label: "Fri" },
	{ value: "sat", label: "Sat" },
	{ value: "sun", label: "Sun" },
];

/** "" in a matcher field means any. */
export interface SilenceDraft {
	ruleId: string;
	host: string;
	container: string;
	project: string;
	comment: string;
	mode: "for" | "indefinite" | "between" | "weekly";
	forValue: string;
	forUnit: "minutes" | "hours" | "days";
	/** datetime-local values, in the browser's time zone. */
	from: string;
	until: string;
	days: SilenceDay[];
	at: string;
	durationMinutes: string;
	timezone: string;
}

export const EMPTY_SILENCE_DRAFT: SilenceDraft = {
	ruleId: "",
	host: "",
	container: "",
	project: "",
	comment: "",
	mode: "for",
	forValue: "1",
	forUnit: "hours",
	from: "",
	until: "",
	days: [],
	at: "02:00",
	durationMinutes: "60",
	timezone: "",
};

const UNIT_MS = { minutes: 60_000, hours: 3_600_000, days: 86_400_000 };

function rfc3339(date: Date): string {
	return date.toISOString().replace(/\.\d{3}Z$/, "Z");
}

export function buildSilencePayload(
	draft: SilenceDraft,
	now: Date,
): { error: string } | { payload: AlertSilencePayload } {
	const payload: AlertSilencePayload = {};
	if (draft.ruleId) payload.ruleId = draft.ruleId;
	if (draft.host) payload.host = draft.host;
	if (draft.container) payload.container = draft.container;
	if (draft.project) payload.project = draft.project;
	const comment = draft.comment.trim();
	if (comment) payload.comment = comment;

	switch (draft.mode) {
		case "indefinite":
			break;
		case "for": {
			const value = Number.parseFloat(draft.forValue);
			if (Number.isNaN(value) || value <= 0) {
				return { error: "Enter how long to mute for" };
			}
			payload.endsAt = rfc3339(
				new Date(now.getTime() + value * UNIT_MS[draft.forUnit]),
			);
			break;
		}
		case "between": {
			const from = new Date(draft.from);
			const until = new Date(draft.until);
			if (Number.isNaN(from.getTime()) || Number.isNaN(until.getTime())) {
				return { error: "Pick when the silence starts and ends" };
			}
			if (until <= from || until <= now) {
				return { error: "The end must be after the start and in the future" };
			}
			payload.startsAt = rfc3339(from);
			payload.endsAt = rfc3339(until);
			break;
		}
		case "weekly": {
			const minutes = Number.parseInt(draft.durationMinutes, 10);
			if (Number.isNaN(minutes) || minutes < 1 || minutes > 1440) {
				return { error: "The window must last between 1 and 1440 minutes" };
			}
			if (!/^\d{2}:\d{2}$/.test(draft.at)) {
				return { error: "Pick the time the window starts" };
			}
			payload.schedule = {
				start: draft.at,
				durationMinutes: minutes,
				...(draft.days.length > 0 && { days: draft.days }),
				...(draft.timezone && { timezone: draft.timezone }),
			};
			break;
		}
	}
	return { payload };
}

/** What a silence mutes, e.g. "rule Crash looping · project shop". */
export function describeSilenceTarget(
	silence: AlertSilence,
	rules: AlertRule[],
): string {
	const parts: string[] = [];
	if (silence.ruleId) {
		const rule = rules.find((r) => r.id === silence.ruleId);
		parts.push(`rule ${rule?.name ?? silence.ruleId}`);
	}
	if (silence.host) parts.push(`host ${silence.host}`);
	if (silence.container) parts.push(`container ${silence.container}`);
	if (silence.project) parts.push(`project ${silence.project}`);
	return parts.length > 0 ? parts.join(" · ") : "all alerts";
}

function formatDuration(minutes: number): string {
	if (minutes % 60 === 0) return `${minutes / 60} h`;
	if (minutes > 60) return `${Math.floor(minutes / 60)} h ${minutes % 60} min`;
	return `${minutes} min`;
}

/** When a silence mutes, e.g. "Muting until 14:00" or "Sat, Sun at 02:00 for 2 h". */
export function describeSilenceWhen(silence: AlertSilence): string {
	const time = (iso: string) => new Date(iso).toLocaleString();
	if (silence.schedule) {
		const { days, start, durationMinutes, timezone } = silence.schedule;
		const dayLabels = days?.length
			? SILENCE_DAYS.filter((d) => days.includes(d.value))
					.map((d) => d.label)
					.join(", ")
			: "Every day";
		const window = `${dayLabels} at ${start} for ${formatDuration(durationMinutes)} (${timezone || "UTC"})`;
		return silence.active && silence.until
			? `Muting until ${time(silence.until)} · ${window}`
			: window;
	}
	if (silence.active) {
		return silence.until
			? `Muting until ${time(silence.until)}`
			: "Muting until unmuted";
	}
	return silence.startsAt
		? `From ${time(silence.startsAt)}${silence.endsAt ? ` to ${time(silence.endsAt)}` : ""}`
		: "Not muting";
}
