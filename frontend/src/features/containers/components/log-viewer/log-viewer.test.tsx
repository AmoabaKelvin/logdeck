import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { LogEntry } from "@/features/containers/api/get-container-logs-parsed";
import { LogViewer } from "./log-viewer";
import { useLocalLogViewState } from "./use-log-view-state";

// These tests cover the live path; with persistence off the viewer never
// leaves it and the source toggle stays hidden.
vi.mock("@/features/containers/api/get-history", () => ({
	getHistoryStatus: vi.fn().mockResolvedValue({ enabled: false }),
	getHistoryContainers: vi.fn().mockResolvedValue([]),
	getHistoryLogs: vi.fn().mockResolvedValue({ logs: [], count: 0 }),
}));

const mocks = vi.hoisted(() => ({
	getLogs: vi.fn<() => Promise<LogEntry[]>>(),
	streamLogs:
		vi.fn<
			(
				id: string,
				host: string,
				options: unknown,
				signal?: AbortSignal,
			) => AsyncGenerator<LogEntry, void, unknown>
		>(),
}));

vi.mock(
	"@/features/containers/api/get-container-logs-parsed",
	async (importOriginal) => {
		const actual =
			await importOriginal<
				typeof import("@/features/containers/api/get-container-logs-parsed")
			>();
		return {
			...actual,
			getContainerLogsParsed: (...args: Parameters<typeof mocks.getLogs>) =>
				mocks.getLogs(...args),
			streamContainerLogsParsed: (
				...args: Parameters<typeof mocks.streamLogs>
			) => mocks.streamLogs(...args),
		};
	},
);

const entry = (id: number): LogEntry => ({
	level: "INFO",
	message: `line ${id}`,
	timestamp: new Date(1700000000000 + id * 1000).toISOString(),
});

// A push-controlled async generator standing in for the NDJSON stream (same
// shape as the use-container-log-stream tests).
function createControlledStream() {
	const queue: LogEntry[] = [];
	let notify: (() => void) | null = null;
	let ended = false;

	const push = (...entries: LogEntry[]) => {
		queue.push(...entries);
		notify?.();
		notify = null;
	};

	const end = () => {
		ended = true;
		notify?.();
		notify = null;
	};

	async function* stream(): AsyncGenerator<LogEntry, void, unknown> {
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

async function drainMicrotasks(iterations = 100) {
	for (let i = 0; i < iterations; i++) {
		await Promise.resolve();
	}
}

function Harness() {
	const viewState = useLocalLogViewState();
	const [queryClient] = useState(
		() =>
			new QueryClient({
				defaultOptions: { queries: { retry: false } },
			}),
	);
	return (
		<QueryClientProvider client={queryClient}>
			<LogViewer
				variant="page"
				containerId="container-1"
				host="host-1"
				viewState={viewState}
			/>
		</QueryClientProvider>
	);
}

describe("LogViewer streaming lifecycle", () => {
	beforeEach(() => {
		vi.useFakeTimers();
		mocks.getLogs.mockReset().mockResolvedValue([]);
		mocks.streamLogs.mockReset();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it("keeps a started stream alive across the isStreaming state transition", async () => {
		const controlled = createControlledStream();
		let streamSignal: AbortSignal | undefined;
		mocks.streamLogs.mockImplementation((_id, _host, _options, signal) => {
			streamSignal = signal;
			return controlled.stream();
		});

		await act(async () => {
			render(<Harness />);
			await drainMicrotasks();
		});
		expect(mocks.getLogs).toHaveBeenCalledTimes(1);

		await act(async () => {
			fireEvent.click(screen.getByRole("button", { name: "Stream" }));
			await drainMicrotasks();
		});

		// The regression: the fetch/teardown effect re-ran on the isStreaming
		// transition and its cleanup aborted the stream that had just started,
		// silently degrading Stream into a one-shot fetch.
		expect(streamSignal?.aborted).toBe(false);
		expect(screen.getByRole("button", { name: "Stop" })).toBeTruthy();
		expect(mocks.getLogs).toHaveBeenCalledTimes(1);

		// The live stream still lands entries after the transition settles
		// (both empty states disappear once logs.length > 0).
		await act(async () => {
			controlled.push(entry(1), entry(2));
			await drainMicrotasks();
		});
		await act(async () => {
			vi.advanceTimersByTime(100);
		});
		expect(screen.queryByText("No logs available")).toBeNull();
		expect(screen.queryByText("Loading logs...")).toBeNull();

		await act(async () => {
			controlled.end();
			await drainMicrotasks();
		});
	});

	it("stops the stream when the viewer unmounts", async () => {
		const controlled = createControlledStream();
		let streamSignal: AbortSignal | undefined;
		mocks.streamLogs.mockImplementation((_id, _host, _options, signal) => {
			streamSignal = signal;
			return controlled.stream();
		});

		let view: ReturnType<typeof render> | undefined;
		await act(async () => {
			view = render(<Harness />);
			await drainMicrotasks();
		});
		await act(async () => {
			fireEvent.click(screen.getByRole("button", { name: "Stream" }));
			await drainMicrotasks();
		});
		expect(streamSignal?.aborted).toBe(false);

		await act(async () => {
			view?.unmount();
			await drainMicrotasks();
		});
		expect(streamSignal?.aborted).toBe(true);
	});
});

describe("LogViewer shortcut help overlay", () => {
	beforeEach(() => {
		vi.useFakeTimers();
		mocks.getLogs.mockReset().mockResolvedValue([]);
		mocks.streamLogs.mockReset();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it("toggles the overlay with ? and ignores ? typed into inputs", async () => {
		await act(async () => {
			render(<Harness />);
			await drainMicrotasks();
		});
		expect(screen.queryByText("Keyboard shortcuts")).toBeNull();

		// ? typed while focused in an input must not open the overlay.
		await act(async () => {
			fireEvent.keyDown(screen.getByPlaceholderText("Search logs..."), {
				key: "?",
				shiftKey: true,
			});
		});
		expect(screen.queryByText("Keyboard shortcuts")).toBeNull();

		await act(async () => {
			fireEvent.keyDown(window, { key: "?", shiftKey: true });
		});
		expect(screen.getByText("Keyboard shortcuts")).toBeTruthy();

		// Pressing ? again closes it.
		await act(async () => {
			fireEvent.keyDown(window, { key: "?", shiftKey: true });
		});
		expect(screen.queryByText("Keyboard shortcuts")).toBeNull();
	});
});
