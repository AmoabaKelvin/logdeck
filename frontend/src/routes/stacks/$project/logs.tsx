import {
	createFileRoute,
	useCanGoBack,
	useNavigate,
	useRouter,
} from "@tanstack/react-router";
import { ArrowLeftIcon } from "lucide-react";
import { useMemo } from "react";
import { ThemeToggle } from "@/components/theme-toggle";
import { Button } from "@/components/ui/button";
import {
	Tooltip,
	TooltipContent,
	TooltipTrigger,
} from "@/components/ui/tooltip";
import type { AggregateLogTarget } from "@/features/containers/api/get-aggregated-logs";
import {
	formatContainerName,
	getComposeProject,
} from "@/features/containers/components/container-utils";
import { LogViewer } from "@/features/containers/components/log-viewer/log-viewer";
import { useUrlLogViewState } from "@/features/containers/components/log-viewer/use-log-view-state";
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
	const navigate = useNavigate();
	const router = useRouter();
	const canGoBack = useCanGoBack();

	const project = decodeURIComponent(encodedProject);
	const logViewState = useUrlLogViewState();

	const { data: containersData } = useLiveContainersQuery();
	const containers = containersData?.containers ?? [];

	const targets: AggregateLogTarget[] | undefined = useMemo(() => {
		const stackContainers = containers.filter(
			(container) => getComposeProject(container.labels) === project,
		);
		if (stackContainers.length === 0) return undefined;
		return stackContainers.map((container) => ({
			id: container.id,
			host: container.host,
			name: formatContainerName(container.names),
		}));
	}, [containers, project]);

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
										canGoBack
											? router.history.back()
											: navigate({ to: "/" })
									}
								>
									<ArrowLeftIcon className="size-4" />
								</Button>
							</TooltipTrigger>
							<TooltipContent>Back to Dashboard</TooltipContent>
						</Tooltip>
						<div className="flex-1">
							<h1 className="text-2xl font-bold">Stack Logs</h1>
							<p className="text-sm text-muted-foreground">
								{project}
								{targets
									? ` · ${targets.length} container${targets.length === 1 ? "" : "s"}`
									: " · no containers found"}
							</p>
						</div>
						<ThemeToggle />
					</div>

					{/* Aggregated Logs Card */}
					<LogViewer
						variant="page"
						containerName={project}
						viewState={logViewState}
						targets={targets}
					/>
				</div>
			</div>
		</div>
	);
}
