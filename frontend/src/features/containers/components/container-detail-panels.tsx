import { useState } from "react";

import { ChevronDownIcon } from "@/components/ui/icons";
import type { ContainerInfo } from "../types";
import { formatCreatedDate, isCoolifyManaged } from "./container-utils";
import { EnvironmentVariables } from "./environment-variables";
import { ResourceLimits } from "./resource-limits";
import { Terminal } from "./terminal";

const PANELS = ["details", "environment", "resources", "terminal"] as const;
type Panel = (typeof PANELS)[number];

const PANEL_LABELS: Record<Panel, string> = {
	details: "Details",
	environment: "Environment",
	resources: "Resources",
	terminal: "Terminal",
};

function Field({
	term,
	children,
}: {
	term: string;
	children: React.ReactNode;
}) {
	return (
		<div className="grid gap-1 py-3 first:pt-0 last:pb-0 sm:grid-cols-[9rem_1fr] sm:gap-4">
			<dt className="font-medium">{term}</dt>
			<dd className="min-w-0 break-all text-muted-foreground">{children}</dd>
		</div>
	);
}

function DetailsPanel({ container }: { container: ContainerInfo }) {
	const labels = Object.entries(container.labels ?? {});
	const ports = container.ports ?? [];

	return (
		<div className="grid gap-8 lg:grid-cols-2">
			<dl className="divide-y divide-border/60 text-base sm:text-sm">
				<Field term="Container ID">
					<span className="font-mono">{container.id}</span>
				</Field>
				<Field term="Image">
					<span className="font-mono">{container.image}</span>
				</Field>
				<Field term="Command">
					<span className="font-mono">{container.command}</span>
				</Field>
				<Field term="Created">{formatCreatedDate(container.created)}</Field>
				<Field term="Host">{container.host}</Field>
				<Field term="Ports">
					{ports.length === 0
						? "None published"
						: ports
								.map(
									(port) =>
										`${port.publicPort} → ${port.privatePort}/${port.type}`,
								)
								.join(", ")}
				</Field>
				{isCoolifyManaged(container.labels) && (
					<Field term="Managed by">Coolify</Field>
				)}
			</dl>

			<div className="min-w-0">
				<h2 className="text-base font-medium sm:text-sm">
					Labels
					{labels.length > 0 && (
						<span className="ml-1.5 text-muted-foreground tabular-nums">
							{labels.length}
						</span>
					)}
				</h2>
				{labels.length === 0 ? (
					<p className="mt-3 text-base text-muted-foreground sm:text-sm">
						This container has no labels.
					</p>
				) : (
					<dl className="mt-3 max-h-80 divide-y divide-border/60 overflow-y-auto text-base sm:text-sm">
						{labels.map(([key, value]) => (
							<div key={key} className="py-2 first:pt-0 last:pb-0">
								<dt className="truncate font-medium" title={key}>
									{key}
								</dt>
								<dd className="break-all font-mono text-muted-foreground">
									{value}
								</dd>
							</div>
						))}
					</dl>
				)}
			</div>
		</div>
	);
}

interface ContainerDetailPanelsProps {
	container: ContainerInfo;
	// The real Docker id, which stays correct while the list refetches after a
	// recreate; the panels talk to the API with it.
	containerId: string;
	isReadOnly: boolean;
	onContainerRecreated: (newContainerId: string) => void;
}

/**
 * Configuration and metadata for the container, parked behind a disclosure row
 * so the log stream keeps the page. Opening one panel closes the others;
 * clicking the open panel's button closes it again.
 */
export function ContainerDetailPanels({
	container,
	containerId,
	isReadOnly,
	onContainerRecreated,
}: ContainerDetailPanelsProps) {
	const [openPanel, setOpenPanel] = useState<Panel | null>(null);

	// There is nothing to exec into unless the container is running.
	const canExec = container.state.toLowerCase() === "running";
	const panels: readonly Panel[] = canExec
		? PANELS
		: PANELS.filter((panel) => panel !== "terminal");

	return (
		<div>
			<div className="-mx-2.5 flex flex-wrap items-center gap-1">
				{panels.map((panel) => {
					const isOpen = openPanel === panel;
					return (
						<button
							key={panel}
							type="button"
							aria-expanded={isOpen}
							aria-controls="container-detail-panel"
							onClick={() => setOpenPanel(isOpen ? null : panel)}
							// text-sm on mobile so the row fits a 390px screen; the
							// vertical padding carries the touch target instead.
							className={`inline-flex items-center gap-1.5 rounded-md px-2.5 py-2 text-sm whitespace-nowrap sm:py-1.5 ${
								isOpen
									? "bg-muted text-foreground"
									: "text-muted-foreground hover:text-foreground"
							}`}
						>
							{PANEL_LABELS[panel]}
							{isOpen && (
								<ChevronDownIcon className="size-4 shrink-0 rotate-180" />
							)}
						</button>
					);
				})}
			</div>

			{openPanel && panels.includes(openPanel) && (
				<div
					id="container-detail-panel"
					className={`mt-3 overflow-hidden rounded-xl border border-border/70 ${
						openPanel === "terminal" ? "" : "p-4 sm:p-5"
					}`}
				>
					{openPanel === "details" && <DetailsPanel container={container} />}
					{openPanel === "environment" && (
						<EnvironmentVariables
							containerId={containerId}
							containerHost={container.host}
							isReadOnly={isReadOnly}
							isCoolifyManaged={isCoolifyManaged(container.labels)}
							onContainerIdChange={onContainerRecreated}
						/>
					)}
					{openPanel === "resources" && (
						<ResourceLimits
							containerId={containerId}
							containerHost={container.host}
							isReadOnly={isReadOnly}
						/>
					)}
					{openPanel === "terminal" && (
						<Terminal containerId={containerId} host={container.host} />
					)}
				</div>
			)}
		</div>
	);
}
