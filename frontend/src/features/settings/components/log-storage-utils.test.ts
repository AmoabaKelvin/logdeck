import { describe, expect, it } from "vitest";

import { validateRetentionCaps } from "./log-storage-utils";

describe("validateRetentionCaps", () => {
	it("accepts whole caps where the per-container cap fits inside the total", () => {
		expect(validateRetentionCaps("50", "1024")).toBeNull();
		expect(validateRetentionCaps("1", "1")).toBeNull();
	});

	it("rejects a per-container cap below 1 MB", () => {
		expect(validateRetentionCaps("0", "1024")).toMatch(/per-container/i);
		expect(validateRetentionCaps("-5", "1024")).toMatch(/per-container/i);
	});

	it("rejects a total cap below 1 MB", () => {
		expect(validateRetentionCaps("50", "0")).toMatch(/total/i);
	});

	it("rejects empty and non-numeric caps", () => {
		expect(validateRetentionCaps("", "1024")).toMatch(/per-container/i);
		expect(validateRetentionCaps("50", "")).toMatch(/total/i);
		expect(validateRetentionCaps("abc", "1024")).toMatch(/per-container/i);
		expect(validateRetentionCaps("50", "10.5")).toMatch(/total/i);
	});

	it("rejects a per-container cap larger than the total cap", () => {
		expect(validateRetentionCaps("2048", "1024")).toMatch(/exceed/i);
	});

	// The server accepts at most 1048576 MB (1 TiB) per field.
	it("rejects caps above the server's upper bound", () => {
		expect(validateRetentionCaps("1048576", "1048576")).toBeNull();
		expect(validateRetentionCaps("1048577", "1048577")).toMatch(
			/per-container/i,
		);
		expect(validateRetentionCaps("50", "1048577")).toMatch(/total/i);
	});
});
