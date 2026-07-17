import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { LogEntry } from "@/features/containers/api/get-container-logs-parsed";

import { downloadLogs, formatLogEntryLine } from "./log-export";

describe("formatLogEntryLine", () => {
	it("formats timestamp, level, and message", () => {
		const entry: LogEntry = {
			timestamp: "2026-07-15T12:34:56.000Z",
			level: "INFO",
			message: "hello world",
		};
		expect(formatLogEntryLine(entry)).toBe(
			"[2026-07-15T12:34:56.000Z] [INFO] hello world",
		);
	});

	it("normalizes the timestamp to ISO 8601", () => {
		const entry: LogEntry = {
			timestamp: "2026-07-15T12:34:56+02:00",
			level: "ERROR",
			message: "boom",
		};
		expect(formatLogEntryLine(entry)).toBe(
			"[2026-07-15T10:34:56.000Z] [ERROR] boom",
		);
	});

	it("uses an empty timestamp field when none is present", () => {
		const entry: LogEntry = { level: "DEBUG", message: "no ts" };
		expect(formatLogEntryLine(entry)).toBe("[] [DEBUG] no ts");
	});

	it("falls back to UNKNOWN when the level is empty", () => {
		const entry = { level: "", message: "x" } as unknown as LogEntry;
		expect(formatLogEntryLine(entry)).toBe("[] [UNKNOWN] x");
	});

	it("falls back to raw, then empty, when message is missing", () => {
		expect(formatLogEntryLine({ level: "INFO", raw: "raw text" })).toBe(
			"[] [INFO] raw text",
		);
		expect(formatLogEntryLine({ level: "INFO" })).toBe("[] [INFO] ");
	});
});

// jsdom's Blob does not implement text(), so capture the constructor parts.
class FakeBlob {
	parts: BlobPart[];
	type: string;
	constructor(parts: BlobPart[], options?: BlobPropertyBag) {
		this.parts = parts;
		this.type = options?.type ?? "";
	}
	get content(): string {
		return this.parts.join("");
	}
}

describe("downloadLogs", () => {
	let capturedBlob: FakeBlob | null;
	let anchor: HTMLAnchorElement;
	let clickSpy: ReturnType<typeof vi.fn>;

	beforeEach(() => {
		vi.useFakeTimers();
		vi.setSystemTime(new Date("2026-07-15T12:34:56.789Z"));

		capturedBlob = null;
		clickSpy = vi.fn();

		vi.stubGlobal("Blob", FakeBlob);
		vi.stubGlobal("URL", {
			createObjectURL: vi.fn((blob: FakeBlob) => {
				capturedBlob = blob;
				return "blob:mock";
			}),
			revokeObjectURL: vi.fn(),
		});

		anchor = document.createElement("a");
		anchor.click = clickSpy;
		vi.spyOn(document, "createElement").mockReturnValue(anchor);
	});

	afterEach(() => {
		vi.useRealTimers();
		vi.unstubAllGlobals();
		vi.restoreAllMocks();
	});

	const entries: LogEntry[] = [
		{ timestamp: "2026-07-15T00:00:00.000Z", level: "INFO", message: "a" },
		{ level: "ERROR", message: "b" },
	];

	it("builds a timestamped filename with the requested extension", () => {
		downloadLogs(entries, "web", "txt");
		expect(anchor.download).toBe("web-logs-2026-07-15T12-34-56.txt");
		expect(clickSpy).toHaveBeenCalledOnce();
	});

	it("strips a leading slash and sanitizes unsafe filename characters", () => {
		downloadLogs(entries, "/my/app:v1", "json");
		expect(anchor.download).toBe("my-app-v1-logs-2026-07-15T12-34-56.json");
	});

	it("defaults the container name to 'container' when undefined", () => {
		downloadLogs(entries, undefined, "txt");
		expect(anchor.download).toBe("container-logs-2026-07-15T12-34-56.txt");
	});

	it("serializes JSON exports as a pretty-printed array", () => {
		downloadLogs(entries, "web", "json");
		expect(capturedBlob?.type).toBe("application/json");
		expect(capturedBlob?.content).toBe(JSON.stringify(entries, null, 2));
	});

	it("serializes text exports as newline-joined formatted lines", () => {
		downloadLogs(entries, "web", "txt");
		expect(capturedBlob?.type).toBe("text/plain");
		expect(capturedBlob?.content).toBe(
			"[2026-07-15T00:00:00.000Z] [INFO] a\n[] [ERROR] b",
		);
	});
});
