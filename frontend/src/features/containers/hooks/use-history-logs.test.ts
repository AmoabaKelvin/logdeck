import { describe, expect, it } from "vitest";

import type { LogEntry } from "../api/get-container-logs-parsed";
import type { HistoryLogsPage } from "../api/get-history";
import { flattenHistoryPages } from "./use-history-logs";

const entry = (message: string): LogEntry => ({ level: "INFO", message });

const page = (messages: string[], nextCursor?: string): HistoryLogsPage => ({
	logs: messages.map(entry),
	count: messages.length,
	nextCursor,
});

describe("flattenHistoryPages", () => {
	it("returns an empty list when there are no pages", () => {
		expect(flattenHistoryPages([])).toEqual([]);
	});

	it("keeps a single page in its ascending order", () => {
		const logs = flattenHistoryPages([page(["a", "b", "c"])]);

		expect(logs.map((log) => log.message)).toEqual(["a", "b", "c"]);
	});

	it("puts older pages before newer ones", () => {
		// Page 0 is the newest window; each further page walks back in time.
		const logs = flattenHistoryPages([
			page(["e", "f"], "cursor-1"),
			page(["c", "d"], "cursor-2"),
			page(["a", "b"]),
		]);

		expect(logs.map((log) => log.message)).toEqual([
			"a",
			"b",
			"c",
			"d",
			"e",
			"f",
		]);
	});

	it("skips empty pages without disturbing the order", () => {
		const logs = flattenHistoryPages([
			page(["c"], "cursor-1"),
			page([], "cursor-2"),
			page(["a"]),
		]);

		expect(logs.map((log) => log.message)).toEqual(["a", "c"]);
	});
});
