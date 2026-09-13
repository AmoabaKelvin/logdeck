import type { ReactNode } from "react";

// Diagram primitives for the engineering posts: flat neutral canvas,
// monospace uppercase labels, gray boxes with an inset dashed border, one
// accent element per figure, dashed orthogonal wires with open chevrons.

export type Tone = "gray" | "accent" | "ghost" | "outline";

export const mono = "font-mono";
export const T = {
  title: `${mono} text-[13px] font-semibold`,
  sub: `${mono} text-[10.5px]`,
  note: `${mono} text-[11px]`,
};

export function Fig({
  caption,
  children,
  minWidth = 600,
}: {
  caption: string;
  children: ReactNode;
  minWidth?: number;
}) {
  return (
    <figure className="not-prose my-8 rounded-xl bg-base-100 p-5 sm:p-8">
      <div className="overflow-x-auto">
        <div style={{ minWidth }}>{children}</div>
      </div>
      <figcaption className="mt-4 text-pretty text-sm text-base-500">
        {caption}
      </figcaption>
    </figure>
  );
}

// Defs renders the chevron markers every figure uses.
export function Defs() {
  return (
    <defs>
      <marker
        id="chev"
        viewBox="0 0 10 10"
        refX="8"
        refY="5"
        markerWidth="8"
        markerHeight="8"
        orient="auto-start-reverse"
      >
        <path
          d="M2 1 L7 5 L2 9"
          className="fill-none stroke-base-400"
          strokeWidth="1.2"
        />
      </marker>
      <marker
        id="chev-accent"
        viewBox="0 0 10 10"
        refX="8"
        refY="5"
        markerWidth="8"
        markerHeight="8"
        orient="auto-start-reverse"
      >
        <path
          d="M2 1 L7 5 L2 9"
          className="fill-none stroke-accent-500"
          strokeWidth="1.2"
        />
      </marker>
    </defs>
  );
}

const box = {
  gray: {
    outer: "fill-base-200 stroke-base-400",
    inner: "stroke-base-400",
    title: "fill-base-900",
    sub: "fill-base-500",
  },
  accent: {
    outer: "fill-accent-50 stroke-accent-500",
    inner: "stroke-accent-500",
    title: "fill-accent-600",
    sub: "fill-accent-500",
  },
  ghost: {
    outer: "fill-white stroke-base-300",
    inner: "",
    title: "fill-base-400",
    sub: "fill-base-400",
  },
  outline: {
    outer: "fill-white stroke-accent-500",
    inner: "",
    title: "fill-accent-600",
    sub: "fill-accent-500",
  },
} as const;

export function Box({
  x,
  y,
  w,
  h,
  title,
  sub,
  tone = "gray",
  inset = true,
  children,
}: {
  x: number;
  y: number;
  w: number;
  h: number;
  title?: string;
  sub?: string | string[];
  tone?: Tone;
  inset?: boolean;
  children?: ReactNode;
}) {
  const c = box[tone];
  const empty: string[] = [];
  const subs = empty.concat(sub ?? []);
  const lines = (title ? 1 : 0) + subs.length;
  const lineH = 15;
  const top = y + h / 2 - ((lines - 1) * lineH) / 2;
  return (
    <g>
      <rect
        x={x}
        y={y}
        width={w}
        height={h}
        className={c.outer}
        strokeWidth="1.2"
        strokeDasharray={
          tone === "ghost" || tone === "outline" ? "4 3" : undefined
        }
      />
      {inset && tone !== "ghost" && tone !== "outline" && (
        <rect
          x={x + 7}
          y={y + 7}
          width={w - 14}
          height={h - 14}
          className={`fill-none ${c.inner}`}
          strokeWidth="1"
          strokeDasharray="4 3"
        />
      )}
      {title && (
        <text
          x={x + w / 2}
          y={top + 4}
          textAnchor="middle"
          className={`${T.title} ${c.title}`}
          style={{ letterSpacing: "0.08em" }}
        >
          {title}
        </text>
      )}
      {subs.map((s, i) => (
        <text
          key={s}
          x={x + w / 2}
          y={top + 4 + (i + (title ? 1 : 0)) * lineH}
          textAnchor="middle"
          className={`${T.sub} ${c.sub}`}
          style={{ letterSpacing: "0.1em" }}
        >
          {s}
        </text>
      ))}
      {children}
    </g>
  );
}

export function Wire({
  d,
  tone = "gray",
  arrow = true,
}: {
  d: string;
  tone?: "gray" | "accent";
  arrow?: boolean;
}) {
  return (
    <path
      d={d}
      className={`fill-none ${tone === "accent" ? "stroke-accent-500" : "stroke-base-400"}`}
      strokeWidth="1.2"
      strokeDasharray="4 4"
      markerEnd={
        arrow
          ? `url(#${tone === "accent" ? "chev-accent" : "chev"})`
          : undefined
      }
    />
  );
}

// Label is a small caption: uppercase heading with an optional accent
// eyebrow, or a lowercase note.
export function Label({
  x,
  y,
  text,
  tone = "gray",
  anchor = "start",
  upper = true,
  strong = false,
}: {
  x: number;
  y: number;
  text: string;
  tone?: "gray" | "accent" | "dark";
  anchor?: "start" | "middle" | "end";
  upper?: boolean;
  strong?: boolean;
}) {
  const fill =
    tone === "accent"
      ? "fill-accent-600"
      : tone === "dark"
        ? "fill-base-900"
        : "fill-base-500";
  return (
    <text
      x={x}
      y={y}
      textAnchor={anchor}
      className={`${strong ? T.title : upper ? T.sub : T.note} ${fill}`}
      style={{ letterSpacing: upper ? "0.1em" : "0.02em" }}
    >
      {upper ? text.toUpperCase() : text}
    </text>
  );
}
