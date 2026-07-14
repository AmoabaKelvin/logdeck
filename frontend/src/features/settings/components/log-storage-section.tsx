import { Trash2Icon } from "lucide-react";
import { useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { Spinner } from "@/components/ui/spinner";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import {
	formatBytes,
	sortStoredContainersBySize,
} from "@/features/containers/components/container-utils";
import type { PurgeHistoryTarget } from "@/features/containers/components/purge-history-dialog";
import { PurgeHistoryDialog } from "@/features/containers/components/purge-history-dialog";
import { useDeleteHistoryContainer } from "@/features/containers/hooks/use-delete-history-container";
import { useHistoryContainers } from "@/features/containers/hooks/use-history-containers";
import { useHistoryStatus } from "@/features/containers/hooks/use-history-status";

const MB = 1024 * 1024;

function formatTimestamp(value: string) {
	const parsed = Date.parse(value);
	if (Number.isNaN(parsed)) return "—";
	return new Date(parsed).toLocaleString(undefined, {
		dateStyle: "short",
		timeStyle: "short",
	});
}

export function LogStorageSection() {
	const { data: status } = useHistoryStatus();
	const isEnabled = status?.enabled === true;
	const {
		data: storedContainers,
		isLoading,
		error,
	} = useHistoryContainers(isEnabled);
	const purgeHistory = useDeleteHistoryContainer();
	const [purgeTarget, setPurgeTarget] = useState<PurgeHistoryTarget | null>(
		null,
	);

	// Persistence is off: there is nothing to report or reclaim.
	if (!isEnabled) return null;

	const containers = sortStoredContainersBySize(storedContainers ?? []);
	const usedBytes = status.dbSizeBytes ?? 0;
	const totalBytes = status.totalMB ? status.totalMB * MB : 0;
	const usedPercent = totalBytes
		? Math.min(100, Math.round((usedBytes / totalBytes) * 100))
		: 0;

	function handleConfirmPurge() {
		if (!purgeTarget) return;
		purgeHistory.mutate(
			{ name: purgeTarget.name, host: purgeTarget.host },
			{ onSettled: () => setPurgeTarget(null) },
		);
	}

	return (
		<Card>
			<CardHeader>
				<CardTitle>Log storage</CardTitle>
				<CardDescription>
					Logs are persisted on disk so they stay readable after a container is
					removed. Deleting a container's history frees its space immediately
					and cannot be undone.
				</CardDescription>
			</CardHeader>
			<CardContent className="space-y-6">
				<div className="space-y-2">
					<div className="flex items-baseline justify-between text-sm">
						<span className="font-medium">
							{formatBytes(usedBytes)}
							{status.totalMB ? ` of ${status.totalMB} MB used` : " used"}
						</span>
						{status.perContainerMB ? (
							<span className="text-xs text-muted-foreground">
								{status.perContainerMB} MB cap per container
							</span>
						) : null}
					</div>
					{status.totalMB ? (
						<div className="h-2 w-full overflow-hidden rounded-full bg-muted">
							<div
								className="h-full rounded-full bg-primary transition-all"
								style={{ width: `${usedPercent}%` }}
							/>
						</div>
					) : null}
				</div>

				{isLoading && <Spinner className="size-4" />}
				{error && (
					<p className="text-sm text-destructive">
						Failed to load stored containers: {error.message}
					</p>
				)}

				{!isLoading && !error && containers.length === 0 && (
					<p className="text-sm text-muted-foreground">
						No container logs stored yet.
					</p>
				)}

				{containers.length > 0 && (
					<Table>
						<TableHeader>
							<TableRow>
								<TableHead>Container</TableHead>
								<TableHead>Host</TableHead>
								<TableHead>Project</TableHead>
								<TableHead>Stored</TableHead>
								<TableHead>Time span</TableHead>
								<TableHead className="text-right">Actions</TableHead>
							</TableRow>
						</TableHeader>
						<TableBody>
							{containers.map((container) => (
								<TableRow key={`${container.host}/${container.name}`}>
									<TableCell className="font-medium">
										<div className="flex items-center gap-2">
											{container.name}
											{container.removed && (
												<Badge
													variant="outline"
													className="text-muted-foreground text-[10px] px-1.5 h-4"
												>
													Removed
												</Badge>
											)}
										</div>
									</TableCell>
									<TableCell className="text-xs text-muted-foreground">
										{container.host}
									</TableCell>
									<TableCell className="text-xs text-muted-foreground">
										{container.composeProject ?? "—"}
									</TableCell>
									<TableCell className="font-mono text-xs">
										{formatBytes(container.storedBytes)}
									</TableCell>
									<TableCell className="text-xs text-muted-foreground">
										{formatTimestamp(container.oldestTs)} →{" "}
										{formatTimestamp(container.newestTs)}
									</TableCell>
									<TableCell className="text-right">
										<Button
											variant="ghost"
											size="icon"
											disabled={purgeHistory.isPending}
											onClick={() =>
												setPurgeTarget({
													name: container.name,
													host: container.host,
													removed: container.removed,
												})
											}
											className="size-8 text-destructive hover:text-destructive"
											aria-label={`Delete stored logs for ${container.name}`}
										>
											<Trash2Icon className="size-4" />
										</Button>
									</TableCell>
								</TableRow>
							))}
						</TableBody>
					</Table>
				)}
			</CardContent>

			<PurgeHistoryDialog
				target={purgeTarget}
				isPending={purgeHistory.isPending}
				onConfirm={handleConfirmPurge}
				onOpenChange={(open) => {
					if (!open) setPurgeTarget(null);
				}}
			/>
		</Card>
	);
}
