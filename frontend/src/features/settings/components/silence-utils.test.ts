import { describe, expect, it } from "vitest";

import type { AlertSilence } from "../api/get-alert-silences";
import {
	buildSilencePayload,
	describeSilenceTarget,
	describeSilenceWhen,
	EMPTY_SILENCE_DRAFT,
} from "./silence-utils";

const NOW = new Date("2026-07-10T12:00:00Z");

describe("buildSilencePayload", () => {
	it("mutes a project for a while", () => {
		expect(
			buildSilencePayload(
				{
					...EMPTY_SILENCE_DRAFT,
					project: "shop",
					forValue: "2",
					comment: " deploy ",
				},
				NOW,
			),
		).toEqual({
			payload: {
				project: "shop",
				comment: "deploy",
				endsAt: "2026-07-10T14:00:00Z",
			},
		});
	});

	it("mutes everything until unmuted", () => {
		expect(
			buildSilencePayload({ ...EMPTY_SILENCE_DRAFT, mode: "indefinite" }, NOW),
		).toEqual({ payload: {} });
	});

	it("rejects a window that ends before it starts", () => {
		expect(
			buildSilencePayload(
				{
					...EMPTY_SILENCE_DRAFT,
					mode: "between",
					from: "2026-07-11T10:00",
					until: "2026-07-11T09:00",
				},
				NOW,
			),
		).toHaveProperty("error");
	});

	it("builds a weekly maintenance window", () => {
		expect(
			buildSilencePayload(
				{
					...EMPTY_SILENCE_DRAFT,
					host: "prod",
					mode: "weekly",
					days: ["sun"],
					at: "02:00",
					durationMinutes: "120",
					timezone: "Europe/Berlin",
				},
				NOW,
			),
		).toEqual({
			payload: {
				host: "prod",
				schedule: {
					days: ["sun"],
					start: "02:00",
					durationMinutes: 120,
					timezone: "Europe/Berlin",
				},
			},
		});
	});

	it("rejects a window longer than a day", () => {
		expect(
			buildSilencePayload(
				{ ...EMPTY_SILENCE_DRAFT, mode: "weekly", durationMinutes: "1441" },
				NOW,
			),
		).toHaveProperty("error");
	});
});

describe("describeSilence", () => {
	const base: AlertSilence = { id: "s1", createdAt: "", active: true };

	it("names the rule and the other matchers", () => {
		const rules = [
			{
				id: "r1",
				name: "Crash looping",
				enabled: true,
				type: "event" as const,
				threshold: 3,
				createdAt: "",
			},
		];
		expect(
			describeSilenceTarget({ ...base, ruleId: "r1", project: "shop" }, rules),
		).toBe("rule Crash looping · project shop");
		expect(describeSilenceTarget(base, rules)).toBe("all alerts");
	});

	it("describes an open-ended mute and a weekly window", () => {
		expect(describeSilenceWhen(base)).toBe("Muting until unmuted");
		expect(
			describeSilenceWhen({
				...base,
				active: false,
				schedule: { days: ["sat", "sun"], start: "02:00", durationMinutes: 90 },
			}),
		).toBe("Sat, Sun at 02:00 for 1 h 30 min (UTC)");
	});
});
