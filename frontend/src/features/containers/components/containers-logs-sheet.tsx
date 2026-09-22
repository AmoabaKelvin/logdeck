import { Link } from "@tanstack/react-router";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ExternalLinkIcon } from "@/components/ui/icons";
import {
	Sheet,
	SheetContent,
	SheetDescription,
	SheetHeader,
	SheetTitle,
} from "@/components/ui/sheet";

import { useContainerInspect } from "../hooks/use-container-inspect";
import type { ContainerInfo, ContainerStats } from "../types";
import { ContainerVitals } from "./container-detail-header";
import { ContainerDetailPanels } from "./container-detail-panels";
import {
	formatContainerName,
	formatImageName,
	formatRelativeCreated,
	getContainerUrlIdentifier,
	getHealthBadgeClass,
	getStateBadgeClass,
	splitContainerStatus,
	toTitleCase,
} from "./container-utils";
import { LogViewer } from "./log-viewer/log-viewer";
import { useLocalLogViewState } from "./log-viewer/use-log-view-state";

interface ContainersLogsSheetProps {
	container: ContainerInfo | null;
	stats?: ContainerStats;
	// CPU samples, for the same sparkline the detail page shows.
	history?: number[];
	hostAddress?: string;
	isOpen: boolean;
	isReadOnly?: boolean;
	onOpenChange: (open: boolean) => void;
	onContainerRecreated?: (newContainerId: string) => void;
}

/**
 * A quick look at one container without leaving the list: the detail page's
 * header, panels and log viewer, in a side sheet. The full page adds
 * fullscreen and the container actions.
 */
export function ContainersLogsSheet({
	container,
	stats,
	history = [],
	hostAddress,
	isOpen,
	isReadOnly = false,
	onOpenChange,
	onContainerRecreated,
}: ContainersLogsSheetProps) {
	// Kept here (not inside LogViewer) so the sheet's view settings survive
	// closing and reopening; LogViewer itself remounts per container.
	const logViewState = useLocalLogViewState();
	const { data: inspect, isError: isInspectError } = useContainerInspect(
		isOpen ? container?.id : undefined,
		container?.host,
	);

	const name = container ? formatContainerName(container.names) : "";
	const { label } = container ? splitContainerStatus(container) : { label: "" };
	const meta = container
		? [
				formatImageName(container.image),
				container.host,
				`created ${formatRelativeCreated(container.created)}`,
			].join(" · ")
		: "";

	return (
		<Sheet open={isOpen} onOpenChange={onOpenChange}>
			<SheetContent className="flex w-full flex-col gap-0 p-0 sm:max-w-4xl">
				{container && (
					<>
						<SheetHeader className="shrink-0 gap-0 px-4 pt-5 pr-14 sm:px-6">
							<div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between sm:gap-6">
								<div className="min-w-0">
									<div className="flex flex-wrap items-center gap-x-2.5 gap-y-1">
										<SheetTitle className="min-w-0 truncate text-lg font-semibold tracking-tight">
											{name}
										</SheetTitle>
										<Badge
											className={`border-transparent ${getStateBadgeClass(container.state)}`}
										>
											{label}
										</Badge>
										{container.health && (
											<Badge
												className={`border-transparent ${getHealthBadgeClass(container.health)}`}
											>
												{toTitleCase(container.health)}
											</Badge>
										)}
									</div>
									<SheetDescription className="mt-1 truncate text-base/6 text-muted-foreground sm:text-sm/6">
										{meta}
									</SheetDescription>
								</div>
								<Button
									variant="outline"
									size="sm"
									asChild
									className="sm:shrink-0"
								>
									<Link
										to="/containers/$containerId/logs"
										params={{
											containerId: getContainerUrlIdentifier(container),
										}}
										search={{ host: container.host }}
									>
										Open page
										<ExternalLinkIcon className="size-4" />
									</Link>
								</Button>
							</div>

							<ContainerVitals
								container={container}
								isRemoved={false}
								stats={stats}
								history={history}
								inspect={inspect}
								className="mt-4 border-t border-border/70 pt-4"
							/>
						</SheetHeader>

						<div className="shrink-0 px-4 pt-3 sm:px-6">
							<ContainerDetailPanels
								container={container}
								containerId={container.id}
								hostAddress={hostAddress}
								isReadOnly={isReadOnly}
								stats={stats}
								inspect={inspect}
								isInspectError={isInspectError}
								onContainerRecreated={(id) => onContainerRecreated?.(id)}
								panelClassName="max-h-[40dvh] overflow-y-auto"
							/>
						</div>

						<div className="mt-3 flex min-h-0 flex-1 flex-col">
							<LogViewer
								key={container.id}
								variant="sheet"
								containerId={container.id}
								host={container.host}
								containerName={container.names?.[0]}
								viewState={logViewState}
							/>
						</div>
					</>
				)}
			</SheetContent>
		</Sheet>
	);
}
