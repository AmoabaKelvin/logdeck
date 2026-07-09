import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { authenticatedFetch } from "./api-client";

const TOKEN_KEY = "logdeck_auth_token";

const fetchMock = vi.fn();
const originalLocation = window.location;

function stubLocation(pathname: string) {
	Object.defineProperty(window, "location", {
		value: { pathname, href: `http://localhost${pathname}` },
		writable: true,
		configurable: true,
	});
}

beforeEach(() => {
	vi.stubGlobal("fetch", fetchMock);
	localStorage.clear();
	stubLocation("/");
});

afterEach(() => {
	vi.unstubAllGlobals();
	fetchMock.mockReset();
	Object.defineProperty(window, "location", {
		value: originalLocation,
		writable: true,
		configurable: true,
	});
});

describe("authenticatedFetch", () => {
	it("attaches the stored token as a Bearer header", async () => {
		localStorage.setItem(TOKEN_KEY, "my-token");
		fetchMock.mockResolvedValue(new Response("ok", { status: 200 }));

		await authenticatedFetch("/api/v1/containers");

		const headers = fetchMock.mock.calls[0][1]?.headers as Headers;
		expect(headers.get("Authorization")).toBe("Bearer my-token");
	});

	it("clears the token and redirects to /login on 401", async () => {
		localStorage.setItem(TOKEN_KEY, "expired-token");
		fetchMock.mockResolvedValue(new Response("unauthorized", { status: 401 }));

		await authenticatedFetch("/api/v1/containers");

		expect(localStorage.getItem(TOKEN_KEY)).toBeNull();
		expect(window.location.href).toBe("/login");
	});

	it("does not redirect when already on /login", async () => {
		stubLocation("/login");
		fetchMock.mockResolvedValue(new Response("unauthorized", { status: 401 }));

		await authenticatedFetch("/api/v1/containers");

		expect(window.location.href).toBe("http://localhost/login");
	});

	it("never issues an empty-credential login probe", async () => {
		fetchMock.mockResolvedValue(new Response("unauthorized", { status: 401 }));

		await authenticatedFetch("/api/v1/containers?since=1h");

		// Only the original request goes out - no follow-up POST probing
		// /auth/login with empty credentials.
		expect(fetchMock).toHaveBeenCalledTimes(1);
		expect(fetchMock.mock.calls[0][0]).toBe("/api/v1/containers?since=1h");
	});

	it("passes non-401 responses through untouched", async () => {
		localStorage.setItem(TOKEN_KEY, "my-token");
		fetchMock.mockResolvedValue(new Response("boom", { status: 500 }));

		const response = await authenticatedFetch("/api/v1/containers");

		expect(response.status).toBe(500);
		expect(localStorage.getItem(TOKEN_KEY)).toBe("my-token");
		expect(window.location.href).toBe("http://localhost/");
	});
});
