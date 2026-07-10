import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ContainerInfo } from "../types";
import { useContainerActions } from "./use-container-actions";

// Deferred per-container promises so tests control when each action settles.
const { resolvers, deferredAction } = vi.hoisted(() => {
	const resolvers = new Map<string, (message: string) => void>();
	const deferredAction = (id: string) =>
		new Promise<string>((resolve) => {
			resolvers.set(id, resolve);
		});
	return { resolvers, deferredAction };
});

vi.mock("../api/container-actions", () => ({
	startContainer: (id: string) => deferredAction(id),
	stopContainer: (id: string) => deferredAction(id),
	restartContainer: (id: string) => deferredAction(id),
	removeContainer: (id: string) => deferredAction(id),
}));

vi.mock("../api/compose-actions", () => ({
	performComposeAction: () => Promise.resolve({ succeeded: 0 }),
}));

function makeContainer(id: string): ContainerInfo {
	return { id, host: "local" } as ContainerInfo;
}

describe("useContainerActions pending state", () => {
	beforeEach(() => {
		resolvers.clear();
	});

	it("tracks a pending action per container", () => {
		const { result } = renderHook(() => useContainerActions(async () => {}));

		act(() => result.current.startContainerAction(makeContainer("a")));

		expect(result.current.pendingActions.get("a")).toBe("start");
		expect(result.current.pendingActions.has("b")).toBe(false);
	});

	it("keeps other containers pending when one action settles", async () => {
		const { result } = renderHook(() => useContainerActions(async () => {}));

		act(() => result.current.startContainerAction(makeContainer("a")));
		act(() => result.current.restartContainerAction(makeContainer("b")));

		expect(result.current.pendingActions.get("a")).toBe("start");
		expect(result.current.pendingActions.get("b")).toBe("restart");

		await act(async () => {
			resolvers.get("b")?.("");
		});

		expect(result.current.pendingActions.get("a")).toBe("start");
		expect(result.current.pendingActions.has("b")).toBe(false);
	});

	it("clears a container once its action settles", async () => {
		const { result } = renderHook(() => useContainerActions(async () => {}));

		act(() => result.current.startContainerAction(makeContainer("a")));

		await act(async () => {
			resolvers.get("a")?.("");
		});

		expect(result.current.pendingActions.size).toBe(0);
	});
});
