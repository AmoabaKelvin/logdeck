import { describe, expect, it } from "vitest";

import { describeContents, validateSchedule } from "./backup-utils";

const empty = {
	dockerHosts: 0,
	coolifyHosts: 0,
	dokployHosts: 0,
	alertRules: 0,
	alertChannels: 0,
	alertHistory: 0,
	apiTokens: 0,
	logsBytes: 0,
};

describe("describeContents", () => {
	it("names only what the backup holds, with singular and plural forms", () => {
		expect(
			describeContents({
				...empty,
				dockerHosts: 1,
				coolifyHosts: 1,
				alertRules: 1,
				alertHistory: 12,
				logsBytes: 2048,
			}),
		).toBe("2 connections, 1 alert rule, 12 past alerts, 2 KB of logs");
	});

	it("says so when there is nothing beyond the defaults", () => {
		expect(describeContents(empty)).toBe("Default settings only");
	});
});

describe("validateSchedule", () => {
	it("accepts an interval of 0 to turn the schedule off", () => {
		expect(validateSchedule("0", "7")).toBeNull();
		expect(validateSchedule("24", "1")).toBeNull();
	});

	it("rejects fractional, negative, empty and oversized intervals", () => {
		for (const value of ["1.5", "-1", "", "721"]) {
			expect(validateSchedule(value, "7")).toMatch(/interval/i);
		}
	});

	it("rejects keeping fewer than one backup", () => {
		expect(validateSchedule("24", "0")).toMatch(/keep/i);
		expect(validateSchedule("24", "")).toMatch(/keep/i);
	});
});
