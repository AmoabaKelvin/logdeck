import { describe, expect, it } from "vitest";

import { formatJson, isJsonString } from "./json-format";

describe("formatJson", () => {
	it("pretty-prints a JSON object with two-space indentation", () => {
		const result = formatJson('{"a":1,"b":2}');
		expect(result.isJson).toBe(true);
		expect(result.formatted).toBe('{\n  "a": 1,\n  "b": 2\n}');
	});

	it("pretty-prints a JSON array", () => {
		const result = formatJson("[1,2,3]");
		expect(result.isJson).toBe(true);
		expect(result.formatted).toBe("[\n  1,\n  2,\n  3\n]");
	});

	it("treats an empty object and empty array as JSON", () => {
		expect(formatJson("{}")).toEqual({ formatted: "{}", isJson: true });
		expect(formatJson("[]")).toEqual({ formatted: "[]", isJson: true });
	});

	it("ignores leading and trailing whitespace around JSON", () => {
		const result = formatJson('  {"a":1}  ');
		expect(result.isJson).toBe(true);
		expect(result.formatted).toBe('{\n  "a": 1\n}');
	});

	it("excludes bare primitives even though they are valid JSON", () => {
		for (const primitive of ["42", "true", "false", "null", '"hi"']) {
			const result = formatJson(primitive);
			expect(result.isJson).toBe(false);
			expect(result.formatted).toBe(primitive);
		}
	});

	it("returns the original text for a brace-prefixed but invalid string", () => {
		const result = formatJson("{not valid json}");
		expect(result).toEqual({
			formatted: "{not valid json}",
			isJson: false,
		});
	});

	it("does not treat trailing garbage after valid JSON as JSON", () => {
		const result = formatJson('{"a":1} trailing');
		expect(result.isJson).toBe(false);
	});

	it("returns the original text for plain (non-JSON) log lines", () => {
		const line = "2026-07-15 INFO server started";
		expect(formatJson(line)).toEqual({ formatted: line, isJson: false });
	});

	it("handles an empty string", () => {
		expect(formatJson("")).toEqual({ formatted: "", isJson: false });
	});

	it("caches results and returns the identical reference for repeated input", () => {
		const first = formatJson('{"cached":true}');
		const second = formatJson('{"cached":true}');
		expect(second).toBe(first);
	});
});

describe("isJsonString", () => {
	it("is true for objects and arrays, false for primitives and plain text", () => {
		expect(isJsonString('{"a":1}')).toBe(true);
		expect(isJsonString("[1,2]")).toBe(true);
		expect(isJsonString("42")).toBe(false);
		expect(isJsonString("plain log line")).toBe(false);
	});
});
