import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import { CheckIcon, CopyIcon } from "@/components/ui/icons";

interface CopyButtonProps {
  value: string;
  label: string;
  className?: string;
}

/** Icon button that copies `value` and confirms with a tick for a moment. */
export function CopyButton({ value, label, className }: CopyButtonProps) {
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) return;
    const timer = setTimeout(() => setCopied(false), 1500);
    return () => clearTimeout(timer);
  }, [copied]);

  return (
    <Button
      variant="ghost"
      size="icon-sm"
      aria-label={copied ? `${label} copied` : label}
      onClick={async () => {
        try {
          await navigator.clipboard.writeText(value);
          setCopied(true);
        } catch {
          // A denied clipboard permission is not worth a toast here; the
          // value is on screen and selectable either way.
        }
      }}
      className={className}
    >
      {copied ? (
        <CheckIcon className="size-4 text-emerald-600 dark:text-emerald-400" />
      ) : (
        <CopyIcon className="size-4 text-muted-foreground" />
      )}
    </Button>
  );
}
