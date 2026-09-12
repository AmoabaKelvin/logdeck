import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { useEnvDraft } from "./use-env-draft";

const ORIGINAL = { PORT: "8080", LOG_LEVEL: "info" };

function draft(original: Record<string, string> = ORIGINAL) {
	return renderHook(() => useEnvDraft(original));
}

describe("useEnvDraft", () => {
	it("starts clean and sends back what it was given", () => {
		const { result } = draft();
		expect(result.current.isDirty).toBe(false);
		expect(result.current.counts).toEqual({ added: 0, changed: 0, removed: 0 });
		expect(result.current.payload()).toEqual(ORIGINAL);
	});

	it("counts an edit as changed and sends the new value", () => {
		const { result } = draft();
		act(() => result.current.setValue("PORT", "9090"));

		expect(result.current.counts).toMatchObject({ changed: 1, added: 0 });
		expect(result.current.isChanged("PORT")).toBe(true);
		expect(result.current.payload()).toEqual({ ...ORIGINAL, PORT: "9090" });
	});

	it("keeps a removed variable visible but drops it from the payload", () => {
		const { result } = draft();
		act(() => result.current.remove("PORT"));

		expect(result.current.isRemoved("PORT")).toBe(true);
		// Still on screen, struck through, so the removal can be undone.
		expect(result.current.effective).toHaveProperty("PORT");
		expect(result.current.counts).toMatchObject({ removed: 1 });
		expect(result.current.payload()).toEqual({ LOG_LEVEL: "info" });
	});

	it("restores a removed variable", () => {
		const { result } = draft();
		act(() => result.current.remove("PORT"));
		act(() => result.current.restore("PORT"));

		expect(result.current.isDirty).toBe(false);
		expect(result.current.payload()).toEqual(ORIGINAL);
	});

	// The bug this model replaced: removing a variable you had just added
	// counted as a removal, so the panel reported a pending change and offered
	// to recreate the container over nothing at all.
	it("treats adding then removing a variable as no change", () => {
		const { result } = draft();
		act(() => result.current.add({ APP_MODE: "debug" }));
		expect(result.current.counts).toMatchObject({ added: 1, removed: 0 });

		act(() => result.current.remove("APP_MODE"));

		expect(result.current.counts).toEqual({ added: 0, changed: 0, removed: 0 });
		expect(result.current.isDirty).toBe(false);
		expect(result.current.effective).not.toHaveProperty("APP_MODE");
		expect(result.current.payload()).toEqual(ORIGINAL);
	});

	it("un-removes a variable that an import brings back", () => {
		const { result } = draft();
		act(() => result.current.remove("PORT"));
		act(() => result.current.add({ PORT: "3000" }));

		expect(result.current.isRemoved("PORT")).toBe(false);
		expect(result.current.counts).toMatchObject({ changed: 1, removed: 0 });
		expect(result.current.payload()).toEqual({ ...ORIGINAL, PORT: "3000" });
	});

	it("marks imported keys as recently added", () => {
		const { result } = draft();
		act(() => result.current.add({ A: "1", B: "2" }));

		expect([...result.current.recentlyAdded].sort()).toEqual(["A", "B"]);
	});

	it("discards every pending change", () => {
		const { result } = draft();
		act(() => {
			result.current.setValue("PORT", "9090");
			result.current.add({ APP_MODE: "debug" });
			result.current.remove("LOG_LEVEL");
		});
		act(() => result.current.discard());

		expect(result.current.isDirty).toBe(false);
		expect(result.current.payload()).toEqual(ORIGINAL);
	});

	it("holds edits made before the original arrives", () => {
		const { result, rerender } = renderHook(
			({ original }: { original: Record<string, string> | undefined }) =>
				useEnvDraft(original),
			{
				initialProps: {
					original: undefined as Record<string, string> | undefined,
				},
			},
		);

		act(() => result.current.add({ APP_MODE: "debug" }));
		rerender({ original: ORIGINAL });

		expect(result.current.payload()).toEqual({
			...ORIGINAL,
			APP_MODE: "debug",
		});
	});
});
