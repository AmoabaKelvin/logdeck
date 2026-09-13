"use client";

import type * as React from "react";
import { useContext } from "react";
import { Toaster as Sonner } from "sonner";

import type { ToasterProps } from "sonner";

import { DemoThemeContext } from "@/components/logdeck-demo/demo-view";

const Toaster = ({ ...props }: ToasterProps) => {
  const { theme } = useContext(DemoThemeContext);

  return (
    <Sonner
      theme={theme === "dark" || theme === "system" ? theme : "light"}
      className="toaster group"
      style={
        // SAFETY: every key is a CSS custom property (`--*`), which React
        // applies via style.setProperty; CSSProperties just doesn't list them.
        {
          "--normal-bg": "var(--popover)",
          "--normal-text": "var(--popover-foreground)",
          "--normal-border": "var(--border)",
        } as React.CSSProperties
      }
      {...props}
    />
  );
};

export { Toaster };
