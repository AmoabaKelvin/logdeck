import { useQueryClient } from "@tanstack/react-query";
import {
	createFileRoute,
	useCanGoBack,
	useNavigate,
	useRouter,
} from "@tanstack/react-router";
import { ArrowLeftIcon, ChevronDownIcon, TerminalIcon } from "lucide-react";
import { useRef, useState } from "react";
import { ThemeToggle } from "@/components/theme-toggle";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
	Tooltip,
	TooltipContent,
	TooltipTrigger,
} from "@/components/ui/tooltip";
import {
	formatContainerName,
	formatCPUPercent,
	formatCreatedDate,
	formatMemoryStats,
	formatUptime,
	getStateBadgeClass,
	isCoolifyManaged,
	toTitleCase,
} from "@/features/containers/components/container-utils";
import { EnvironmentVariables } from "@/features/containers/components/environment-variables";
import type { LogViewerHandle } from "@/features/containers/components/log-viewer/log-viewer";
import { LogViewer } from "@/features/containers/components/log-viewer/log-viewer";
import { useUrlLogViewState } from "@/features/containers/components/log-viewer/use-log-view-state";
import { ResourceLimits } from "@/features/containers/components/resource-limits";
import { Terminal } from "@/features/containers/components/terminal";
import { useContainerStats } from "@/features/containers/hooks/use-container-stats";
import { useLiveContainersQuery } from "@/features/containers/hooks/use-live-containers-query";
import { requireAuthIfEnabled } from "@/lib/auth-guard";

export const Route = createFileRoute("/containers/$containerId/logs")({
	beforeLoad: async () => {
		await requireAuthIfEnabled();
	},
	component: ContainerLogsPage,
});

function ContainerLogsPage() {
	const { containerId: encodedContainerId } = Route.useParams();
	const navigate = useNavigate();
	const router = useRouter();
	const canGoBack = useCanGoBack();
	const queryClient = useQueryClient();

	const [showLabels, setShowLabels] = useState(false);
	const [showEnvVariables, setShowEnvVariables] = useState(false);
	const [showResourceLimits, setShowResourceLimits] = useState(false);
	const [showTerminal, setShowTerminal] = useState(false);
	const logViewerRef = useRef<LogViewerHandle>(null);
	const logViewState = useUrlLogViewState();

	// Decode the URL parameter (could be name or ID)
	const containerIdentifier = decodeURIComponent(encodedContainerId);

	// Fetch container info
	const { data: containersData } = useLiveContainersQuery();
	const { statsMap } = useContainerStats();

	const containers = containersData?.containers ?? [];

	// Find container by name (preferred) or ID (fallback for backward compatibility)
	const container = containers.find((c) => {
		// Check if identifier matches the container name (without leading slash)
		if (c.names && c.names.length > 0) {
			const cleanName = c.names[0].startsWith("/")
				? c.names[0].slice(1)
				: c.names[0];
			if (cleanName === containerIdentifier) {
				return true;
			}
		}
		// Fallback: check if it matches the ID (full or short)
		return c.id === containerIdentifier || c.id.startsWith(containerIdentifier);
	});

	// Use the actual container ID for API calls (Docker API accepts both name and ID, but we'll use ID for consistency)
	const actualContainerId = container?.id || containerIdentifier;

	const handleContainerRecreated = async (_newContainerId: string) => {
		await queryClient.invalidateQueries({ queryKey: ["containers"] });
		await logViewerRef.current?.refreshAfterRecreate();
	};

	return (
		<div className="min-h-screen bg-background">
			<div className="container mx-auto px-4 py-6">
				<div className="space-y-6">
					{/* Header */}
					<div className="flex items-center gap-4">
						<Tooltip>
							<TooltipTrigger asChild>
								<Button
									variant="ghost"
									size="icon"
									onClick={() =>
										canGoBack ? router.history.back() : navigate({ to: "/" })
									}
								>
									<ArrowLeftIcon className="size-4" />
								</Button>
							</TooltipTrigger>
							<TooltipContent>Back to Dashboard</TooltipContent>
						</Tooltip>
						<div className="flex-1">
							<h1 className="text-2xl font-bold">Container Logs</h1>
							{container && (
								<p className="text-sm text-muted-foreground">
									{container.names?.[0]?.replace(/^\//, "") ||
										containerIdentifier}
								</p>
							)}
						</div>
						<ThemeToggle />
					</div>

					{/* Container Info Card */}
					{container && (
						<Card>
							<CardHeader>
								<CardTitle className="text-base">Container Details</CardTitle>
							</CardHeader>
							<CardContent>
								<div className="space-y-4">
									<div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm">
										<div>
											<span className="text-muted-foreground block mb-1">
												Name
											</span>
											<p className="font-medium">
												{formatContainerName(container.names)}
											</p>
										</div>
										<div className="md:col-span-2">
											<span className="text-muted-foreground block mb-1">
												ID
											</span>
											<Tooltip>
												<TooltipTrigger asChild>
													<p className="font-mono text-xs truncate cursor-help">
														{container.id}
													</p>
												</TooltipTrigger>
												<TooltipContent className="max-w-md">
													{container.id}
												</TooltipContent>
											</Tooltip>
										</div>
									</div>

									<div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm">
										<div>
											<span className="text-muted-foreground block mb-1">
												Image
											</span>
											<p className="font-medium">{container.image}</p>
										</div>
										<div>
											<span className="text-muted-foreground block mb-1">
												State
											</span>
											<Badge
												className={`${getStateBadgeClass(container.state)} border-0`}
											>
												{toTitleCase(container.state)}
											</Badge>
										</div>
										<div>
											<span className="text-muted-foreground block mb-1">
												Status
											</span>
											<p className="font-medium">{container.status}</p>
										</div>
									</div>

									{/* Resource Usage Section - only for running containers */}
									{container.state.toLowerCase() === "running" && (
										<div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm">
											<div className="md:col-span-3">
												<span className="text-muted-foreground block mb-1">
													Resource Usage
												</span>
												<div className="flex gap-6 font-mono text-sm">
													<div>
														<span className="text-muted-foreground">CPU: </span>
														<span className="font-medium">
															{formatCPUPercent(
																statsMap[container.id]?.cpu_percent,
															)}
														</span>
													</div>
													<div>
														<span className="text-muted-foreground">
															Memory:{" "}
														</span>
														<span className="font-medium">
															{formatMemoryStats(statsMap[container.id])}
														</span>
													</div>
												</div>
											</div>
										</div>
									)}

									<div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm">
										<div>
											<span className="text-muted-foreground block mb-1">
												Uptime
											</span>
											<p className="font-medium">
												{formatUptime(container.created)}
											</p>
										</div>
										<div>
											<span className="text-muted-foreground block mb-1">
												Created
											</span>
											<p className="font-medium">
												{formatCreatedDate(container.created)}
											</p>
										</div>
										<div>
											<span className="text-muted-foreground block mb-1">
												Command
											</span>
											<p className="font-mono text-xs break-all">
												{container.command}
											</p>
										</div>
									</div>

									{/* Labels Section */}
									{container.labels &&
										Object.keys(container.labels).length > 0 && (
											<div className="space-y-2 border-t pt-4">
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
									<div className="space-y-2 border-t pt-4">
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
											{showEnvVariables ? "Hide" : "Show"} environment variables
										</Button>
										{showEnvVariables && container && (
											<div className="max-h-[300px] overflow-y-auto">
												<EnvironmentVariables
													containerId={actualContainerId}
													containerHost={container.host}
													isCoolifyManaged={isCoolifyManaged(container?.labels)}
													onContainerIdChange={handleContainerRecreated}
												/>
											</div>
										)}
									</div>

									{/* Resource Limits Section */}
									<div className="space-y-2 border-t pt-4">
										<Button
											variant="ghost"
											size="sm"
											onClick={() => setShowResourceLimits((value) => !value)}
											className="h-8 w-full justify-start text-muted-foreground hover:text-foreground"
										>
											<ChevronDownIcon
												className={`mr-2 size-4 transition-transform ${
													showResourceLimits ? "rotate-180" : ""
												}`}
											/>
											{showResourceLimits ? "Hide" : "Show"} resource limits
										</Button>
										{showResourceLimits && container && (
											<ResourceLimits
												containerId={actualContainerId}
												containerHost={container.host}
												isReadOnly={containersData?.readOnly}
											/>
										)}
									</div>

									{/* Terminal Section */}
									<div className="space-y-2 border-t pt-4">
										<Button
											variant="ghost"
											size="sm"
											onClick={() => setShowTerminal((value) => !value)}
											className="h-8 w-full justify-start text-muted-foreground hover:text-foreground"
										>
											<ChevronDownIcon
												className={`mr-2 size-4 transition-transform ${
													showTerminal ? "rotate-180" : ""
												}`}
											/>
											<TerminalIcon className="mr-2 size-4" />
											{showTerminal ? "Hide" : "Show"} terminal
										</Button>
										{container && showTerminal && (
											<div className="mt-2">
												<Terminal
													containerId={actualContainerId}
													host={container.host}
												/>
											</div>
										)}
									</div>
								</div>
							</CardContent>
						</Card>
					)}

					{/* Logs Card */}
					<LogViewer
						ref={logViewerRef}
						variant="page"
						containerId={actualContainerId}
						host={container?.host}
						containerName={container?.names?.[0]}
						viewState={logViewState}
					/>
				</div>
			</div>
		</div>
	);
}
