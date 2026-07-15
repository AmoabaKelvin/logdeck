import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { iterateNDJSONStream } from "./ndjson";

const encoder = new TextEncoder();

function streamFromChunks(chunks: Uint8Array[]): ReadableStream<Uint8Array> {
	let index = 0;
	return new ReadableStream({
		pull(controller) {
			if (index < chunks.length) {
				controller.enqueue(chunks[index++]);
			} else {
				controller.close();
			}
		},
	});
}

function streamFromStrings(strings: string[]): ReadableStream<Uint8Array> {
	return streamFromChunks(strings.map((s) => encoder.encode(s)));
}

async function collect<T>(gen: AsyncGenerator<T, void, unknown>): Promise<T[]> {
	const out: T[] = [];
	for await (const value of gen) {
		out.push(value);
	}
	return out;
}

let errorSpy: ReturnType<typeof vi.spyOn>;

beforeEach(() => {
	errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
	errorSpy.mockRestore();
});

describe("iterateNDJSONStream", () => {
	it("parses one object per newline-terminated line", async () => {
		const stream = streamFromStrings(['{"n":1}\n{"n":2}\n{"n":3}\n']);
		const result = await collect<{ n: number }>(iterateNDJSONStream(stream));
		expect(result).toEqual([{ n: 1 }, { n: 2 }, { n: 3 }]);
	});

	it("yields the trailing line that has no terminating newline", async () => {
		const stream = streamFromStrings(['{"n":1}\n{"n":2}']);
		const result = await collect<{ n: number }>(iterateNDJSONStream(stream));
		expect(result).toEqual([{ n: 1 }, { n: 2 }]);
	});

	it("reassembles a JSON object split across two chunks", async () => {
		const stream = streamFromStrings(['{"msg":"hel', 'lo"}\n']);
		const result = await collect<{ msg: string }>(iterateNDJSONStream(stream));
		expect(result).toEqual([{ msg: "hello" }]);
	});

	it("reassembles a line whose newline arrives in a later chunk", async () => {
		const stream = streamFromStrings(['{"a":1}', "\n", '{"b":2}\n']);
		const result = await collect<Record<string, number>>(
			iterateNDJSONStream(stream),
		);
		expect(result).toEqual([{ a: 1 }, { b: 2 }]);
	});

	it("skips blank and whitespace-only lines", async () => {
		const stream = streamFromStrings(['{"n":1}\n\n   \n{"n":2}\n']);
		const result = await collect<{ n: number }>(iterateNDJSONStream(stream));
		expect(result).toEqual([{ n: 1 }, { n: 2 }]);
	});

	it("skips malformed lines, logs them, and keeps parsing", async () => {
		const stream = streamFromStrings(['{"n":1}\nnot json\n{"n":2}\n']);
		const result = await collect<{ n: number }>(iterateNDJSONStream(stream));
		expect(result).toEqual([{ n: 1 }, { n: 2 }]);
		expect(errorSpy).toHaveBeenCalledWith(
			"Failed to parse NDJSON line:",
			"not json",
			expect.any(SyntaxError),
		);
	});

	it("logs when the final unterminated line is malformed", async () => {
		const stream = streamFromStrings(['{"n":1}\n{oops']);
		const result = await collect<{ n: number }>(iterateNDJSONStream(stream));
		expect(result).toEqual([{ n: 1 }]);
		expect(errorSpy).toHaveBeenCalledWith(
			"Failed to parse final NDJSON line:",
			"{oops",
			expect.any(SyntaxError),
		);
	});

	it("decodes multibyte UTF-8 characters split across chunk boundaries", async () => {
		const bytes = encoder.encode('{"m":"café"}\n');
		// Split inside the two-byte é sequence (é is the 4th char, bytes 9-10).
		const split = 9;
		const stream = streamFromChunks([
			bytes.slice(0, split),
			bytes.slice(split),
		]);
		const result = await collect<{ m: string }>(iterateNDJSONStream(stream));
		expect(result).toEqual([{ m: "café" }]);
	});

	it("yields nothing and never reads when the signal is already aborted", async () => {
		const controller = new AbortController();
		controller.abort();
		const stream = streamFromStrings(['{"n":1}\n']);
		const result = await collect<{ n: number }>(
			iterateNDJSONStream(stream, controller.signal),
		);
		expect(result).toEqual([]);
	});

	it("produces no output for an empty stream", async () => {
		const stream = streamFromStrings([]);
		const result = await collect(iterateNDJSONStream(stream));
		expect(result).toEqual([]);
	});
});
