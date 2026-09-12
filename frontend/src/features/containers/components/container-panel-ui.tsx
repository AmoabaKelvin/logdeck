import type React from "react";

import { ChevronDownIcon } from "@/components/ui/icons";
import { Spinner } from "@/components/ui/spinner";

/**
 * The button that opens a detail panel. Quiet until it is the open one, since
 * a row of them sits above content that should keep the eye.
 */
export function DisclosureTrigger({
	label,
	isOpen,
	controls,
	onClick,
	children,
}: {
	label: string;
	isOpen: boolean;
	controls: string;
	onClick: () => void;
	children?: React.ReactNode;
}) {
	return (
		<button
			type="button"
			aria-expanded={isOpen}
			aria-controls={controls}
			onClick={onClick}
			// text-sm on mobile so a row of these fits a 390px screen; the
			// vertical padding carries the touch target instead.
			className={`inline-flex items-center gap-1.5 rounded-md px-2.5 py-2 text-sm whitespace-nowrap sm:py-1.5 ${
				isOpen
					? "bg-muted text-foreground"
					: "text-muted-foreground hover:text-foreground"
			}`}
		>
			{label}
			{children}
			{isOpen && <ChevronDownIcon className="size-4 shrink-0 rotate-180" />}
		</button>
	);
}

/** A titled block inside a detail panel. */
export function PanelSection({
	title,
	// Status about the section itself, which belongs beside the heading — at
	// full width, pinning it to the far right strands it.
	badge,
	action,
	children,
	className,
}: {
	title: string;
	badge?: React.ReactNode;
	action?: React.ReactNode;
	children: React.ReactNode;
	className?: string;
}) {
	return (
		<section className={`min-w-0 ${className ?? ""}`}>
			<div className="flex min-h-8 items-center justify-between gap-3">
				<div className="flex min-w-0 items-center gap-2.5">
					<h3 className="text-base font-medium sm:text-sm">{title}</h3>
					{badge}
				</div>
				{action}
			</div>
			<div className="mt-2">{children}</div>
		</section>
	);
}

export function PanelFields({ children }: { children: React.ReactNode }) {
	return (
		<dl className="divide-y divide-border/60 text-base sm:text-sm">
			{children}
		</dl>
	);
}

export function PanelField({
	term,
	mono = false,
	children,
}: {
	term: string;
	mono?: boolean;
	children: React.ReactNode;
}) {
	return (
		<div className="grid gap-1 py-2.5 first:pt-0 last:pb-0 sm:grid-cols-[8rem_1fr] sm:gap-4">
			<dt className="font-medium">{term}</dt>
			<dd
				className={`min-w-0 break-all text-muted-foreground ${mono ? "font-mono" : ""}`}
			>
				{children}
			</dd>
		</div>
	);
}

export function PanelNote({ children }: { children: React.ReactNode }) {
	return (
		<p className="text-base text-muted-foreground sm:text-sm">{children}</p>
	);
}

export function PanelLoading({ label }: { label: string }) {
	return (
		<div className="flex items-center gap-2 py-2 text-base text-muted-foreground sm:text-sm">
			<Spinner className="size-4" />
			{label}
		</div>
	);
}

export function PanelError({ children }: { children: React.ReactNode }) {
	return (
		<p className="py-2 text-base text-destructive sm:text-sm">{children}</p>
	);
}

/** Docker reports healthcheck timings in nanoseconds. */
export function formatNanoseconds(value: number | undefined): string {
	if (!value) return "—";
	const seconds = value / 1e9;
	if (seconds < 1) return `${Math.round(seconds * 1000)}ms`;
	if (seconds < 60) return `${Number(seconds.toFixed(seconds < 10 ? 1 : 0))}s`;
	return `${Math.round(seconds / 60)}m`;
}

/** Docker uses this zero value for "never happened". */
export function isZeroTime(value: string | undefined): boolean {
	return !value || value.startsWith("0001-01-01");
}

export function formatTimestamp(value: string | undefined): string {
	if (isZeroTime(value)) return "—";
	return new Date(value as string).toLocaleString(undefined, {
		dateStyle: "medium",
		timeStyle: "medium",
	});
}
