import { describe, expect, it } from "vitest";

import {
	cleanRedactionConfig,
	EMPTY_REDACTION_RULE,
	redactionDraft,
	validateRedactionDraft,
} from "./redaction-utils";

describe("cleanRedactionConfig", () => {
	it("drops empty optional fields so an untouched draft equals the saved config", () => {
		const saved = {
			enabled: true,
			builtins: ["bearer"],
			rules: [{ name: "ids", enabled: true, pattern: "id-\\d+" }],
		};
		const draft = redactionDraft({
			...saved,
			catalog: [],
			defaultReplacement: "[REDACTED]",
		});
		draft.rules = [
			{
				...draft.rules[0],
				replacement: "",
				hosts: [],
				containers: [],
				projects: [],
			},
		];

		expect(cleanRedactionConfig(draft)).toEqual(cleanRedactionConfig(saved));
		expect(cleanRedactionConfig(draft).replacement).toBeUndefined();
	});

	it("trims rule names and keeps targeting that was set", () => {
		const cleaned = cleanRedactionConfig({
			enabled: false,
			builtins: [],
			rules: [
				{
					...EMPTY_REDACTION_RULE,
					name: "  card numbers ",
					pattern: "\\d{16}",
					replacement: "$1***",
					projects: ["shop"],
				},
			],
		});
		expect(cleaned.rules[0]).toEqual({
			name: "card numbers",
			enabled: true,
			pattern: "\\d{16}",
			replacement: "$1***",
			hosts: undefined,
			containers: undefined,
			projects: ["shop"],
		});
	});
});

describe("validateRedactionDraft", () => {
	it("accepts rules that all have a pattern", () => {
		expect(
			validateRedactionDraft({
				enabled: true,
				builtins: [],
				rules: [{ ...EMPTY_REDACTION_RULE, pattern: "x" }],
			}),
		).toBeNull();
	});

	it("names the rule that has no pattern", () => {
		const base = { enabled: true, builtins: [] };
		expect(
			validateRedactionDraft({
				...base,
				rules: [{ ...EMPTY_REDACTION_RULE, name: "tokens" }],
			}),
		).toBe('"tokens" needs a pattern');
		expect(
			validateRedactionDraft({
				...base,
				rules: [
					{ ...EMPTY_REDACTION_RULE, pattern: "a" },
					EMPTY_REDACTION_RULE,
				],
			}),
		).toBe("Custom pattern 2 needs a pattern");
	});
});
