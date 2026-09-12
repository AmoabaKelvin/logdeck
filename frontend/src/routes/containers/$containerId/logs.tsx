import { useQueryClient } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { useRef, useState } from "react";
import { AppHeader } from "@/components/app-header";
import { ConfirmActionDialog } from "@/features/containers/components/confirm-action-dialog";
import { ContainerDetailHeader } from "@/features/containers/components/container-detail-header";
import { ContainerDetailPanels } from "@/features/containers/components/container-detail-panels";
import { formatContainerName } from "@/features/containers/components/container-utils";
import type { LogViewerHandle } from "@/features/containers/components/log-viewer/log-viewer";
import { LogViewer } from "@/features/containers/components/log-viewer/log-viewer";
import { useUrlLogViewState } from "@/features/containers/components/log-viewer/use-log-view-state";
import { TerminalDialog } from "@/features/containers/components/terminal-dialog";
import { useContainerActions } from "@/features/containers/hooks/use-container-actions";
import { useContainerInspect } from "@/features/containers/hooks/use-container-inspect";
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
	const queryClient = useQueryClient();

	const logViewerRef = useRef<LogViewerHandle>(null);
	const logViewState = useUrlLogViewState();
	const [isShellOpen, setIsShellOpen] = useState(false);

	// The URL parameter can be a container name or an ID
	const containerIdentifier = decodeURIComponent(encodedContainerId);

	const { data: containersData, refetch } = useLiveContainersQuery();
	const { statsMap, statsHistory } = useContainerStats();

	const containers = containersData?.containers ?? [];
	const isReadOnly = containersData?.readOnly ?? false;

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

	// Inspect is where the interesting facts live: restart counts, health probe
	// output, mounts, networks. The container list carries none of it.
	const { data: inspect, isError: isInspectError } = useContainerInspect(
		container?.id,
		container?.host,
	);

	const hostAddress = containersData?.hosts?.find(
		(host) => host.name === container?.host,
	)?.host;

	const {
		pendingActions,
		confirmAction,
		isConfirmActionPending,
		startContainerAction,
		stopContainerAction,
		restartContainerAction,
		deleteContainerAction,
		confirmPendingAction,
		handleConfirmDialogOpenChange,
	} = useContainerActions(refetch);

	const handleContainerRecreated = async (_newContainerId: string) => {
		await queryClient.invalidateQueries({ queryKey: ["containers"] });
		await logViewerRef.current?.refreshAfterRecreate();
	};

	// On a wide screen the page is an app shell: the chrome stays put and the log
	// list takes whatever height is left. Narrow screens keep normal document
	// flow, where a fixed-height shell would squeeze the logs to nothing.
	return (
		<div className="isolate flex min-h-dvh flex-col bg-background lg:h-dvh lg:min-h-0 lg:overflow-hidden">
			<AppHeader />
			<main className="app-width flex w-full flex-1 flex-col px-4 py-8 sm:px-6 lg:min-h-0 lg:overflow-y-auto lg:px-8">
				<ContainerDetailHeader
					name={
						container
							? formatContainerName(container.names)
							: containerIdentifier
					}
					container={container}
					isRemoved={isRemoved}
					isReadOnly={isReadOnly}
					stats={container ? statsMap[container.id] : undefined}
					history={container ? (statsHistory[container.id] ?? []) : []}
					inspect={inspect}
					isActionPending={container ? pendingActions.has(container.id) : false}
					onStart={() => container && startContainerAction(container)}
					onStop={() => container && stopContainerAction(container)}
					onRestart={() => container && restartContainerAction(container)}
					onDelete={() => container && deleteContainerAction(container)}
					onOpenShell={() => setIsShellOpen(true)}
				/>

				{container && (
					<section className="mt-4">
						<ContainerDetailPanels
							container={container}
							containerId={actualContainerId}
							hostAddress={hostAddress}
							isReadOnly={isReadOnly}
							stats={statsMap[container.id]}
							inspect={inspect}
							isInspectError={isInspectError}
							onContainerRecreated={handleContainerRecreated}
						/>
					</section>
				)}

				<section className="mt-6 flex flex-col lg:min-h-0 lg:flex-1">
					<LogViewer
						ref={logViewerRef}
						variant="page"
						containerId={actualContainerId}
						host={container?.host ?? storedContainer?.host}
						containerName={container?.names?.[0] ?? storedContainer?.name}
						viewState={logViewState}
						historyOnly={isRemoved}
					/>
				</section>
			</main>

			{container && (
				<TerminalDialog
					containerId={actualContainerId}
					containerName={formatContainerName(container.names)}
					host={container.host}
					open={isShellOpen}
					onOpenChange={setIsShellOpen}
				/>
			)}

			<ConfirmActionDialog
				action={confirmAction}
				isPending={isConfirmActionPending}
				onConfirm={() => {
					void confirmPendingAction();
				}}
				onOpenChange={handleConfirmDialogOpenChange}
			/>
		</div>
	);
}
