import { Trash2Icon } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
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

import type { UpdateLogStoragePayload } from "../api/update-log-storage";
import { useUpdateLogStorage } from "../hooks/use-settings";
import type { LogStoreConfig } from "../types";
import { EnvBadge } from "./env-badge";
import { validateRetentionCaps } from "./log-storage-utils";
import { showResultToast } from "./mutation-toast";
import { SaveButton } from "./save-button";

const MB = 1024 * 1024;

// A container with nothing stored has zero timestamps, which would otherwise
// render as a 1970-era date.
function formatSpan(oldest: string, newest: string, storedBytes: number) {
	const from = Date.parse(oldest);
	const to = Date.parse(newest);
	if (storedBytes <= 0 || Number.isNaN(from) || Number.isNaN(to)) return "—";

	const format = (value: number) =>
		new Date(value).toLocaleDateString(undefined, {
			month: "short",
			day: "numeric",
		});
	return `${format(from)} → ${format(to)}`;
}

// The caps live in the settings payload, where each one carries its own source:
// a cap pinned by an environment variable is shown but cannot be edited here.
function RetentionCapsForm({ config }: { config: LogStoreConfig }) {
	const [perContainerMB, setPerContainerMB] = useState(
		String(config.perContainerMB),
	);
	const [totalMB, setTotalMB] = useState(String(config.totalMB));
	const updateMutation = useUpdateLogStorage();

	const perContainerIsEnv = config.perContainerMBSource === "env";
	const totalIsEnv = config.totalMBSource === "env";
	const hasChanges =
		(!perContainerIsEnv && perContainerMB !== String(config.perContainerMB)) ||
		(!totalIsEnv && totalMB !== String(config.totalMB));

	function handleSave() {
		const error = validateRetentionCaps(perContainerMB, totalMB);
		if (error) {
			toast.error(error);
			return;
		}
		const payload: UpdateLogStoragePayload = {};
		if (!perContainerIsEnv) payload.perContainerMB = Number(perContainerMB);
		if (!totalIsEnv) payload.totalMB = Number(totalMB);
		updateMutation.mutate(payload, showResultToast);
	}

	return (
		<div className="space-y-3">
			<div className="grid gap-4 sm:grid-cols-2">
				<div className="space-y-1.5">
					<div className="flex flex-wrap items-center gap-2">
						<Label htmlFor="log-store-per-container">
							Per-container cap (MB)
						</Label>
						{perContainerIsEnv && <EnvBadge />}
					</div>
					<Input
						id="log-store-per-container"
						type="number"
						min={1}
						className="h-8"
						value={perContainerMB}
						disabled={perContainerIsEnv}
						onChange={(e) => setPerContainerMB(e.target.value)}
					/>
				</div>
				<div className="space-y-1.5">
					<div className="flex flex-wrap items-center gap-2">
						<Label htmlFor="log-store-total">Total cap (MB)</Label>
						{totalIsEnv && <EnvBadge />}
					</div>
					<Input
						id="log-store-total"
						type="number"
						min={1}
						className="h-8"
						value={totalMB}
						disabled={totalIsEnv}
						onChange={(e) => setTotalMB(e.target.value)}
					/>
				</div>
			</div>
			<p className="text-xs text-muted-foreground">
				Lowering a cap evicts the oldest stored logs on the next retention pass.
			</p>
			{hasChanges && (
				<SaveButton isPending={updateMutation.isPending} onClick={handleSave} />
			)}
		</div>
	);
}

interface LogStorageSectionProps {
	/** Absent on servers that do not report the log store settings. */
	config?: LogStoreConfig;
}

export function LogStorageSection({ config }: LogStorageSectionProps) {
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
	if (!isEnabled) {
		return (
			<Card>
				<CardHeader>
					<CardTitle>Log storage</CardTitle>
					<CardDescription>
						Log persistence is disabled, so no logs are stored on disk. Enable
						it in the environment and restart LogDeck to keep logs readable
						after a container is removed.
					</CardDescription>
				</CardHeader>
			</Card>
		);
	}

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

				{config && (
					<RetentionCapsForm
						key={`${config.perContainerMB}-${config.totalMB}`}
						config={config}
					/>
				)}

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
									<TableCell className="whitespace-nowrap text-xs text-muted-foreground">
										{formatSpan(
											container.oldestTs,
											container.newestTs,
											container.storedBytes,
										)}
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
