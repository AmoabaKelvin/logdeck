import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { LogStreamHeartbeat } from "@/features/containers/api/get-container-logs-parsed";

import { useContainerLogStream } from "./use-container-log-stream";

interface TestEntry {
	id: number;
	message: string;
}

const entry = (id: number): TestEntry => ({ id, message: `line ${id}` });
const heartbeat = (): LogStreamHeartbeat => ({ type: "heartbeat" });

// A push-controlled async generator standing in for the NDJSON stream. Like a
// real fetch stream, a pending read ends when the signal aborts.
function createControlledStream() {
	const queue: (TestEntry | LogStreamHeartbeat)[] = [];
	let notify: (() => void) | null = null;
	let ended = false;

	const push = (...entries: (TestEntry | LogStreamHeartbeat)[]) => {
		queue.push(...entries);
		notify?.();
		notify = null;
	};

	const end = () => {
		ended = true;
		notify?.();
		notify = null;
	};

	async function* stream(
		signal?: AbortSignal,
	): AsyncGenerator<TestEntry | LogStreamHeartbeat, void, unknown> {
		while (true) {
			while (queue.length > 0) {
				const next = queue.shift();
				if (next) yield next;
			}
			if (ended || signal?.aborted) return;
			await new Promise<void>((resolve) => {
				notify = resolve;
				signal?.addEventListener("abort", () => resolve(), { once: true });
			});
			if (signal?.aborted) return;
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
	const streamLogs = vi.fn(
		(_id: string, _host: string, _options: unknown, signal: AbortSignal) =>
			controlled.stream(signal),
	);

	const hook = renderHook(() => {
		renderCount.current += 1;
		return useContainerLogStream<TestEntry>({
			containerId: "container-1",
			host: "host-1",
			tail: 100,
			maxLogLines: options?.maxLogLines,
			getLogs: vi.fn().mockResolvedValue([]),
			streamLogs,
			scrollToBottom,
		});
	});

	return { ...hook, controlled, scrollToBottom, renderCount, streamLogs };
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
		// The 3 overflow entries never reached the displayed array, so they are
		// reported separately from the pin-shift counter.
		expect(result.current.droppedCount).toBe(0);
		expect(result.current.bufferedDroppedCount).toBe(3);

		await act(async () => {
			controlled.push(entry(9), entry(10));
			await drainMicrotasks();
		});
		await act(async () => {
			vi.advanceTimersByTime(100);
		});

		expect(result.current.logs.map((e) => e.id)).toEqual([6, 7, 8, 9, 10]);
		expect(result.current.droppedCount).toBe(2);
		expect(result.current.bufferedDroppedCount).toBe(3);
	});

	it("does not count paused-backlog trims in the pin-shift counter", async () => {
		const { result, controlled } = setup({ maxLogLines: 5 });

		await act(async () => {
			void result.current.startStreaming();
			controlled.push(entry(1), entry(2));
			await drainMicrotasks();
		});
		await act(async () => {
			vi.advanceTimersByTime(100);
		});
		expect(result.current.logs.map((e) => e.id)).toEqual([1, 2]);
		expect(result.current.droppedCount).toBe(0);

		act(() => {
			result.current.togglePauseStreaming();
		});

		// Push more than maxLogLines while paused: the backlog is trimmed, but
		// the displayed array is untouched, so droppedCount (which consumers
		// use to shift pin indices) must not move.
		await act(async () => {
			controlled.push(...[3, 4, 5, 6, 7, 8, 9, 10].map(entry));
			await drainMicrotasks();
		});
		await act(async () => {
			vi.advanceTimersByTime(100);
		});
		expect(result.current.bufferedCount).toBe(5);
		expect(result.current.bufferedDroppedCount).toBe(3);
		expect(result.current.droppedCount).toBe(0);
		expect(result.current.logs.map((e) => e.id)).toEqual([1, 2]);

		// On resume the trimmed backlog (6..10) replaces the displayed buffer;
		// both displayed entries are dropped from the front, so the pin-shift
		// counter advances by exactly what left the displayed array.
		act(() => {
			result.current.togglePauseStreaming();
		});
		expect(result.current.logs.map((e) => e.id)).toEqual([6, 7, 8, 9, 10]);
		expect(result.current.droppedCount).toBe(2);
		expect(result.current.bufferedDroppedCount).toBe(3);
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

	it("aborts a silent stream after the idle timeout and reconnects with backoff", async () => {
		const { result, controlled, streamLogs } = setup();

		await act(async () => {
			void result.current.startStreaming();
			controlled.push(entry(1));
			await drainMicrotasks();
		});
		await act(async () => {
			vi.advanceTimersByTime(100);
		});
		expect(result.current.logs.map((e) => e.id)).toEqual([1]);
		expect(streamLogs).toHaveBeenCalledTimes(1);

		// Silence past the 45s idle timeout: the watchdog aborts the session
		// and schedules a reconnect.
		await act(async () => {
			vi.advanceTimersByTime(45_100);
			await drainMicrotasks();
		});
		expect(result.current.isReconnecting).toBe(true);
		expect(result.current.isStreaming).toBe(true);
		expect(streamLogs).toHaveBeenCalledTimes(1); // still in the 1s backoff

		await act(async () => {
			vi.advanceTimersByTime(1_000);
			await drainMicrotasks();
		});
		expect(streamLogs).toHaveBeenCalledTimes(2);

		// Data on the new session clears the reconnecting state and appends
		// without wiping what was already displayed.
		await act(async () => {
			controlled.push(entry(2));
			await drainMicrotasks();
		});
		await act(async () => {
			vi.advanceTimersByTime(100);
		});
		expect(result.current.isReconnecting).toBe(false);
		expect(result.current.logs.map((e) => e.id)).toEqual([1, 2]);

		act(() => {
			result.current.stopStreaming();
		});
		expect(result.current.isStreaming).toBe(false);
	});

	it("skips heartbeats as log rows while they reset the idle timer", async () => {
		const { result, controlled, streamLogs } = setup();

		await act(async () => {
			void result.current.startStreaming();
			controlled.push(entry(1));
			await drainMicrotasks();
		});
		await act(async () => {
			vi.advanceTimersByTime(100);
		});
		expect(result.current.logs.map((e) => e.id)).toEqual([1]);

		// 30s of silence, then a heartbeat, then 30s more: 60s total without
		// entries, but the heartbeat keeps the stream alive.
		await act(async () => {
			vi.advanceTimersByTime(30_000);
			controlled.push(heartbeat());
			await drainMicrotasks();
		});
		await act(async () => {
			vi.advanceTimersByTime(30_000);
			await drainMicrotasks();
		});

		expect(streamLogs).toHaveBeenCalledTimes(1); // no reconnect happened
		expect(result.current.isReconnecting).toBe(false);
		expect(result.current.isStreaming).toBe(true);
		// The heartbeat never shows up as a rendered row.
		expect(result.current.logs.map((e) => e.id)).toEqual([1]);
	});
});
