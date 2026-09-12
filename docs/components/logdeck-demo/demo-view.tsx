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

// Where Radix portals (sheets, dialogs, menus) mount. Default: document.body.
// The landing-page hero sets its own frame so overlays stay inside the demo.
export const PortalContainerContext = createContext<HTMLElement | null>(null);

export function usePortalContainer(): HTMLElement | undefined {
  return useContext(PortalContainerContext) ?? undefined;
}

// The demo owns its theme: a `dark` class on its root, never the site-wide one.
export interface DemoThemeValue {
  theme: string;
  setTheme: (theme: string) => void;
}
export const DemoThemeContext = createContext<DemoThemeValue>({
  theme: "light",
  setTheme: () => {},
});
