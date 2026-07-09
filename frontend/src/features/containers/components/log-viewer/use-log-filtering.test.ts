import { describe, expect, it } from "vitest";

import type {
	LogEntry,
	LogLevel,
} from "@/features/containers/api/get-container-logs-parsed";
import { computeFilteredLogItems } from "./use-log-filtering";
import { EMPTY_SEARCH, parseSearch } from "./use-log-search";

const entry = (message: string, level: LogLevel = "INFO"): LogEntry => ({
	level,
	message,
});

const noSearch = {
	selectedLevels: new Set<LogLevel>(),
	excludeMatches: false,
	searchText: "",
	useRegex: false,
	searchParsed: EMPTY_SEARCH,
};

describe("computeFilteredLogItems", () => {
	const logs = [
		entry("starting up", "INFO"),
		entry("connection failed", "ERROR"),
		entry("retrying", "WARN"),
		entry("connection ok", "INFO"),
	];

	it("returns every entry with its original index when no filters are active", () => {
		const items = computeFilteredLogItems(logs, noSearch);

		expect(items).toHaveLength(4);
		expect(items.map((item) => item.originalIndex)).toEqual([0, 1, 2, 3]);
		expect(items.map((item) => item.entry)).toEqual(logs);
	});

	it("keeps only entries whose level is selected, preserving original indices", () => {
		const items = computeFilteredLogItems(logs, {
			...noSearch,
			selectedLevels: new Set<LogLevel>(["ERROR", "WARN"]),
		});

		expect(items.map((item) => item.originalIndex)).toEqual([1, 2]);
		expect(items.map((item) => item.entry.message)).toEqual([
			"connection failed",
			"retrying",
		]);
	});

	it("excludes plain-text matches case-insensitively when excludeMatches is on", () => {
		const items = computeFilteredLogItems(logs, {
			...noSearch,
			excludeMatches: true,
			searchText: "CONNECTION",
		});

		expect(items.map((item) => item.originalIndex)).toEqual([0, 2]);
	});

	it("excludes regex matches when excludeMatches is on", () => {
		const searchText = "failed|retry";
		const items = computeFilteredLogItems(logs, {
			...noSearch,
			excludeMatches: true,
			searchText,
			useRegex: true,
			searchParsed: parseSearch(searchText, true),
		});

		expect(items.map((item) => item.originalIndex)).toEqual([0, 3]);
	});

	it("skips exclusion filtering when the regex is invalid", () => {
		const searchText = "(unclosed";
		const items = computeFilteredLogItems(logs, {
			...noSearch,
			excludeMatches: true,
			searchText,
			useRegex: true,
			searchParsed: parseSearch(searchText, true),
		});

		expect(items).toHaveLength(4);
	});

	it("falls back to the raw line when the message is empty", () => {
		const rawOnly: LogEntry = { level: "INFO", raw: "raw noise" };
		const items = computeFilteredLogItems([rawOnly, entry("keep me")], {
			...noSearch,
			excludeMatches: true,
			searchText: "noise",
		});

		expect(items.map((item) => item.entry)).toEqual([entry("keep me")]);
	});

	it("combines level and exclusion filters with correct index mapping", () => {
		const items = computeFilteredLogItems(logs, {
			...noSearch,
			selectedLevels: new Set<LogLevel>(["INFO"]),
			excludeMatches: true,
			searchText: "starting",
		});

		expect(items.map((item) => item.originalIndex)).toEqual([3]);
	});
});
