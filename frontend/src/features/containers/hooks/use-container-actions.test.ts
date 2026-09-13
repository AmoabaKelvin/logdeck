import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";

import type { ContainerInfo } from "../types";
import { useContainerActions } from "./use-container-actions";

// Deferred per-container promises so tests control when each action settles.
const resolvers = new Map<string, (message: string) => void>();
const deferredAction = (id: string) =>
	new Promise<string>((resolve) => {
		resolvers.set(id, resolve);
	});
const actionFns = {
	start: deferredAction,
	stop: deferredAction,
	restart: deferredAction,
	remove: deferredAction,
};

function makeContainer(id: string): ContainerInfo {
	return {
		id,
		names: [],
		image: "",
		image_id: "",
		command: "",
		created: 0,
		state: "",
		status: "",
		host: "local",
	};
}

describe("useContainerActions pending state", () => {
	beforeEach(() => {
		resolvers.clear();
	});

	it("tracks a pending action per container", () => {
		const { result } = renderHook(() =>
			useContainerActions(async () => {}, actionFns),
		);

		act(() => result.current.startContainerAction(makeContainer("a")));

		expect(result.current.pendingActions.get("a")).toBe("start");
		expect(result.current.pendingActions.has("b")).toBe(false);
	});

	it("keeps other containers pending when one action settles", async () => {
		const { result } = renderHook(() =>
			useContainerActions(async () => {}, actionFns),
		);

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
		const { result } = renderHook(() =>
			useContainerActions(async () => {}, actionFns),
		);

		act(() => result.current.startContainerAction(makeContainer("a")));

		await act(async () => {
			resolvers.get("a")?.("");
		});

		expect(result.current.pendingActions.size).toBe(0);
	});
});
