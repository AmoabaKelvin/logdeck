import { describe, expect, it } from "vitest";

import { resolveTimeRange } from "./time-range";

describe("resolveTimeRange", () => {
	const now = Date.parse("2026-07-09T12:00:00.000Z");

	it("returns no bounds for the all preset", () => {
		expect(
			resolveTimeRange({ preset: "all", since: null, until: null }, now),
		).toEqual({});
	});

	it("anchors relative presets to the provided now", () => {
		expect(
			resolveTimeRange({ preset: "15m", since: null, until: null }, now),
		).toEqual({ since: "2026-07-09T11:45:00.000Z" });
		expect(
			resolveTimeRange({ preset: "24h", since: null, until: null }, now),
		).toEqual({ since: "2026-07-08T12:00:00.000Z" });
	});

	it("passes custom bounds through as-is", () => {
		expect(
			resolveTimeRange(
				{
					preset: "custom",
					since: "2026-07-01T00:00:00.000Z",
					until: "2026-07-02T00:00:00.000Z",
				},
				now,
			),
		).toEqual({
			since: "2026-07-01T00:00:00.000Z",
			until: "2026-07-02T00:00:00.000Z",
		});
	});

	it("supports open-ended custom ranges", () => {
		expect(
			resolveTimeRange(
				{ preset: "custom", since: "2026-07-01T00:00:00.000Z", until: null },
				now,
			),
		).toEqual({ since: "2026-07-01T00:00:00.000Z" });
		expect(
			resolveTimeRange({ preset: "custom", since: null, until: null }, now),
		).toEqual({});
	});
});
