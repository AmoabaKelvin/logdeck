"use client";

import { useEffect } from "react";

// The landing page has no dark variant, so drop the dark class while it is
// mounted and put it back when the visitor navigates to docs or the demo.
export function ForceLight() {
  useEffect(() => {
    const html = document.documentElement;
    const wasDark = html.classList.contains("dark");
    html.classList.remove("dark");
    return () => {
      if (wasDark) html.classList.add("dark");
    };
  }, []);
  return null;
}
