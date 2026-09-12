import {
  type IconProps,
  Loader2Icon,
} from "@/components/logdeck-demo/ui/icons";

import { cn } from "@/lib/utils";

function Spinner({ className, ...props }: IconProps) {
  return (
    <Loader2Icon
      aria-label="Loading"
      className={cn("size-4 animate-spin", className)}
      {...props}
    />
  );
}

export { Spinner };
