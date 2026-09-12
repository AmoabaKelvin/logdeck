import { useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { CopyButton } from "@/components/ui/copy-button";
import type {
	ContainerInspect,
	HealthProbe,
} from "../api/get-container-inspect";
import type { ContainerInfo } from "../types";
import {
	formatNanoseconds,
	formatTimestamp,
	isZeroTime,
	PanelField,
	PanelFields,
	PanelNote,
	PanelSection,
} from "./container-panel-ui";

// Compose writes a dozen labels of its own bookkeeping. They are not what
// anyone opens this panel for, so they wait behind a toggle.
const MACHINE_LABEL_PREFIXES = [
	"com.docker.compose.",
	"desktop.docker.io/",
	"org.opencontainers.image.",
];

function isMachineLabel(key: string): boolean {
	return MACHINE_LABEL_PREFIXES.some((prefix) => key.startsWith(prefix));
}

function joinCommand(parts: string[] | null | undefined): string | null {
	if (!parts || parts.length === 0) return null;
	return parts.join(" ");
}

function probeDuration(probe: HealthProbe): string {
	const ms = new Date(probe.End).getTime() - new Date(probe.Start).getTime();
	if (!Number.isFinite(ms) || ms < 0) return "—";
	return ms < 1000 ? `${ms}ms` : `${(ms / 1000).toFixed(1)}s`;
}

/** A band of the panel, ruled off from the one above it. */
function Band({
	children,
	first = false,
}: {
	children: React.ReactNode;
	first?: boolean;
}) {
	return (
		<div className={first ? "" : "mt-6 border-t border-border/70 pt-6"}>
			{children}
		</div>
	);
}

function HealthBand({ inspect }: { inspect: ContainerInspect }) {
	const check = inspect.Config.Healthcheck;
	const health = inspect.State.Health;
	if (!check?.Test || check.Test.length === 0) return null;

	// Docker prefixes the test with how to run it (CMD or CMD-SHELL); the rest
	// is the command itself, which is the part worth reading.
	const [, ...command] = check.Test;
	const probes = (health?.Log ?? []).slice(-5).reverse();
	const failing = (health?.FailingStreak ?? 0) > 0;

	return (
		<Band>
			<PanelSection
				title="Health check"
				badge={
					health && (
						<Badge
							className={`border-transparent ${
								health.Status === "healthy"
									? "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400"
									: health.Status === "starting"
										? "bg-amber-500/10 text-amber-700 dark:text-amber-400"
										: "bg-rose-500/10 text-rose-700 dark:text-rose-400"
							}`}
						>
							{health.Status}
							{failing && ` · ${health.FailingStreak} failing`}
						</Badge>
					)
				}
			>
				<p className="font-mono text-base break-all sm:text-sm">
					{command.join(" ")}
				</p>
				<p className="mt-1 text-base text-muted-foreground sm:text-sm">
					Every {formatNanoseconds(check.Interval)}, timing out after{" "}
					{formatNanoseconds(check.Timeout)}
					{check.Retries
						? `, ${check.Retries} ${check.Retries === 1 ? "retry" : "retries"}`
						: ""}
					.
				</p>

				{probes.length > 0 && (
					<ul className="mt-4 divide-y divide-border/60 font-mono text-xs">
						{probes.map((probe) => (
							<li
								key={probe.Start}
								className="flex items-baseline gap-3 py-1.5 first:pt-0 last:pb-0"
							>
								<span
									className={`w-12 shrink-0 ${
										probe.ExitCode === 0
											? "text-emerald-600 dark:text-emerald-400"
											: "text-rose-600 dark:text-rose-400"
									}`}
								>
									exit {probe.ExitCode}
								</span>
								<span className="w-24 shrink-0 text-muted-foreground tabular-nums">
									{new Date(probe.Start).toLocaleTimeString()}
								</span>
								<span className="w-12 shrink-0 text-right text-muted-foreground tabular-nums">
									{probeDuration(probe)}
								</span>
								<span
									className="min-w-0 flex-1 truncate text-muted-foreground"
									title={probe.Output.trim()}
								>
									{probe.Output.trim()}
								</span>
							</li>
						))}
					</ul>
				)}
			</PanelSection>
		</Band>
	);
}

function LabelsBand({ container }: { container: ContainerInfo }) {
	const [showMachine, setShowMachine] = useState(false);
	const all = Object.entries(container.labels ?? {});
	const own = all.filter(([key]) => !isMachineLabel(key));
	const machine = all.filter(([key]) => isMachineLabel(key));
	const shown = showMachine ? [...own, ...machine] : own;

	return (
		<Band>
			<PanelSection
				title="Labels"
				action={
					machine.length > 0 && (
						<Button
							variant="ghost"
							size="sm"
							onClick={() => setShowMachine((value) => !value)}
							className="-mr-2 h-8 text-muted-foreground hover:text-foreground"
						>
							{showMachine ? "Hide" : "Show"} {machine.length} generated
						</Button>
					)
				}
			>
				{shown.length === 0 ? (
					<PanelNote>
						{machine.length > 0
							? "Only generated labels on this container."
							: "This container has no labels."}
					</PanelNote>
				) : (
					<dl className="grid max-h-72 gap-x-10 gap-y-3 overflow-y-auto text-base sm:grid-cols-2 sm:text-sm lg:grid-cols-3">
						{shown.map(([key, value]) => (
							<div key={key} className="min-w-0">
								<dt className="truncate font-medium" title={key}>
									{key}
								</dt>
								<dd className="break-all font-mono text-muted-foreground">
									{value || <span className="italic">empty</span>}
								</dd>
							</div>
						))}
					</dl>
				)}
			</PanelSection>
		</Band>
	);
}

interface ContainerOverviewPanelProps {
	container: ContainerInfo;
	inspect: ContainerInspect;
}

/**
 * What the container is running and how it has been behaving. Laid out as
 * bands rather than one two-column grid: the health log runs to whatever
 * length it needs, and a grid cell beside it is just a hole.
 */
export function ContainerOverviewPanel({
	container,
	inspect,
}: ContainerOverviewPanelProps) {
	const { State, Config } = inspect;
	const entrypoint = joinCommand(Config.Entrypoint);
	const command = joinCommand(Config.Cmd) ?? container.command;
	const stopped = !State.Running;

	return (
		<div>
			<Band first>
				<div className="grid lg:grid-cols-2">
					{/* Divider padding is one-sided so the rule sits centred between
					    the columns rather than hugging one of them. */}
					<PanelSection title="Runtime" className="lg:pr-10">
						<PanelFields>
							<PanelField term="Container ID" mono>
								<span className="inline-flex items-center gap-1">
									{container.id.slice(0, 12)}
									<CopyButton
										value={container.id}
										label="Copy the full container ID"
										className="-my-1"
									/>
								</span>
							</PanelField>
							<PanelField term="Image" mono>
								{container.image}
							</PanelField>
							{entrypoint && (
								<PanelField term="Entrypoint" mono>
									{entrypoint}
								</PanelField>
							)}
							<PanelField term="Command" mono>
								{command}
							</PanelField>
							{Config.WorkingDir && (
								<PanelField term="Working dir" mono>
									{Config.WorkingDir}
								</PanelField>
							)}
							<PanelField term="User" mono>
								{Config.User || "root"}
							</PanelField>
							{State.Running && State.Pid > 0 && (
								<PanelField term="PID" mono>
									{State.Pid}
								</PanelField>
							)}
						</PanelFields>
					</PanelSection>

					<PanelSection
						title="Lifecycle"
						className="mt-8 lg:mt-0 lg:border-l lg:border-border/70 lg:pl-10"
					>
						<PanelFields>
							<PanelField term="Created">
								{formatTimestamp(inspect.Created)}
							</PanelField>
							<PanelField term="Started">
								{formatTimestamp(State.StartedAt)}
							</PanelField>
							{stopped && !isZeroTime(State.FinishedAt) && (
								<PanelField term="Stopped">
									{formatTimestamp(State.FinishedAt)}
								</PanelField>
							)}
							<PanelField term="Restarts">
								<span
									className={
										inspect.RestartCount > 0
											? "font-medium text-amber-700 tabular-nums dark:text-amber-400"
											: "tabular-nums"
									}
								>
									{inspect.RestartCount}
								</span>
							</PanelField>
							{stopped && (
								<PanelField term="Exit code">
									<span
										className={
											State.ExitCode === 0
												? "tabular-nums"
												: "font-medium text-rose-700 tabular-nums dark:text-rose-400"
										}
									>
										{State.ExitCode}
									</span>
									{State.OOMKilled &&
										" · killed for exceeding its memory limit"}
								</PanelField>
							)}
							{State.Error && (
								<PanelField term="Last error" mono>
									{State.Error}
								</PanelField>
							)}
						</PanelFields>
					</PanelSection>
				</div>
			</Band>

			<HealthBand inspect={inspect} />
			<LabelsBand container={container} />
		</div>
	);
}
