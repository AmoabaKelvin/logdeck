import { HugeiconsIcon } from "@hugeicons/react";
import type { ComponentProps } from "react";

import { cn } from "@/lib/utils";

// The dashed "blueprint" frame that runs down the whole landing page.
export function Wrapper({
  className,
  children,
}: {
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <div
      className={cn(
        "mx-auto w-full max-w-5xl border-x border-dashed border-base-200 px-4 2xl:max-w-6xl 2xl:px-12",
        className,
      )}
    >
      {children}
    </div>
  );
}

const pillBase =
  "inline-flex items-center justify-center rounded-full text-sm font-medium h-9 px-4 duration-300 focus:outline-none focus:ring-2 focus:ring-offset-2";

export const pill = {
  accent: `${pillBase} bg-accent-500 text-white hover:bg-accent-600 focus:ring-accent-600`,
  muted: `${pillBase} bg-sand-100 text-sand-950 hover:text-accent-500 focus:ring-sand-100`,
  black: `${pillBase} bg-black text-white hover:bg-base-700 focus:ring-black`,
};

export const h1Class =
  "font-display text-2xl font-medium tracking-tight text-black md:text-3xl lg:text-4xl";
export const h2Class =
  "font-display text-lg font-medium tracking-tight text-black sm:text-xl md:text-2xl lg:text-3xl";

// Hugeicons stroke-rounded set. Pass `className` for colour and `size` for the box.
export function Icon({
  icon,
  className,
  size = 24,
}: {
  icon: ComponentProps<typeof HugeiconsIcon>["icon"];
  className?: string;
  size?: number;
}) {
  return (
    <HugeiconsIcon
      icon={icon}
      size={size}
      strokeWidth={1.5}
      className={className}
    />
  );
}
