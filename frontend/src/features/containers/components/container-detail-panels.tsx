import { useState } from "react";

import { ChevronDownIcon } from "@/components/ui/icons";
import type { ContainerInspect } from "../api/get-container-inspect";
import type { ContainerInfo, ContainerStats } from "../types";
import { ContainerEnvPanel } from "./container-env-panel";
import { ContainerLimitsPanel } from "./container-limits-panel";
import { ContainerNetworkPanel } from "./container-network-panel";
import { ContainerOverviewPanel } from "./container-overview-panel";
import { isCoolifyManaged } from "./container-utils";

const PANELS = ["overview", "network", "environment", "limits"] as const;
type Panel = (typeof PANELS)[number];

const PANEL_LABELS: Record<Panel, string> = {
	overview: "Overview",
	network: "Network",
	environment: "Environment",
	limits: "Limits",
};

interface ContainerDetailPanelsProps {
	container: ContainerInfo;
	// The real Docker id, which stays correct while the list refetches after a
	// recreate; the panels talk to the API with it.
	containerId: string;
	hostAddress: string | undefined;
	isReadOnly: boolean;
	stats: ContainerStats | undefined;
	inspect: ContainerInspect | undefined;
	isInspectLoading: boolean;
	isInspectError: boolean;
	onContainerRecreated: (newContainerId: string) => void;
}

/**
 * Everything about the container that is not its log stream, parked behind a
 * disclosure row so the stream keeps the page. One panel at a time; clicking
 * the open one closes it again.
 */
export function ContainerDetailPanels({
	container,
	containerId,
	hostAddress,
	isReadOnly,
	stats,
	inspect,
	isInspectLoading,
	isInspectError,
	onContainerRecreated,
}: ContainerDetailPanelsProps) {
	const [openPanel, setOpenPanel] = useState<Panel | null>(null);

	return (
		<div>
			<div className="-mx-2.5 flex flex-wrap items-center gap-1">
				{PANELS.map((panel) => {
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

			{openPanel && (
				<div
					id="container-detail-panel"
					className="mt-3 border-t border-border/70 pt-5 pb-2"
				>
					{openPanel === "overview" && (
						<ContainerOverviewPanel
							container={container}
							inspect={inspect}
							isLoading={isInspectLoading}
							isError={isInspectError}
						/>
					)}
					{openPanel === "network" && (
						<ContainerNetworkPanel
							inspect={inspect}
							isLoading={isInspectLoading}
							isError={isInspectError}
							hostAddress={hostAddress}
						/>
					)}
					{openPanel === "environment" && (
						<ContainerEnvPanel
							containerId={containerId}
							containerHost={container.host}
							isReadOnly={isReadOnly}
							isCoolifyManaged={isCoolifyManaged(container.labels)}
							onContainerIdChange={onContainerRecreated}
						/>
					)}
					{openPanel === "limits" && (
						<ContainerLimitsPanel
							containerId={containerId}
							containerHost={container.host}
							isReadOnly={isReadOnly}
							stats={stats}
							inspect={inspect}
						/>
					)}
				</div>
			)}
		</div>
	);
}
