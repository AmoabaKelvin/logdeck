import { describe, expect, it } from "vitest";

import { isSecretKey, parseEnvFile } from "./env-file";

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

describe("isSecretKey", () => {
	it("masks the names that usually hold credentials", () => {
		for (const key of [
			"DATABASE_PASSWORD",
			"API_TOKEN",
			"JWT_SECRET",
			"STRIPE_SECRET_KEY",
			"AWS_SECRET_ACCESS_KEY",
			"SESSION_SALT",
			"DATABASE_DSN",
		]) {
			expect(isSecretKey(key), key).toBe(true);
		}
	});

	it("leaves ordinary configuration visible", () => {
		for (const key of [
			"PORT",
			"NODE_ENV",
			"LOG_LEVEL",
			"KEYCLOAK_URL",
			"PATH",
		]) {
			expect(isSecretKey(key), key).toBe(false);
		}
	});
});
