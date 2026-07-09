import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useContainerLogStream } from "./use-container-log-stream";

interface TestEntry {
	id: number;
	message: string;
}

const entry = (id: number): TestEntry => ({ id, message: `line ${id}` });

// A push-controlled async generator standing in for the NDJSON stream.
function createControlledStream() {
	const queue: TestEntry[] = [];
	let notify: (() => void) | null = null;
	let ended = false;

	const push = (...entries: TestEntry[]) => {
		queue.push(...entries);
		notify?.();
		notify = null;
	};

	const end = () => {
		ended = true;
		notify?.();
		notify = null;
	};

	async function* stream(): AsyncGenerator<TestEntry, void, unknown> {
		while (true) {
			while (queue.length > 0) {
				const next = queue.shift();
				if (next) yield next;
			}
			if (ended) return;
			await new Promise<void>((resolve) => {
				notify = resolve;
			});
		}
	}

	return { push, end, stream };
}

// Let the streaming loop consume queued entries. Consumption advances via
// microtask chains only (no timers), so awaiting repeatedly is enough.
async function drainMicrotasks(iterations = 500) {
	for (let i = 0; i < iterations; i++) {
		await Promise.resolve();
	}
}

function setup(options?: { maxLogLines?: number }) {
	const controlled = createControlledStream();
	const scrollToBottom = vi.fn();
	const renderCount = { current: 0 };

	const hook = renderHook(() => {
		renderCount.current += 1;
		return useContainerLogStream<TestEntry>({
			containerId: "container-1",
			host: "host-1",
			tail: 100,
			maxLogLines: options?.maxLogLines,
			getLogs: vi.fn().mockResolvedValue([]),
			streamLogs: () => controlled.stream(),
			scrollToBottom,
		});
	});

	return { ...hook, controlled, scrollToBottom, renderCount };
}

describe("useContainerLogStream", () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it("batches streamed entries into interval flushes, preserving order", async () => {
		const { result, controlled, scrollToBottom, renderCount } = setup();

		await act(async () => {
			void result.current.startStreaming();
			controlled.push(entry(1), entry(2), entry(3));
			await drainMicrotasks();
		});

		// Entries have been consumed but not flushed into state yet
		expect(result.current.logs).toHaveLength(0);

		const rendersBeforeFlush = renderCount.current;
		await act(async () => {
			vi.advanceTimersByTime(100);
		});

		expect(result.current.logs.map((e) => e.id)).toEqual([1, 2, 3]);
		// One flush means one batched state update, not one render per line
		expect(renderCount.current - rendersBeforeFlush).toBeLessThanOrEqual(2);

		// One scheduled scroll for the whole flush
		await act(async () => {
			vi.advanceTimersByTime(100);
		});
		expect(scrollToBottom).toHaveBeenCalledTimes(1);

		await act(async () => {
			controlled.push(entry(4), entry(5));
			await drainMicrotasks();
		});
		expect(result.current.logs).toHaveLength(3);

		await act(async () => {
			vi.advanceTimersByTime(200);
		});
		expect(result.current.logs.map((e) => e.id)).toEqual([1, 2, 3, 4, 5]);
		expect(scrollToBottom).toHaveBeenCalledTimes(2);
	});

	it("caps the log buffer, dropping the oldest entries and reporting the count", async () => {
		const { result, controlled } = setup({ maxLogLines: 5 });

		expect(result.current.maxLogLines).toBe(5);

		await act(async () => {
			void result.current.startStreaming();
			controlled.push(...[1, 2, 3, 4, 5, 6, 7, 8].map(entry));
			await drainMicrotasks();
		});
		await act(async () => {
			vi.advanceTimersByTime(100);
		});

		expect(result.current.logs.map((e) => e.id)).toEqual([4, 5, 6, 7, 8]);
		expect(result.current.droppedCount).toBe(3);

		await act(async () => {
			controlled.push(entry(9), entry(10));
			await drainMicrotasks();
		});
		await act(async () => {
			vi.advanceTimersByTime(100);
		});

		expect(result.current.logs.map((e) => e.id)).toEqual([6, 7, 8, 9, 10]);
		expect(result.current.droppedCount).toBe(5);
	});

	it("buffers while paused and lands buffered lines after resume", async () => {
		const { result, controlled } = setup();

		await act(async () => {
			void result.current.startStreaming();
			controlled.push(entry(1), entry(2));
			await drainMicrotasks();
		});
		await act(async () => {
			vi.advanceTimersByTime(100);
		});
		expect(result.current.logs).toHaveLength(2);

		act(() => {
			result.current.togglePauseStreaming();
		});
		expect(result.current.isStreamPaused).toBe(true);

		await act(async () => {
			controlled.push(entry(3), entry(4), entry(5));
			await drainMicrotasks();
		});
		// Buffered count updates on the flush cadence, not per line
		expect(result.current.bufferedCount).toBe(0);

		await act(async () => {
			vi.advanceTimersByTime(100);
		});
		expect(result.current.bufferedCount).toBe(3);
		expect(result.current.logs).toHaveLength(2);

		act(() => {
			result.current.togglePauseStreaming();
		});
		expect(result.current.isStreamPaused).toBe(false);
		expect(result.current.bufferedCount).toBe(0);
		expect(result.current.logs.map((e) => e.id)).toEqual([1, 2, 3, 4, 5]);
	});
});
