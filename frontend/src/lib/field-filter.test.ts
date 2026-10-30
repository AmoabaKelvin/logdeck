import { describe, expect, it } from "vitest";

import {
	formatFieldTerm,
	matchFieldTerms,
	parseLineFields,
	splitFieldTerms,
} from "./field-filter";

function matches(filter: string, message: string): boolean {
	return matchFieldTerms(splitFieldTerms(filter).terms, message, () =>
		parseLineFields(message),
	);
}

describe("splitFieldTerms", () => {
	it("separates terms from free text", () => {
		const { terms, text } = splitFieldTerms(
			'level=error  timeout user.id!=42 msg="slow \\"db\\" query" "connection refused"',
		);
		expect(
			terms.map(({ key, value, negate }) => ({ key, value, negate })),
		).toEqual([
			{ key: "level", value: "error", negate: false },
			{ key: "user.id", value: "42", negate: true },
			{ key: "msg", value: 'slow "db" query', negate: false },
		]);
		expect(terms[2].raw).toBe('msg="slow \\"db\\" query"');
		expect(text).toBe('timeout "connection refused"');
	});

	it("leaves text without terms exactly as typed", () => {
		for (const search of ["  two  spaces ", "GET /api?user=42", "=x", "1a=b"]) {
			expect(splitFieldTerms(search)).toEqual({ terms: [], text: search });
		}
	});
});

describe("matchFieldTerms", () => {
	const jsonLine =
		'{"level":"ERROR","msg":"checkout failed","user_id":42,"ok":false,"err":null,' +
		'"http":{"path":"/checkout","status":500},"log.level":"warn","tags":["a"]}';
	const logfmtLine =
		'time=2026-05-28T05:00:38Z level=error msg="checkout failed" user_id=42 http.path=/checkout';

	// The same cases as TestFieldFilterMatch in server/internal/models.
	it.each([
		["level=error", jsonLine, true],
		["level=error user_id=42", jsonLine, true],
		["user_id=42.0", jsonLine, true],
		["user_id=43", jsonLine, false],
		["http.path=/checkout http.status=500", jsonLine, true],
		["http.status=404", jsonLine, false],
		["log.level=warn", jsonLine, true],
		["ok=false err=null", jsonLine, true],
		['msg="checkout failed"', jsonLine, true],
		["msg=checkout", jsonLine, false],
		["http=x", jsonLine, false],
		["tags=a", jsonLine, false],
		["missing=x", jsonLine, false],
		["level!=info", jsonLine, true],
		["level!=error", jsonLine, false],
		["missing!=x", jsonLine, true],
		["level=ERROR user_id=42", logfmtLine, true],
		['msg="checkout failed"', logfmtLine, true],
		["http.path=/checkout", logfmtLine, true],
		["user_id=42.0", logfmtLine, false],
		["level!=error", logfmtLine, false],
		["err=nil", "request done err=nil took=3ms", true],
		["took=3ms", "request done err=nil took=3ms", true],
		["request=done", "request done err=nil took=3ms", false],
		["level=error", "ERROR something broke", false],
		["level!=error", "ERROR something broke", true],
		["path=/checkout", '{"path":"\\/checkout"}', true],
		['msg="say \\"hi\\""', 'msg="say \\"hi\\"" level=info', true],
		["msg=café", '{"msg":"caf\\u00e9"}', true],
		["a=1", '{"a":1} trailing a=1', true],
	])("%s on %s is %s", (filter, message, want) => {
		expect(matches(filter, message)).toBe(want);
	});

	it("parses only lines the cheap check cannot rule out", () => {
		const { terms } = splitFieldTerms("user=alice");
		let parses = 0;
		const read = (message: string) => () => {
			parses++;
			return parseLineFields(message);
		};
		expect(
			matchFieldTerms(terms, '{"user":"bob"}', read('{"user":"bob"}')),
		).toBe(false);
		expect(matchFieldTerms(terms, '{"id":1}', read('{"id":1}'))).toBe(false);
		expect(parses).toBe(0);
	});
});

describe("formatFieldTerm", () => {
	it("quotes values that would not survive as a bare token", () => {
		expect(formatFieldTerm("user.id", "42")).toBe("user.id=42");
		expect(formatFieldTerm("msg", 'say "hi" now')).toBe(
			'msg="say \\"hi\\" now"',
		);
		expect(formatFieldTerm("msg", "")).toBe('msg=""');
	});

	it("round-trips through splitFieldTerms", () => {
		const term = formatFieldTerm("msg", 'a "b" \\ c');
		expect(matches(term, JSON.stringify({ msg: 'A "b" \\ c' }))).toBe(true);
	});
});
