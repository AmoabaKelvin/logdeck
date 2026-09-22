import type React from "react";

import { Badge } from "@/components/ui/badge";
import { Label } from "@/components/ui/label";

/**
 * One settings group: a heading with its explanation, an optional control
 * beside the heading (a switch, an env badge), and the body below. Sections
 * sit directly on the page and are separated by a rule, the way the container
 * detail panels are.
 */
export function SettingsSection({
	title,
	description,
	badge,
	action,
	children,
}: {
	title: string;
	description: React.ReactNode;
	badge?: React.ReactNode;
	action?: React.ReactNode;
	children?: React.ReactNode;
}) {
	return (
		<section className="py-8 first:pt-0 last:pb-0">
			<div className="flex items-start justify-between gap-6">
				<div className="min-w-0">
					<div className="flex flex-wrap items-center gap-x-2.5 gap-y-1">
						<h2 className="text-lg font-medium tracking-tight sm:text-base">
							{title}
						</h2>
						{badge}
					</div>
					<p className="mt-1 max-w-prose text-pretty text-base/6 text-muted-foreground sm:text-sm/6">
						{description}
					</p>
				</div>
				{action && <div className="shrink-0 pt-0.5">{action}</div>}
			</div>
			{children && <div className="mt-5">{children}</div>}
		</section>
	);
}

/** A subheading inside a section, with room for one action on the right. */
export function SettingsSubsection({
	title,
	action,
	children,
}: {
	title: string;
	action?: React.ReactNode;
	children: React.ReactNode;
}) {
	return (
		<div>
			<div className="flex min-h-8 items-center justify-between gap-3">
				<h3 className="text-base font-medium sm:text-sm">{title}</h3>
				{action}
			</div>
			<div className="mt-2">{children}</div>
		</div>
	);
}

export function EnvBadge() {
	return (
		<Badge className="border-transparent bg-muted font-normal text-muted-foreground">
			Set via environment
		</Badge>
	);
}

/** Recessed panel for an inline form that is not yet part of the list. */
export function Well({ children }: { children: React.ReactNode }) {
	return <div className="rounded-lg bg-muted/40 p-4">{children}</div>;
}

export function Field({
	id,
	label,
	hint,
	children,
}: {
	id: string;
	label: React.ReactNode;
	hint?: React.ReactNode;
	children: React.ReactNode;
}) {
	return (
		<div className="grid gap-1.5">
			<Label htmlFor={id} className="text-base sm:text-sm">
				{label}
			</Label>
			{children}
			{hint && (
				<p className="text-base/6 text-muted-foreground sm:text-sm/6">{hint}</p>
			)}
		</div>
	);
}

export function Note({ children }: { children: React.ReactNode }) {
	return (
		<p className="text-base text-muted-foreground sm:text-sm">{children}</p>
	);
}

export function ErrorNote({ children }: { children: React.ReactNode }) {
	return <p className="text-base text-destructive sm:text-sm">{children}</p>;
}

/** Outcome of a connection or delivery test, beside the row it belongs to. */
export function Outcome({
	ok,
	children,
	title,
}: {
	ok: boolean;
	children: React.ReactNode;
	title?: string;
}) {
	return (
		<span
			title={title}
			className={`inline-flex items-center gap-1.5 ${
				ok
					? "text-emerald-600 dark:text-emerald-400"
					: "text-rose-600 dark:text-rose-400"
			}`}
		>
			<span
				className={`size-1.5 shrink-0 rounded-full ${ok ? "bg-emerald-500" : "bg-rose-500"}`}
				aria-hidden="true"
			/>
			{children}
		</span>
	);
}

/**
 * Tables sit on the page with row rules only, and bleed to the screen edge on
 * small screens so wide rows scroll instead of squeezing.
 */
export function SettingsTable({ children }: { children: React.ReactNode }) {
	return (
		<div className="-mx-4 overflow-x-auto sm:-mx-6 lg:mx-0">
			<div className="inline-block min-w-full px-4 align-middle sm:px-6 lg:px-0">
				<table className="w-full text-sm">{children}</table>
			</div>
		</div>
	);
}

export const thClass =
	"h-10 px-3 text-left align-middle font-medium whitespace-nowrap text-muted-foreground first:pl-0 last:pr-0";
export const tdClass =
	"h-12 px-3 align-middle whitespace-nowrap first:pl-0 last:pr-0";

export function Th({
	className = "",
	children,
}: {
	className?: string;
	children?: React.ReactNode;
}) {
	return <th className={`${thClass} ${className}`}>{children}</th>;
}

export function Td({
	className = "",
	children,
	title,
}: {
	className?: string;
	children?: React.ReactNode;
	title?: string;
}) {
	return (
		<td className={`${tdClass} ${className}`} title={title}>
			{children}
		</td>
	);
}

export function THead({ children }: { children: React.ReactNode }) {
	return (
		<thead>
			<tr className="border-b border-border/70">{children}</tr>
		</thead>
	);
}

export function TBody({ children }: { children: React.ReactNode }) {
	return <tbody className="divide-y divide-border/60">{children}</tbody>;
}
