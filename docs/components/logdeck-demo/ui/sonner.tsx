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
      theme={theme as ToasterProps["theme"]}
      className="toaster group"
      style={
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
