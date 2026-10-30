import { describe, expect, it } from "vitest";

import type { CollectionHealthReport } from "../api/get-collection-health";
import {
	collectionProblems,
	describeLive,
	describeStored,
} from "./collection-health";

const since = "2026-01-01T00:00:00Z";

const report: CollectionHealthReport = {
	status: "degraded",
	hosts: [
		{
			host: "local",
			state: "ok",
			since,
			containers: [
				{ id: "a", name: "web", live: { state: "ok", since } },
				{
					id: "b",
					name: "worker",
					live: { state: "failing", since, error: "EOF" },
				},
				{ id: "c", name: "syslogged", live: { state: "unsupported", since } },
				{ id: "d", name: "secret", stored: { state: "excluded", since } },
			],
		},
		{
			host: "remote",
			state: "failing",
			since,
			error: "connection refused",
			containers: [{ id: "e", name: "api", live: { state: "failing", since } }],
		},
	],
	storage: {
		state: "ok",
		since,
		lowSpace: true,
		freeBytes: 0,
		droppedLines: 0,
		backlog: 0,
		backlogCap: 8192,
		recovering: 0,
	},
};

describe("collectionProblems", () => {
	it("reports failures once per host and leaves deliberate gaps out", () => {
		expect(collectionProblems(report)).toEqual([
			"local: can't collect logs from worker",
			"remote: connection refused",
			"Log storage is running out of disk space",
		]);
	});

	it("can leave unreachable hosts to the caller", () => {
		expect(collectionProblems(report, { includeHosts: false })).toEqual([
			"local: can't collect logs from worker",
			"Log storage is running out of disk space",
		]);
	});
});

describe("describeLive / describeStored", () => {
	it("keeps a quiet container, a broken one and a deliberate gap apart", () => {
		expect(describeLive({ state: "ok", since }).tone).toBe("ok");
		expect(describeLive({ state: "failing", since }).tone).toBe("bad");
		expect(describeLive({ state: "unsupported", since }).tone).toBe("muted");
		expect(describeStored({ state: "excluded", since }).tone).toBe("muted");
		expect(describeLive(undefined).tone).toBe("muted");
	});
});
