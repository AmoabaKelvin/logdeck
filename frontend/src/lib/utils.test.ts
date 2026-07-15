import { describe, expect, it } from "vitest";

import { cn, escapeRegExp } from "./utils";

describe("escapeRegExp", () => {
	it("escapes all regex metacharacters", () => {
		// biome-ignore lint/suspicious/noTemplateCurlyInString: "${}" here are literal regex metacharacters under test, not a template placeholder
		expect(escapeRegExp(".*+?^${}()|[]\\")).toBe(
			"\\.\\*\\+\\?\\^\\$\\{\\}\\(\\)\\|\\[\\]\\\\",
		);
	});

	it("leaves ordinary characters untouched", () => {
		expect(escapeRegExp("hello world 123")).toBe("hello world 123");
	});

	it("produces a pattern that matches the input literally", () => {
		const input = "a.b(c)*";
		const re = new RegExp(escapeRegExp(input));
		expect(re.test(input)).toBe(true);
		// Without escaping, "." and "*" would let this false-positive match.
		expect(re.test("axbxc")).toBe(false);
	});
});

describe("cn", () => {
	it("merges conflicting tailwind classes, keeping the last one", () => {
		expect(cn("px-2", "px-4")).toBe("px-4");
	});

	it("drops falsy and conditional values", () => {
		expect(cn("a", false, null, undefined, "b")).toBe("a b");
		expect(cn("base", { active: false, muted: true })).toBe("base muted");
	});
});
