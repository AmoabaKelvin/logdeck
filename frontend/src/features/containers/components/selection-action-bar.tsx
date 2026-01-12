import { CheckIcon, CopyIcon, XIcon } from "lucide-react";
import { useCallback, useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface SelectionActionBarProps {
  selectedCount: number;
  onCopy: () => void;
  onClear: () => void;
}

export function SelectionActionBar({
  selectedCount,
  onCopy,
  onClear,
}: SelectionActionBarProps) {
  const [copied, setCopied] = useState(false);
  const [isVisible, setIsVisible] = useState(false);

  // Animate in/out based on selection
  useEffect(() => {
    if (selectedCount > 0) {
      // Small delay for enter animation
      const timer = setTimeout(() => setIsVisible(true), 10);
      return () => clearTimeout(timer);
    } else {
      setIsVisible(false);
    }
  }, [selectedCount]);

  const handleCopy = useCallback(() => {
    onCopy();
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }, [onCopy]);

  if (selectedCount === 0) return null;

  return (
    <div
      className={cn(
        "sticky top-0 z-20 flex items-center justify-between gap-4 px-4 py-2",
        "bg-gradient-to-r from-primary/[0.08] via-primary/[0.05] to-transparent",
        "border-b border-primary/20 backdrop-blur-sm",
        "transition-all duration-300 ease-out",
        isVisible
          ? "opacity-100 translate-y-0"
          : "opacity-0 -translate-y-2"
      )}
    >
      {/* Selection indicator with animated dot */}
      <div className="flex items-center gap-3">
        <div className="relative flex items-center justify-center">
          <span className="absolute size-2 rounded-full bg-primary/60 animate-ping" />
          <span className="relative size-2 rounded-full bg-primary" />
        </div>
        <span className="text-sm font-medium text-foreground/90 tabular-nums">
          <span className="text-primary font-semibold">{selectedCount}</span>
          {" "}
          {selectedCount === 1 ? "line" : "lines"} selected
        </span>
      </div>

      {/* Action buttons */}
      <div className="flex items-center gap-2">
        <Button
          size="sm"
          onClick={handleCopy}
          className={cn(
            "h-7 px-3 text-xs font-medium gap-1.5",
            "transition-all duration-200",
            "shadow-sm hover:shadow",
            copied && "bg-emerald-600 hover:bg-emerald-600"
          )}
        >
          {copied ? (
            <>
              <CheckIcon className="size-3.5" />
              Copied!
            </>
          ) : (
            <>
              <CopyIcon className="size-3.5" />
              Copy
            </>
          )}
        </Button>
        <Button
          size="sm"
          variant="ghost"
          onClick={onClear}
          className={cn(
            "h-7 w-7 p-0",
            "text-muted-foreground hover:text-foreground",
            "transition-colors duration-150"
          )}
          aria-label="Clear selection"
        >
          <XIcon className="size-3.5" />
        </Button>
      </div>
    </div>
  );
}
