import { useQueryClient } from "@tanstack/react-query";
import {
	createFileRoute,
	useCanGoBack,
	useNavigate,
	useRouter,
} from "@tanstack/react-router";
import { ArrowLeftIcon, TerminalIcon } from "lucide-react";
import { useRef } from "react";
import { ThemeToggle } from "@/components/theme-toggle";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
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
	getHealthBadgeClass,
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
import { useHistoryContainers } from "@/features/containers/hooks/use-history-containers";
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

	const logViewerRef = useRef<LogViewerHandle>(null);
	const logViewState = useUrlLogViewState();

	// The URL parameter can be a container name or an ID
	const containerIdentifier = decodeURIComponent(encodedContainerId);

	const { data: containersData } = useLiveContainersQuery();
	const { statsMap } = useContainerStats();

	const containers = containersData?.containers ?? [];

	// Find container by name (preferred) or ID (fallback for backward compatibility)
	const container = containers.find((c) => {
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

	// Prefer the real ID for API calls; fall back to the raw identifier while
	// the container list is still loading.
	const actualContainerId = container?.id || containerIdentifier;

	// The container may have been removed or recreated under a new ID while its
	// logs live on in the store. Once the live list has loaded without a match,
	// look the name up there before treating it as gone.
	const isUnresolved = containersData !== undefined && !container;
	const { data: storedContainers } = useHistoryContainers(isUnresolved);
	const storedContainer = isUnresolved
		? storedContainers?.find(
				(stored) => stored.name.replace(/^\//, "") === containerIdentifier,
			)
		: undefined;
	const isRemoved = storedContainer !== undefined;

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
							{(container || isRemoved) && (
								<div className="flex items-center gap-2">
									<p className="text-sm text-muted-foreground">
										{container?.names?.[0]?.replace(/^\//, "") ||
											containerIdentifier}
									</p>
									{isRemoved && (
										<Badge variant="outline" className="text-muted-foreground">
											Removed
										</Badge>
									)}
								</div>
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
											<div className="flex items-center gap-1.5">
												<Badge
													className={`${getStateBadgeClass(container.state)} border-0`}
												>
													{toTitleCase(container.state)}
												</Badge>
												{container.health && (
													<Badge
														className={`${getHealthBadgeClass(container.health)} border-0`}
													>
														{toTitleCase(container.health)}
													</Badge>
												)}
											</div>
										</div>
										<div>
											<span className="text-muted-foreground block mb-1">
												Status
											</span>
											<p className="font-medium">{container.status}</p>
										</div>
									</div>

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

									<Tabs defaultValue="environment" className="border-t pt-4">
										<TabsList>
											<TabsTrigger value="environment">Environment</TabsTrigger>
											<TabsTrigger value="resources">Resources</TabsTrigger>
											<TabsTrigger value="labels">
												Labels
												{container.labels &&
													Object.keys(container.labels).length > 0 && (
														<span className="text-muted-foreground">
															{Object.keys(container.labels).length}
														</span>
													)}
											</TabsTrigger>
											<TabsTrigger value="terminal">
												<TerminalIcon className="size-4" />
												Terminal
											</TabsTrigger>
										</TabsList>

										<TabsContent value="environment" className="pt-2">
											<div className="max-h-[300px] overflow-y-auto">
												<EnvironmentVariables
													containerId={actualContainerId}
													containerHost={container.host}
													isCoolifyManaged={isCoolifyManaged(container.labels)}
													onContainerIdChange={handleContainerRecreated}
												/>
											</div>
										</TabsContent>

										<TabsContent value="resources" className="pt-2">
											<ResourceLimits
												containerId={actualContainerId}
												containerHost={container.host}
												isReadOnly={containersData?.readOnly}
											/>
										</TabsContent>

										<TabsContent value="labels" className="pt-2">
											{container.labels &&
											Object.keys(container.labels).length > 0 ? (
												<div className="max-h-[300px] overflow-y-auto">
													{Object.entries(container.labels).map(
														([key, value]) => (
															<div
																key={key}
																className="border-b py-2 text-xs last:border-b-0"
															>
																<div className="mb-0.5 font-medium text-foreground">
																	{key}
																</div>
																<div className="break-all font-mono text-muted-foreground">
																	{value}
																</div>
															</div>
														),
													)}
												</div>
											) : (
												<p className="py-2 text-sm text-muted-foreground">
													This container has no labels.
												</p>
											)}
										</TabsContent>

										<TabsContent value="terminal" className="pt-2">
											<Terminal
												containerId={actualContainerId}
												host={container.host}
											/>
										</TabsContent>
									</Tabs>
								</div>
							</CardContent>
						</Card>
					)}

					{/* Logs Card */}
					<LogViewer
						ref={logViewerRef}
						variant="page"
						containerId={actualContainerId}
						host={container?.host ?? storedContainer?.host}
						containerName={container?.names?.[0] ?? storedContainer?.name}
						viewState={logViewState}
						historyOnly={isRemoved}
					/>
				</div>
			</div>
		</div>
	);
}
