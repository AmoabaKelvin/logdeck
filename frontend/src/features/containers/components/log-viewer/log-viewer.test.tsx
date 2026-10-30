import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen } from "@testing-library/react";
import type React from "react";
import { useState } from "react";
import { toast } from "sonner";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { LogEntry } from "@/features/containers/api/get-container-logs-parsed";
import type {
	HistoryLogsPage,
	HistoryStatus,
} from "@/features/containers/api/get-history";
import { LogViewer } from "./log-viewer";
import { useLocalLogViewState } from "./use-log-view-state";

// The API modules run for real; only fetch underneath them is faked. Each
// endpoint the viewer reads answers from one of these.
const server = {
	historyStatus: vi.fn<() => HistoryStatus>(),
	// An Error becomes the store's JSON error response.
	historyLogs: vi.fn<(cursor: string | null) => HistoryLogsPage | Error>(),
	historySearch: vi.fn<(params: URLSearchParams) => HistoryLogsPage>(),
	logs: vi.fn<() => LogEntry[]>(),
	aggregate: vi.fn<() => Response>(),
	stream:
		vi.fn<(signal: AbortSignal | undefined) => ReadableStream<Uint8Array>>(),
};

function respond(input: RequestInfo | URL, init?: RequestInit): Response {
	const url = new URL(
		input instanceof Request ? input.url : input,
		"http://localhost",
	);
	if (url.pathname.endsWith("/history/status")) {
		return Response.json(server.historyStatus());
	}
	if (url.pathname.endsWith("/history/logs")) {
		if (!url.searchParams.has("container")) {
			return Response.json(server.historySearch(url.searchParams));
		}
		const page = server.historyLogs(url.searchParams.get("cursor"));
		return page instanceof Error
			? Response.json({ error: page.message }, { status: 500 })
			: Response.json(page);
	}
	if (url.pathname.endsWith("/logs/aggregate")) {
		if (url.searchParams.get("follow") === "true") {
			return new Response(server.stream(init?.signal ?? undefined));
		}
		return server.aggregate();
	}
	if (url.pathname.endsWith("/logs/parsed")) {
		if (url.searchParams.get("follow") === "true") {
			return new Response(server.stream(init?.signal ?? undefined));
		}
		const logs = server.logs();
		return Response.json({ logs, count: logs.length });
	}
	return new Response("not found", { status: 404 });
}

// Persistence off by default: the viewer never leaves the live path and the
// source toggle stays hidden. The history suite overrides these.
beforeEach(() => {
	server.historyStatus.mockReset().mockReturnValue({ enabled: false });
	server.historyLogs.mockReset().mockReturnValue({ logs: [], count: 0 });
	server.historySearch.mockReset().mockReturnValue({ logs: [], count: 0 });
	server.logs.mockReset().mockReturnValue([]);
	server.aggregate.mockReset();
	server.stream.mockReset();
	vi.stubGlobal(
		"fetch",
		vi.fn<typeof fetch>(async (input, init) => respond(input, init)),
	);
	vi.spyOn(toast, "error").mockReturnValue("");
	vi.spyOn(toast, "success").mockReturnValue("");
	vi.spyOn(toast, "info").mockReturnValue("");
});

afterEach(() => {
	vi.unstubAllGlobals();
	vi.restoreAllMocks();
});

const entry = (id: number): LogEntry => ({
	level: "INFO",
	message: `line ${id}`,
	timestamp: new Date(1700000000000 + id * 1000).toISOString(),
});

// A push-controlled NDJSON body standing in for the live log stream.
function createControlledStream() {
	const encoder = new TextEncoder();
	let controller: ReadableStreamDefaultController<Uint8Array> | undefined;
	const stream = new ReadableStream<Uint8Array>({
		start(streamController) {
			controller = streamController;
		},
	});

	// Log entries, or any other line the server writes (source events).
	const push = (...entries: object[]) => {
		for (const next of entries) {
			controller?.enqueue(encoder.encode(`${JSON.stringify(next)}\n`));
		}
	};

	const end = () => {
		controller?.close();
	};

	return { push, end, stream };
}

async function drainMicrotasks(iterations = 100) {
	for (let i = 0; i < iterations; i++) {
		await Promise.resolve();
	}
}

function Harness(
	props: Pick<
		React.ComponentProps<typeof LogViewer>,
		"history" | "historyOnly" | "targets"
	>,
) {
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
				history={{ container: "container-1" }}
				{...props}
			/>
		</QueryClientProvider>
	);
}

describe("LogViewer streaming lifecycle", () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it("keeps a started stream alive across the isStreaming state transition", async () => {
		const controlled = createControlledStream();
		let streamSignal: AbortSignal | undefined;
		server.stream.mockImplementation((signal) => {
			streamSignal = signal;
			return controlled.stream;
		});

		await act(async () => {
			render(<Harness />);
			await drainMicrotasks();
		});
		expect(server.logs).toHaveBeenCalledTimes(1);

		await act(async () => {
			fireEvent.click(screen.getByRole("button", { name: "Stream" }));
			await drainMicrotasks();
		});

		// The regression: the fetch/teardown effect re-ran on the isStreaming
		// transition and its cleanup aborted the stream that had just started,
		// silently degrading Stream into a one-shot fetch.
		expect(streamSignal?.aborted).toBe(false);
		expect(screen.getByRole("button", { name: "Stop" })).toBeTruthy();
		expect(server.logs).toHaveBeenCalledTimes(1);

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
		server.stream.mockImplementation((signal) => {
			streamSignal = signal;
			return controlled.stream;
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

describe("LogViewer live field filter", () => {
	it("hides live lines whose fields do not match", async () => {
		server.logs.mockReturnValue([
			{ level: "ERROR", message: '{"user":{"id":42}}' },
			{ level: "ERROR", message: '{"user":{"id":7}}' },
			{ level: "INFO", message: "user.id=42 logged in" },
		]);
		await act(async () => {
			render(<Harness />);
			await drainMicrotasks();
		});

		await act(async () => {
			fireEvent.change(screen.getByRole("searchbox", { name: "Search logs" }), {
				target: { value: "user.id=42" },
			});
		});

		expect(screen.getByText("2 of 3")).toBeTruthy();
		// Field terms filter; they are not text to step through.
		expect(screen.queryByText("No matches")).toBeNull();
	});
});

describe("LogViewer shortcut help overlay", () => {
	beforeEach(() => {
		vi.useFakeTimers();
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
			fireEvent.keyDown(
				screen.getByRole("searchbox", { name: "Search logs" }),
				{
					key: "?",
					shiftKey: true,
				},
			);
		});
		expect(screen.queryByText("Keyboard shortcuts")).toBeNull();

		await act(async () => {
			fireEvent.keyDown(window, { key: "?", shiftKey: true });
		});
		expect(screen.getByText("Keyboard shortcuts")).toBeTruthy();

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
		server.historyStatus.mockReturnValue({ enabled: true });
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
		server.historyLogs.mockReturnValue(
			storedPage(["stored one", "stored two"]),
		);

		await switchToHistory();

		expect(server.historyLogs).toHaveBeenCalledTimes(1);
		expect(screen.getByText("stored one")).toBeTruthy();
		expect(screen.getByText("stored two")).toBeTruthy();
		// Nothing older to fetch, so the list says so instead of offering a button.
		expect(screen.getByText("Beginning of stored history")).toBeTruthy();
	});

	it("appends an older page when Load older is clicked", async () => {
		server.historyLogs.mockImplementation((cursor) =>
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
		server.historyLogs.mockImplementation((cursor) =>
			cursor
				? new Error("store unavailable")
				: storedPage(["newer one"], "cursor-1"),
		);

		await switchToHistory();
		expect(toast.error).not.toHaveBeenCalled();

		await act(async () => {
			fireEvent.click(screen.getByRole("button", { name: "Load older" }));
		});
		await settleQueries();

		// The loaded page is still on screen, so the empty state cannot explain
		// the failure: without the toast the spinner would just vanish.
		expect(screen.getByText("newer one")).toBeTruthy();
		expect(toast.error).toHaveBeenCalledTimes(1);
		expect(toast.error).toHaveBeenCalledWith(
			"Failed to load stored logs: store unavailable",
		);
	});

	it("explains an empty store through the empty state", async () => {
		server.historyLogs.mockReturnValue(storedPage([]));

		await switchToHistory();

		expect(screen.getByText("No stored logs match these filters")).toBeTruthy();
		expect(toast.error).not.toHaveBeenCalled();
	});

	async function renderSearch(props: React.ComponentProps<typeof Harness>) {
		await act(async () => {
			render(<Harness {...props} />);
			await drainMicrotasks();
		});
		await settleQueries();
		await settleQueries();
	}

	it("searches every container with a badge per row", async () => {
		server.historySearch.mockReturnValue({
			logs: [
				{ level: "ERROR", message: "connection refused", containerName: "api" },
				{ level: "ERROR", message: "upstream down", containerName: "proxy" },
			],
			count: 2,
		});

		await renderSearch({ history: {}, historyOnly: true });

		expect(server.historyLogs).not.toHaveBeenCalled();
		expect(server.historySearch).toHaveBeenCalledTimes(1);
		expect(server.historySearch.mock.calls[0][0].has("project")).toBe(false);
		expect(screen.getByText("connection refused")).toBeTruthy();
		expect(screen.getByText("api")).toBeTruthy();
		expect(screen.getByText("proxy")).toBeTruthy();
	});

	it("scopes stack history to the project and strips its prefix", async () => {
		server.historySearch.mockReturnValue({
			logs: [{ level: "INFO", message: "ready", containerName: "shop-api-1" }],
			count: 1,
		});

		await renderSearch({ history: { project: "shop" }, historyOnly: true });

		expect(server.historySearch.mock.calls[0][0].get("project")).toBe("shop");
		expect(screen.getByText("api-1")).toBeTruthy();
	});

	it("waits for a click after a partial scan comes back empty", async () => {
		server.historySearch.mockReturnValue({
			logs: [],
			count: 0,
			nextCursor: "cursor-1",
			scannedTo: "2026-09-23T02:40:00Z",
		});

		await renderSearch({ history: {}, historyOnly: true });
		await settleQueries();

		expect(server.historySearch).toHaveBeenCalledTimes(1);
		expect(screen.getByText(/^No matches yet, searched back to /)).toBeTruthy();
		expect(screen.getByText(/^Searched back to /)).toBeTruthy();

		await act(async () => {
			fireEvent.click(screen.getByRole("button", { name: "Search older" }));
		});
		await settleQueries();

		expect(server.historySearch).toHaveBeenCalledTimes(2);
		expect(server.historySearch.mock.calls[1][0].get("cursor")).toBe(
			"cursor-1",
		);
	});

	it("filters by a clicked JSON value on the server", async () => {
		server.historySearch.mockReturnValue({
			logs: [
				{
					level: "ERROR",
					message: '{"msg":"checkout failed","user":{"id":42},"tags":["a"]}',
					containerName: "api",
				},
			],
			count: 1,
		});

		await renderSearch({ history: {}, historyOnly: true });
		await act(async () => {
			fireEvent.click(screen.getByRole("button", { name: "Expand JSON" }));
		});
		// Array items have no dot path, so only scalar object values are offered.
		expect(screen.queryByRole("button", { name: '"a"' })).toBeNull();
		await act(async () => {
			fireEvent.click(screen.getByRole("button", { name: "42" }));
		});

		const search = screen.getByRole("searchbox", { name: "Search logs" });
		expect(search).toHaveProperty("value", "user.id=42");
		await act(async () => {
			fireEvent.change(search, {
				target: { value: 'user.id=42 msg="checkout failed" timeout' },
			});
			vi.advanceTimersByTime(300);
		});
		await settleQueries();

		const params = server.historySearch.mock.calls.at(-1)?.[0];
		expect(params?.get("where")).toBe('user.id=42 msg="checkout failed"');
		expect(params?.get("search")).toBe("timeout");
	});
});

describe("LogViewer fullscreen", () => {
	it("toggles from the toolbar and exits on Escape", async () => {
		render(
			<>
				<button type="button">behind</button>
				<Harness />
			</>,
		);
		const behind = screen.getByRole("button", { name: "behind" });
		await act(async () => {
			await drainMicrotasks();
		});

		fireEvent.click(screen.getByRole("button", { name: "Fullscreen" }));
		expect(
			screen.getByRole("button", { name: "Exit fullscreen" }),
		).toBeTruthy();
		expect(behind.hasAttribute("inert")).toBe(true);

		// From the search field, the first Escape only leaves the field.
		const search = screen.getByLabelText("Search logs");
		search.focus();
		fireEvent.keyDown(search, { key: "Escape" });
		expect(document.activeElement).not.toBe(search);
		expect(
			screen.getByRole("button", { name: "Exit fullscreen" }),
		).toBeTruthy();

		fireEvent.keyDown(window, { key: "Escape" });
		expect(screen.getByRole("button", { name: "Fullscreen" })).toBeTruthy();
		expect(behind.hasAttribute("inert")).toBe(false);

		fireEvent.keyDown(window, { key: "f" });
		expect(
			screen.getByRole("button", { name: "Exit fullscreen" }),
		).toBeTruthy();
	});
});

describe("LogViewer combined coverage", () => {
	const targets = [
		{ id: "web1", host: "prod", name: "web" },
		{ id: "api1", host: "prod", name: "api" },
	];
	const source = (containerId: string, status: string, error?: string) => ({
		host: "prod",
		containerId,
		containerName: containerId === "web1" ? "web" : "api",
		status,
		error,
	});
	const banner = () => screen.queryByText(/Incomplete: \d+ sources? missing/);

	it("lists a source that could not be read until a retry reads it", async () => {
		server.aggregate
			.mockImplementationOnce(() =>
				Response.json({
					logs: [entry(1)],
					count: 1,
					incomplete: true,
					sources: [
						source("web1", "ok"),
						source("api1", "failed", "daemon exploded"),
					],
				}),
			)
			.mockImplementation(() =>
				Response.json({
					logs: [entry(1)],
					count: 1,
					incomplete: false,
					sources: [source("web1", "ok"), source("api1", "no_matches")],
				}),
			);

		await act(async () => {
			render(<Harness targets={targets} />);
			await drainMicrotasks();
		});
		expect(banner()?.textContent).toBe("Incomplete: 1 source missing");
		expect(screen.getByText("api on prod: daemon exploded")).toBeTruthy();

		await act(async () => {
			fireEvent.click(screen.getByRole("button", { name: "Retry" }));
			await drainMicrotasks();
		});
		expect(server.aggregate).toHaveBeenCalledTimes(2);
		expect(banner()).toBeNull();
	});

	it("names every source when none could be read", async () => {
		server.aggregate.mockImplementation(() =>
			Response.json(
				{
					error: "no source could be read: web on prod: x; api on prod: y",
					sources: [
						source("web1", "failed", "x"),
						source("api1", "failed", "y"),
					],
				},
				{ status: 502 },
			),
		);

		await act(async () => {
			render(<Harness targets={targets} />);
			await drainMicrotasks();
		});
		expect(banner()?.textContent).toBe("Incomplete: 2 sources missing");
		expect(toast.error).toHaveBeenCalledWith(
			"Failed to fetch logs: no source could be read: web on prod: x; api on prod: y",
		);
	});

	it("keeps a stream failure visible while healthy sources keep logging", async () => {
		vi.useFakeTimers();
		server.aggregate.mockImplementation(() =>
			Response.json({ logs: [], count: 0, sources: [] }),
		);
		const controlled = createControlledStream();
		server.stream.mockReturnValue(controlled.stream);

		await act(async () => {
			render(<Harness targets={targets} />);
			await drainMicrotasks();
		});
		await act(async () => {
			fireEvent.click(screen.getByRole("button", { name: "Stream" }));
			await drainMicrotasks();
		});

		await act(async () => {
			controlled.push(
				{ type: "source", ...source("web1", "ok") },
				{ type: "source", ...source("api1", "failed", "stream broke") },
			);
			for (let i = 1; i <= 5; i++) {
				controlled.push({ ...entry(i), containerId: "web1" });
			}
			await drainMicrotasks();
		});
		await act(async () => {
			vi.advanceTimersByTime(100);
		});
		expect(banner()?.textContent).toBe("Incomplete: 1 source missing");
		expect(screen.getByText("api on prod: stream broke")).toBeTruthy();

		await act(async () => {
			controlled.push({ type: "source", ...source("api1", "ok") });
			await drainMicrotasks();
		});
		expect(banner()).toBeNull();

		await act(async () => {
			controlled.end();
			await drainMicrotasks();
		});
		vi.useRealTimers();
	});
});

describe("LogViewer text size", () => {
	it("steps with the keyboard, clamps at the ends and persists", async () => {
		localStorage.clear();
		render(<Harness />);
		await act(async () => {
			await drainMicrotasks();
		});

		fireEvent.keyDown(window, { key: "+" });
		expect(localStorage.getItem("logdeck.logFontSize")).toBe("13");

		for (let i = 0; i < 10; i++) fireEvent.keyDown(window, { key: "-" });
		expect(localStorage.getItem("logdeck.logFontSize")).toBe("11");
	});
});
