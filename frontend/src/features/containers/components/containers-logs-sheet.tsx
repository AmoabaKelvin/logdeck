import { ChevronDownIcon, ExternalLinkIcon } from "lucide-react";
import { useEffect, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
	Sheet,
	SheetContent,
	SheetDescription,
	SheetHeader,
	SheetTitle,
} from "@/components/ui/sheet";
import {
	Tooltip,
	TooltipContent,
	TooltipTrigger,
} from "@/components/ui/tooltip";

import type { ContainerInfo } from "../types";
import {
	formatContainerName,
	formatCreatedDate,
	getContainerUrlIdentifier,
	getHealthBadgeClass,
	getStateBadgeClass,
	isCoolifyManaged,
	toTitleCase,
} from "./container-utils";
import { EnvironmentVariables } from "./environment-variables";
import { LogViewer } from "./log-viewer/log-viewer";
import { useLocalLogViewState } from "./log-viewer/use-log-view-state";

interface ContainersLogsSheetProps {
	container: ContainerInfo | null;
	isOpen: boolean;
	isReadOnly?: boolean;
	onOpenChange: (open: boolean) => void;
	onContainerRecreated?: (newContainerId: string) => void;
}

export function ContainersLogsSheet({
	container,
	isOpen,
	isReadOnly = false,
	onOpenChange,
	onContainerRecreated,
}: ContainersLogsSheetProps) {
	const [showLabels, setShowLabels] = useState(false);
	const [showEnvVariables, setShowEnvVariables] = useState(false);
	// Kept here (not inside LogViewer) so the sheet's view settings survive
	// closing and reopening; LogViewer itself remounts per container.
	const logViewState = useLocalLogViewState();

	// Collapse the detail sections whenever the sheet closes or the shown
	// container changes.
	// biome-ignore lint/correctness/useExhaustiveDependencies: intentionally reset on isOpen/container changes
	useEffect(() => {
		setShowLabels(false);
		setShowEnvVariables(false);
	}, [isOpen, container]);

	return (
		<Sheet open={isOpen} onOpenChange={onOpenChange}>
			<SheetContent className="sm:max-w-3xl w-full overflow-y-auto p-6">
				<SheetHeader>
					<SheetTitle>Container Logs</SheetTitle>
					<SheetDescription>
						{container && formatContainerName(container.names)}
					</SheetDescription>
				</SheetHeader>

				{container && (
					<div className="mt-6 space-y-6 pr-2">
						<Card>
							<CardContent>
								<div className="space-y-4">
									<div className="flex items-center justify-between">
										<h3 className="text-sm font-medium">Container Details</h3>
										<Button
											variant="outline"
											size="sm"
											onClick={() => {
												const identifier = getContainerUrlIdentifier(container);
												window.open(
													`/containers/${encodeURIComponent(identifier)}/logs`,
													"_blank",
												);
											}}
										>
											<ExternalLinkIcon className="mr-2 size-4" />
											Open in new tab
										</Button>
									</div>

									<div className="grid gap-3 text-sm">
										<div className="grid grid-cols-3 gap-4">
											<span className="text-muted-foreground">Name</span>
											<span className="col-span-2 font-medium">
												{formatContainerName(container.names)}
											</span>
										</div>
										<div className="grid grid-cols-3 gap-4">
											<span className="text-muted-foreground">ID</span>
											<Tooltip>
												<TooltipTrigger asChild>
													<span className="col-span-2 font-mono text-xs truncate cursor-help">
														{container.id}
													</span>
												</TooltipTrigger>
												<TooltipContent className="max-w-md">
													{container.id}
												</TooltipContent>
											</Tooltip>
										</div>
										<div className="grid grid-cols-3 gap-4">
											<span className="text-muted-foreground">Image</span>
											<Tooltip>
												<TooltipTrigger asChild>
													<span className="col-span-2 font-medium truncate cursor-help">
														{container.image}
													</span>
												</TooltipTrigger>
												<TooltipContent className="max-w-md break-all">
													{container.image}
												</TooltipContent>
											</Tooltip>
										</div>
										<div className="grid grid-cols-3 gap-4">
											<span className="text-muted-foreground">State</span>
											<span className="col-span-2">
												<Badge
													className={`${getStateBadgeClass(container.state)} border-0`}
												>
													{toTitleCase(container.state)}
												</Badge>
											</span>
										</div>
										{container.health && (
											<div className="grid grid-cols-3 gap-4">
												<span className="text-muted-foreground">Health</span>
												<span className="col-span-2">
													<Badge
														className={`${getHealthBadgeClass(container.health)} border-0`}
													>
														{toTitleCase(container.health)}
													</Badge>
												</span>
											</div>
										)}
										<div className="grid grid-cols-3 gap-4">
											<span className="text-muted-foreground">Status</span>
											<span className="col-span-2 font-medium">
												{container.status}
											</span>
										</div>
										<div className="grid grid-cols-3 gap-4">
											<span className="text-muted-foreground">Created</span>
											<span className="col-span-2 font-medium">
												{formatCreatedDate(container.created)}
											</span>
										</div>
										<div className="grid grid-cols-3 gap-4">
											<span className="text-muted-foreground">Command</span>
											<span className="col-span-2 font-mono text-xs break-all">
												{container.command}
											</span>
										</div>
										{/* Labels Section */}
										{container.labels &&
											Object.keys(container.labels).length > 0 && (
												<div className="space-y-2 border-t pt-2">
													<Button
														variant="ghost"
														size="sm"
														onClick={() => setShowLabels((value) => !value)}
														className="h-8 w-full justify-start text-muted-foreground hover:text-foreground"
													>
														<ChevronDownIcon
															className={`mr-2 size-4 transition-transform ${
																showLabels ? "rotate-180" : ""
															}`}
														/>
														{showLabels ? "Hide" : "Show"} container labels (
														{Object.keys(container.labels).length})
													</Button>
													{showLabels && (
														<div className="max-h-[200px] space-y-2 overflow-y-auto rounded-md border bg-muted/30 p-3">
															{Object.entries(container.labels).map(
																([key, value]) => (
																	<div
																		key={key}
																		className="rounded-md bg-background p-2 text-xs"
																	>
																		<div className="mb-1 font-semibold text-foreground">
																			{key}
																		</div>
																		<div className="break-all font-mono text-muted-foreground">
																			{value}
																		</div>
																	</div>
																),
															)}
														</div>
													)}
												</div>
											)}

										{/* Environment Variables Section */}
										<div className="space-y-2 border-t pt-2">
											<Button
												variant="ghost"
												size="sm"
												onClick={() => setShowEnvVariables((value) => !value)}
												className="h-8 w-full justify-start text-muted-foreground hover:text-foreground"
											>
												<ChevronDownIcon
													className={`mr-2 size-4 transition-transform ${
														showEnvVariables ? "rotate-180" : ""
													}`}
												/>
												{showEnvVariables ? "Hide" : "Show"} environment
												variables
											</Button>
											{showEnvVariables && (
												<div className="max-h-[300px] overflow-y-auto">
													<EnvironmentVariables
														containerId={container.id}
														containerHost={container.host}
														isReadOnly={isReadOnly}
														isCoolifyManaged={isCoolifyManaged(
															container.labels,
														)}
														onContainerIdChange={onContainerRecreated}
													/>
												</div>
											)}
										</div>
									</div>
								</div>
							</CardContent>
						</Card>

						<LogViewer
							key={container.id}
							variant="sheet"
							containerId={container.id}
							host={container.host}
							containerName={container.names?.[0]}
							viewState={logViewState}
						/>
					</div>
				)}
			</SheetContent>
		</Sheet>
	);
}
