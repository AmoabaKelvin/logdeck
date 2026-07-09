import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
	AuthConfigUnavailableError,
	isAuthEnabled,
	resetAuthConfigCache,
} from "./auth-config";

const fetchMock = vi.fn();

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { "Content-Type": "application/json" },
	});
}

beforeEach(() => {
	vi.stubGlobal("fetch", fetchMock);
	resetAuthConfigCache();
});

afterEach(() => {
	vi.unstubAllGlobals();
	fetchMock.mockReset();
});

describe("isAuthEnabled", () => {
	it("returns true when the server reports auth enabled", async () => {
		fetchMock.mockResolvedValue(jsonResponse(200, { authEnabled: true }));
		await expect(isAuthEnabled()).resolves.toBe(true);
	});

	it("returns false when the server reports auth disabled", async () => {
		fetchMock.mockResolvedValue(jsonResponse(200, { authEnabled: false }));
		await expect(isAuthEnabled()).resolves.toBe(false);
	});

	it("caches the result and only fetches once", async () => {
		fetchMock.mockResolvedValue(jsonResponse(200, { authEnabled: false }));
		await isAuthEnabled();
		await isAuthEnabled();
		expect(fetchMock).toHaveBeenCalledTimes(1);
	});

	it("treats a 404 from an older server as auth enabled (fail closed)", async () => {
		fetchMock.mockResolvedValue(jsonResponse(404, "not found"));
		await expect(isAuthEnabled()).resolves.toBe(true);
	});

	it("throws AuthConfigUnavailableError when the server is unreachable", async () => {
		fetchMock.mockRejectedValue(new TypeError("Failed to fetch"));
		await expect(isAuthEnabled()).rejects.toBeInstanceOf(
			AuthConfigUnavailableError,
		);
	});

	it("does not cache failures, so the next call retries", async () => {
		fetchMock
			.mockRejectedValueOnce(new TypeError("Failed to fetch"))
			.mockResolvedValueOnce(jsonResponse(200, { authEnabled: true }));

		await expect(isAuthEnabled()).rejects.toBeInstanceOf(
			AuthConfigUnavailableError,
		);
		await expect(isAuthEnabled()).resolves.toBe(true);
		expect(fetchMock).toHaveBeenCalledTimes(2);
	});
});
