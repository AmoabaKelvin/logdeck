import { describe, expect, it } from "vitest";

import {
	groupRelatedLogEntries,
	type LogEntry,
} from "./get-container-logs-parsed";

const t = "2026-09-14T13:51:07.814Z";

describe("groupRelatedLogEntries", () => {
	it("folds server-flagged continuations into the entry before them", () => {
		const grouped = groupRelatedLogEntries<LogEntry>([
			{
				timestamp: t,
				level: "ERROR",
				message: "Traceback (most recent call last):",
			},
			{
				timestamp: "2026-09-14T13:51:07.900Z",
				level: "UNKNOWN",
				message: 'File "/app/main.py"',
				continuation: true,
			},
			{
				timestamp: "2026-09-14T13:51:07.901Z",
				level: "UNKNOWN",
				message: "requestId: 42",
				continuation: true,
			},
			{ timestamp: t, level: "INFO", message: "INFO done" },
		]);

		expect(
			grouped.map((g) => [g.level, g.continuationCount ?? 0, g.timestamp]),
		).toEqual([
			["ERROR", 2, t],
			["INFO", 0, t],
		]);
		expect(grouped[0].message).toBe(
			'Traceback (most recent call last):\nFile "/app/main.py"\nrequestId: 42',
		);
		expect(grouped[0].fields).toEqual({ requestId: "42" });
	});

	it("never folds across containers on an aggregate stream", () => {
		const grouped = groupRelatedLogEntries<LogEntry>([
			{ timestamp: t, level: "ERROR", message: "boom", containerName: "api" },
			{
				timestamp: t,
				level: "UNKNOWN",
				message: "at x",
				continuation: true,
				containerName: "worker",
			},
		]);

		expect(grouped).toHaveLength(2);
	});
});
