import { describe, expect, it } from "vitest";

import { parseEnvFile } from "./environment-variables";

describe("parseEnvFile", () => {
	it("parses multiple KEY=value lines", () => {
		expect(parseEnvFile("A=1\nB=two\nC=three")).toEqual({
			A: "1",
			B: "two",
			C: "three",
		});
	});

	it("skips comments and blank lines", () => {
		expect(parseEnvFile("# comment\n\nA=1\n  \n# another\nB=2")).toEqual({
			A: "1",
			B: "2",
		});
	});

	it("strips surrounding quotes from values", () => {
		expect(parseEnvFile("A=\"quoted\"\nB='single'")).toEqual({
			A: "quoted",
			B: "single",
		});
	});

	it("keeps equals signs inside values", () => {
		expect(parseEnvFile("URL=postgres://u:p@h/db?sslmode=require")).toEqual({
			URL: "postgres://u:p@h/db?sslmode=require",
		});
	});

	it("ignores lines without an equals sign", () => {
		expect(parseEnvFile("NOTAVAR\nA=1")).toEqual({ A: "1" });
	});
});
