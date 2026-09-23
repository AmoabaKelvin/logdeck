import { createFileRoute } from "@tanstack/react-router";
import { AppHeader } from "@/components/app-header";
import { LogViewer } from "@/features/containers/components/log-viewer/log-viewer";
import { useUrlLogViewState } from "@/features/containers/components/log-viewer/use-log-view-state";
import { useHistoryStatus } from "@/features/containers/hooks/use-history-status";
import { requireAuthIfEnabled } from "@/lib/auth-guard";

export const Route = createFileRoute("/search")({
	beforeLoad: async () => {
		await requireAuthIfEnabled();
	},
	component: SearchLogsPage,
});

function SearchLogsPage() {
	const logViewState = useUrlLogViewState();
	const { data: historyStatus, isPending } = useHistoryStatus();
	const isHistoryEnabled = historyStatus?.enabled === true;

	// Same app-shell layout as the stack logs page.
	return (
		<div className="isolate flex min-h-dvh flex-col bg-background lg:h-dvh lg:min-h-0 lg:overflow-hidden">
			<AppHeader />
			<main className="app-width flex w-full flex-1 flex-col px-4 py-8 sm:px-6 lg:min-h-0 lg:overflow-y-auto lg:px-8">
				<header>
					<h1 className="truncate text-2xl font-semibold tracking-tight">
						Search logs
					</h1>
					<p className="mt-1 text-base/6 text-muted-foreground sm:text-sm/6">
						Stored logs from every container, merged into one timeline.
					</p>
				</header>

				{isHistoryEnabled ? (
					<section className="mt-6 flex flex-col lg:min-h-0 lg:flex-1">
						<LogViewer
							variant="page"
							containerName="All containers"
							viewState={logViewState}
							history={{}}
							historyOnly
						/>
					</section>
				) : (
					!isPending && (
						<p className="mt-6 max-w-prose text-base/6 text-muted-foreground sm:text-sm/6">
							Log persistence is off, so there are no stored logs to search.
						</p>
					)
				)}
			</main>
		</div>
	);
}
