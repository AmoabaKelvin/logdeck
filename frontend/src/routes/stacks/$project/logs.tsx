import { createFileRoute, Link } from "@tanstack/react-router";
import { useMemo } from "react";
import { AppHeader } from "@/components/app-header";
import { ArrowLeftIcon } from "@/components/ui/icons";
import type { AggregateLogTarget } from "@/features/containers/api/get-aggregated-logs";
import {
	formatContainerName,
	getComposeProject,
	selectStackMembers,
	synthesizeRemovedContainers,
} from "@/features/containers/components/container-utils";
import { LogViewer } from "@/features/containers/components/log-viewer/log-viewer";
import { useUrlLogViewState } from "@/features/containers/components/log-viewer/use-log-view-state";
import { StackMembersPanel } from "@/features/containers/components/stack-members-panel";
import { useHistoryContainers } from "@/features/containers/hooks/use-history-containers";
import { useHistoryStatus } from "@/features/containers/hooks/use-history-status";
import { useLiveContainersQuery } from "@/features/containers/hooks/use-live-containers-query";
import { requireAuthIfEnabled } from "@/lib/auth-guard";

export const Route = createFileRoute("/stacks/$project/logs")({
	beforeLoad: async () => {
		await requireAuthIfEnabled();
	},
	component: StackLogsPage,
});

function StackLogsPage() {
	const { project: encodedProject } = Route.useParams();

	const project = decodeURIComponent(encodedProject);
	const logViewState = useUrlLogViewState();

	const { data: containersData } = useLiveContainersQuery();
	const containers = containersData?.containers ?? [];

	// Members that were torn down keep their stored logs, so the stack still
	// lists them — they just cannot join the live aggregated stream.
	const { data: historyStatus } = useHistoryStatus();
	const isHistoryEnabled = historyStatus?.enabled === true;
	const { data: storedContainers } = useHistoryContainers(isHistoryEnabled);
	const members = useMemo(() => {
		const removed =
			isHistoryEnabled && storedContainers
				? synthesizeRemovedContainers(storedContainers, containers)
				: [];
		return selectStackMembers(containers, removed, project);
	}, [containers, storedContainers, isHistoryEnabled, project]);

	const targets: AggregateLogTarget[] | undefined = useMemo(() => {
		const stackContainers = containers.filter(
			(container) => getComposeProject(container.labels) === project,
		);
		if (stackContainers.length === 0) return undefined;
		// Every member carries the project name; repeating it on every log line
		// would just eat the column. "logdeck-api-1" reads as "api-1" here.
		return stackContainers.map((container) => {
			const name = formatContainerName(container.names);
			return {
				id: container.id,
				host: container.host,
				name: name.startsWith(`${project}-`)
					? name.slice(project.length + 1)
					: name,
			};
		});
	}, [containers, project]);

	const liveCount = targets?.length ?? 0;

	// Same app-shell layout as the container detail page: chrome stays put on a
	// wide screen and the log list takes the height that is left.
	return (
		<div className="isolate flex min-h-dvh flex-col bg-background lg:h-dvh lg:min-h-0 lg:overflow-hidden">
			<AppHeader />
			<main className="app-width flex w-full flex-1 flex-col px-4 py-8 sm:px-6 lg:min-h-0 lg:overflow-y-auto lg:px-8">
				<header>
					<Link
						to="/"
						className="inline-flex items-center gap-1.5 rounded-sm text-base text-muted-foreground hover:text-foreground sm:text-sm"
					>
						<ArrowLeftIcon className="size-4 shrink-0" />
						Containers
					</Link>
					<h1 className="mt-3 truncate text-2xl font-semibold tracking-tight">
						{project}
					</h1>
					<p className="mt-1 truncate text-base/6 text-muted-foreground sm:text-sm/6">
						{liveCount === 0
							? "No running containers in this stack."
							: `Merged logs from ${liveCount} container${liveCount === 1 ? "" : "s"}.`}
					</p>
				</header>

				<section className="mt-4">
					<StackMembersPanel
						members={members}
						isReadOnly={containersData?.readOnly ?? false}
					/>
				</section>

				<section className="mt-6 flex flex-col lg:min-h-0 lg:flex-1">
					<LogViewer
						variant="page"
						containerName={project}
						viewState={logViewState}
						targets={targets}
					/>
				</section>
			</main>
		</div>
	);
}
