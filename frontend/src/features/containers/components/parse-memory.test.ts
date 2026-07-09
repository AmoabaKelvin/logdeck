import { describe, expect, it } from "vitest";

import { formatMemoryBytes, parseMemoryInput } from "./parse-memory";

describe("parseMemoryInput", () => {
	it("treats empty input as unlimited", () => {
		expect(parseMemoryInput("")).toBe(0);
		expect(parseMemoryInput("   ")).toBe(0);
	});

	it("parses plain byte values", () => {
		expect(parseMemoryInput("1073741824")).toBe(1073741824);
		expect(parseMemoryInput("512b")).toBe(512);
	});

	it("parses unit suffixes", () => {
		expect(parseMemoryInput("512k")).toBe(512 * 1024);
		expect(parseMemoryInput("512m")).toBe(512 * 1024 ** 2);
		expect(parseMemoryInput("1g")).toBe(1024 ** 3);
	});

	it("accepts two-letter suffixes, uppercase, and whitespace", () => {
		expect(parseMemoryInput("512MB")).toBe(512 * 1024 ** 2);
		expect(parseMemoryInput("1 GB")).toBe(1024 ** 3);
		expect(parseMemoryInput(" 2kb ")).toBe(2 * 1024);
	});

	it("parses fractional values", () => {
		expect(parseMemoryInput("1.5g")).toBe(1.5 * 1024 ** 3);
		expect(parseMemoryInput("0.5m")).toBe(512 * 1024);
	});

	it("rejects invalid input", () => {
		expect(parseMemoryInput("abc")).toBeNull();
		expect(parseMemoryInput("12x")).toBeNull();
		expect(parseMemoryInput("-512m")).toBeNull();
		expect(parseMemoryInput("m512")).toBeNull();
		expect(parseMemoryInput("1.2.3g")).toBeNull();
	});
});

describe("formatMemoryBytes", () => {
	it("formats 0 and negative values as empty (unlimited)", () => {
		expect(formatMemoryBytes(0)).toBe("");
		expect(formatMemoryBytes(-1)).toBe("");
	});

	it("uses the largest exact unit", () => {
		expect(formatMemoryBytes(1024 ** 3)).toBe("1g");
		expect(formatMemoryBytes(512 * 1024 ** 2)).toBe("512m");
		expect(formatMemoryBytes(2 * 1024)).toBe("2k");
		expect(formatMemoryBytes(1000)).toBe("1000");
	});

	it("round-trips fractional gigabytes", () => {
		expect(formatMemoryBytes(parseMemoryInput("1.5g") ?? 0)).toBe("1536m");
	});
});
