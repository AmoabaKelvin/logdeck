import { describe, expect, it } from "vitest";

import {
	validateRemovedDays,
	validateRetentionCaps,
} from "./log-storage-utils";

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

describe("validateRemovedDays", () => {
	it("accepts zero and whole days up to ten years", () => {
		expect(validateRemovedDays("0")).toBeNull();
		expect(validateRemovedDays("30")).toBeNull();
		expect(validateRemovedDays("3650")).toBeNull();
	});

	it("rejects negatives, fractions, blanks and anything past ten years", () => {
		expect(validateRemovedDays("-1")).toMatch(/days/i);
		expect(validateRemovedDays("1.5")).toMatch(/days/i);
		expect(validateRemovedDays("")).toMatch(/days/i);
		expect(validateRemovedDays("3651")).toMatch(/days/i);
	});
});
