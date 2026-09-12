import { useState } from "react";

import type { ContainerInspect } from "../api/get-container-inspect";
import type { ContainerInfo, ContainerStats } from "../types";
import { ContainerEnvPanel } from "./container-env-panel";
import { ContainerLimitsPanel } from "./container-limits-panel";
import { ContainerNetworkPanel } from "./container-network-panel";
import { ContainerOverviewPanel } from "./container-overview-panel";
import {
	DisclosureTrigger,
	PanelError,
	PanelLoading,
} from "./container-panel-ui";
import { isCoolifyManaged } from "./container-utils";

const PANELS = ["overview", "network", "environment", "limits"] as const;
type Panel = (typeof PANELS)[number];

const PANEL_LABELS: Record<Panel, string> = {
	overview: "Overview",
	network: "Network",
	environment: "Environment",
	limits: "Limits",
};

const PANEL_ID = "container-detail-panel";

interface ContainerDetailPanelsProps {
	container: ContainerInfo;
	// The real Docker id, which stays correct while the list refetches after a
	// recreate; the panels talk to the API with it.
	containerId: string;
	hostAddress: string | undefined;
	isReadOnly: boolean;
	stats: ContainerStats | undefined;
	inspect: ContainerInspect | undefined;
	isInspectError: boolean;
	onContainerRecreated: (newContainerId: string) => void;
}

/**
 * Which panel to show. The two panels built on inspect wait for it here rather
 * than each carrying its own loading and error branches — past this point
 * `inspect` is simply there.
 */
function PanelBody({
	panel,
	container,
	containerId,
	hostAddress,
	isReadOnly,
	stats,
	inspect,
	isInspectError,
	onContainerRecreated,
}: ContainerDetailPanelsProps & { panel: Panel }) {
	if (panel === "overview" || panel === "network") {
		if (!inspect) {
			return isInspectError ? (
				<PanelError>Could not inspect this container.</PanelError>
			) : (
				<PanelLoading label="Inspecting container…" />
			);
		}
		return panel === "overview" ? (
			<ContainerOverviewPanel container={container} inspect={inspect} />
		) : (
			<ContainerNetworkPanel inspect={inspect} hostAddress={hostAddress} />
		);
	}

	return panel === "environment" ? (
		<ContainerEnvPanel
			containerId={containerId}
			containerHost={container.host}
			isReadOnly={isReadOnly}
			isCoolifyManaged={isCoolifyManaged(container.labels)}
			onContainerIdChange={onContainerRecreated}
		/>
	) : (
		<ContainerLimitsPanel
			containerId={containerId}
			containerHost={container.host}
			isReadOnly={isReadOnly}
			stats={stats}
			inspect={inspect}
		/>
	);
}

/**
 * Everything about the container that is not its log stream, parked behind a
 * disclosure row so the stream keeps the page. One panel at a time; clicking
 * the open one closes it again.
 */
export function ContainerDetailPanels(props: ContainerDetailPanelsProps) {
	const [openPanel, setOpenPanel] = useState<Panel | null>(null);

	return (
		<div>
			<div className="-mx-2.5 flex flex-wrap items-center gap-1">
				{PANELS.map((panel) => (
					<DisclosureTrigger
						key={panel}
						label={PANEL_LABELS[panel]}
						isOpen={openPanel === panel}
						controls={PANEL_ID}
						onClick={() =>
							setOpenPanel((open) => (open === panel ? null : panel))
						}
					/>
				))}
			</div>

			{openPanel && (
				<div id={PANEL_ID} className="mt-3 border-t border-border/70 pt-5 pb-2">
					<PanelBody {...props} panel={openPanel} />
				</div>
			)}
		</div>
	);
}
