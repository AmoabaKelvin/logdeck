import { isRedirect } from "@tanstack/react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
	AuthConfigUnavailableError,
	resetAuthConfigCache,
} from "./auth-config";
import { requireAuthIfEnabled } from "./auth-guard";

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
	localStorage.clear();
});

afterEach(() => {
	vi.unstubAllGlobals();
	fetchMock.mockReset();
});

describe("requireAuthIfEnabled", () => {
	it("allows access without any network call when a token is stored", async () => {
		localStorage.setItem("logdeck_auth_token", "some-token");
		await expect(requireAuthIfEnabled()).resolves.toBeUndefined();
		expect(fetchMock).not.toHaveBeenCalled();
	});

	it("allows access when auth is disabled", async () => {
		fetchMock.mockResolvedValue(jsonResponse(200, { authEnabled: false }));
		await expect(requireAuthIfEnabled()).resolves.toBeUndefined();
	});

	it("redirects to /login when auth is enabled and no token is stored", async () => {
		fetchMock.mockResolvedValue(jsonResponse(200, { authEnabled: true }));
		const error = await requireAuthIfEnabled().catch((e) => e);
		expect(isRedirect(error)).toBe(true);
	});

	it("fails closed when the auth status cannot be determined", async () => {
		fetchMock.mockRejectedValue(new TypeError("Failed to fetch"));
		const error = await requireAuthIfEnabled().catch((e) => e);
		expect(error).toBeInstanceOf(AuthConfigUnavailableError);
		expect(isRedirect(error)).toBe(false);
	});

	it("uses the cached auth status on repeat navigations", async () => {
		fetchMock.mockResolvedValue(jsonResponse(200, { authEnabled: false }));
		await requireAuthIfEnabled();
		await requireAuthIfEnabled();
		expect(fetchMock).toHaveBeenCalledTimes(1);
	});
});
