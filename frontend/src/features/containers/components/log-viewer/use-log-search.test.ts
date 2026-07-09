import { describe, expect, it } from "vitest";

import type {
	LogEntry,
	LogLevel,
} from "@/features/containers/api/get-container-logs-parsed";
import {
	computeSearchMatches,
	EMPTY_SEARCH,
	parseSearch,
} from "./use-log-search";

const entry = (message: string, level: LogLevel = "INFO"): LogEntry => ({
	level,
	message,
});

describe("parseSearch", () => {
	it("returns the empty result for plain-text searches", () => {
		expect(parseSearch("hello", false)).toBe(EMPTY_SEARCH);
		expect(parseSearch("", true)).toBe(EMPTY_SEARCH);
	});

	it("compiles a case-insensitive global regex", () => {
		const parsed = parseSearch("err(or)?", true);

		expect(parsed.error).toBeNull();
		expect(parsed.regex).toBeInstanceOf(RegExp);
		expect(parsed.regex?.flags).toBe("gi");
		expect(parsed.regex?.test("ERROR")).toBe(true);
	});

	it("reports invalid regex input without throwing", () => {
		const parsed = parseSearch("(unclosed", true);

		expect(parsed.regex).toBeNull();
		expect(parsed.error).toBe("Invalid regex");
	});
});

describe("computeSearchMatches", () => {
	const logs = [
		entry("request started"),
		entry("Request FAILED", "ERROR"),
		entry("cleanup done"),
		entry("request finished"),
	];

	it("returns no matches for an empty search", () => {
		expect(computeSearchMatches(logs, "", false, EMPTY_SEARCH)).toEqual([]);
	});

	it("matches plain text case-insensitively", () => {
		expect(computeSearchMatches(logs, "REQUEST", false, EMPTY_SEARCH)).toEqual([
			0, 1, 3,
		]);
	});

	it("matches with a regex when regex mode is on", () => {
		const searchText = "request (started|finished)";
		const matches = computeSearchMatches(
			logs,
			searchText,
			true,
			parseSearch(searchText, true),
		);

		expect(matches).toEqual([0, 3]);
	});

	it("is not affected by regex lastIndex state across entries", () => {
		// A global regex keeps lastIndex between test() calls unless it is
		// reset; identical consecutive messages would flip-flop otherwise.
		const repeated = [entry("error"), entry("error"), entry("error")];
		const searchText = "error";
		const matches = computeSearchMatches(
			repeated,
			searchText,
			true,
			parseSearch(searchText, true),
		);

		expect(matches).toEqual([0, 1, 2]);
	});

	it("returns no matches for an invalid regex", () => {
		const searchText = "(unclosed";
		const matches = computeSearchMatches(
			logs,
			searchText,
			true,
			parseSearch(searchText, true),
		);

		expect(matches).toEqual([]);
	});

	it("falls back to the raw line when the message is empty", () => {
		const rawOnly: LogEntry = { level: "UNKNOWN", raw: "raw request line" };
		expect(
			computeSearchMatches([rawOnly], "request", false, EMPTY_SEARCH),
		).toEqual([0]);
	});
});
