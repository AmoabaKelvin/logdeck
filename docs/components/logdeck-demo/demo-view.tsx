"use client";

import { createContext, useContext } from "react";

// The real app routes between these pages with TanStack Router; the demo is a
// single docs page, so navigation is plain state shared through context.
export type DemoView = "containers" | "images" | "volumes" | "networks";

interface DemoViewContextValue {
	view: DemoView;
	setView: (view: DemoView) => void;
}

export const DemoViewContext = createContext<DemoViewContextValue>({
	view: "containers",
	setView: () => {},
});

export function useDemoView(): DemoViewContextValue {
	return useContext(DemoViewContext);
}
