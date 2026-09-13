"use client";

import { Copy01Icon, Tick02Icon } from "@hugeicons/core-free-icons";
import copy from "copy-to-clipboard";
import { useState } from "react";

import { Icon } from "@/components/landing/ui";

export function CopyButton({
  text,
  className,
  children,
}: {
  text: string;
  className?: string;
  children?: React.ReactNode;
}) {
  const [copied, setCopied] = useState(false);

  return (
    <button
      type="button"
      className={className}
      onClick={() => {
        copy(text);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
      }}
    >
      <Icon
        icon={copied ? Tick02Icon : Copy01Icon}
        size={16}
        className="shrink-0"
      />
      {children ?? <span className="sr-only">Copy code</span>}
    </button>
  );
}
