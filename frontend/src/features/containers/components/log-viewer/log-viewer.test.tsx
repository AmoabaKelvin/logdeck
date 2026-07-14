import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { LogEntry } from "@/features/containers/api/get-container-logs-parsed";
import type {
	HistoryLogsPage,
	HistoryStatus,
} from "@/features/containers/api/get-history";
import { LogViewer } from "./log-viewer";
import { useLocalLogViewState } from "./use-log-view-state";

const historyMocks = vi.hoisted(() => ({
	getHistoryStatus: vi.fn<() => Promise<{ enabled: boolean }>>(),
	getHistoryLogs: vi.fn<(params: { cursor?: string }) => Promise<unknown>>(),
}));

vi.mock("@/features/containers/api/get-history", () => ({
	getHistoryStatus: historyMocks.getHistoryStatus,
	getHistoryContainers: vi.fn().mockResolvedValue([]),
	getHistoryLogs: historyMocks.getHistoryLogs,
}));

const toastMocks = vi.hoisted(() => ({
	error: vi.fn(),
	success: vi.fn(),
	info: vi.fn(),
}));

vi.mock("sonner", () => ({ toast: toastMocks }));

// Persistence off by default: the viewer never leaves the live path and the
// source toggle stays hidden. The history suite overrides these.
beforeEach(() => {
	historyMocks.getHistoryStatus
		.mockReset()
		.mockResolvedValue({ enabled: false } satisfies HistoryStatus);
	historyMocks.getHistoryLogs
		.mockReset()
		.mockResolvedValue({ logs: [], count: 0 } satisfies HistoryLogsPage);
	toastMocks.error.mockReset();
	toastMocks.success.mockReset();
	toastMocks.info.mockReset();
});

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

describe("LogViewer history mode", () => {
	const storedPage = (
		messages: string[],
		nextCursor?: string,
	): HistoryLogsPage => ({
		logs: messages.map((message) => ({
			level: "INFO",
			message,
			timestamp: "2023-11-14T22:13:20.000Z",
		})),
		count: messages.length,
		nextCursor,
	});

	// jsdom reports every element as zero-sized, and the virtualizer renders no
	// rows when its scroll element measures 0px tall. Hand out a row-sized box so
	// the stored entries actually reach the DOM.
	const ROW_HEIGHT = 36;
	const originalOffsetHeight = Object.getOwnPropertyDescriptor(
		HTMLElement.prototype,
		"offsetHeight",
	);
	const originalOffsetWidth = Object.getOwnPropertyDescriptor(
		HTMLElement.prototype,
		"offsetWidth",
	);

	beforeEach(() => {
		vi.useFakeTimers();
		mocks.getLogs.mockReset().mockResolvedValue([]);
		mocks.streamLogs.mockReset();
		historyMocks.getHistoryStatus.mockResolvedValue({ enabled: true });
		Object.defineProperty(HTMLElement.prototype, "offsetHeight", {
			configurable: true,
			get: () => ROW_HEIGHT,
		});
		Object.defineProperty(HTMLElement.prototype, "offsetWidth", {
			configurable: true,
			get: () => 800,
		});
	});

	afterEach(() => {
		vi.useRealTimers();
		if (originalOffsetHeight) {
			Object.defineProperty(
				HTMLElement.prototype,
				"offsetHeight",
				originalOffsetHeight,
			);
		}
		if (originalOffsetWidth) {
			Object.defineProperty(
				HTMLElement.prototype,
				"offsetWidth",
				originalOffsetWidth,
			);
		}
	});

	// React Query notifies its subscribers through setTimeout(0), which the fake
	// timers hold: a resolved page only reaches the component once they advance.
	async function settleQueries() {
		await act(async () => {
			await drainMicrotasks();
			vi.advanceTimersByTime(1);
			await drainMicrotasks();
		});
	}

	// Renders the viewer (live, as always) and flips the source toggle to
	// History, which is the only way into the stored-logs path.
	async function switchToHistory() {
		await act(async () => {
			render(<Harness />);
			await drainMicrotasks();
		});
		await settleQueries();

		// The toggle is only offered once the server reports persistence is on.
		expect(screen.getByRole("button", { name: "Live" })).toBeTruthy();
		const historyButton = screen.getByRole("button", { name: "History" });

		await act(async () => {
			fireEvent.click(historyButton);
		});
		await settleQueries();
	}

	it("shows the source toggle and renders stored entries in history mode", async () => {
		historyMocks.getHistoryLogs.mockResolvedValue(
			storedPage(["stored one", "stored two"]),
		);

		await switchToHistory();

		expect(historyMocks.getHistoryLogs).toHaveBeenCalledTimes(1);
		expect(screen.getByText("stored one")).toBeTruthy();
		expect(screen.getByText("stored two")).toBeTruthy();
		// Nothing older to fetch, so the list says so instead of offering a button.
		expect(screen.getByText("Beginning of stored history")).toBeTruthy();
	});

	it("appends an older page when Load older is clicked", async () => {
		historyMocks.getHistoryLogs.mockImplementation(async ({ cursor }) =>
			cursor
				? storedPage(["older one", "older two"])
				: storedPage(["newer one", "newer two"], "cursor-1"),
		);

		await switchToHistory();
		expect(screen.queryByText("older one")).toBeNull();

		await act(async () => {
			fireEvent.click(screen.getByRole("button", { name: "Load older" }));
		});
		await settleQueries();

		// Older entries land ahead of the ones already loaded.
		expect(screen.getByText("older one")).toBeTruthy();
		expect(screen.getByText("newer one")).toBeTruthy();
		expect(screen.getByText("Beginning of stored history")).toBeTruthy();
	});

	it("toasts when loading an older page fails", async () => {
		historyMocks.getHistoryLogs.mockImplementation(async ({ cursor }) => {
			if (cursor) throw new Error("store unavailable");
			return storedPage(["newer one"], "cursor-1");
		});

		await switchToHistory();
		expect(toastMocks.error).not.toHaveBeenCalled();

		await act(async () => {
			fireEvent.click(screen.getByRole("button", { name: "Load older" }));
		});
		await settleQueries();

		// The loaded page is still on screen, so the empty state cannot explain
		// the failure: without the toast the spinner would just vanish.
		expect(screen.getByText("newer one")).toBeTruthy();
		expect(toastMocks.error).toHaveBeenCalledTimes(1);
		expect(toastMocks.error).toHaveBeenCalledWith(
			"Failed to load stored logs: store unavailable",
		);
	});

	it("explains an empty store through the empty state", async () => {
		historyMocks.getHistoryLogs.mockResolvedValue(storedPage([]));

		await switchToHistory();

		expect(screen.getByText("No stored logs match these filters")).toBeTruthy();
		expect(toastMocks.error).not.toHaveBeenCalled();
	});
});
