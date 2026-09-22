import { useState } from "react";
import { toast } from "sonner";
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { SearchIcon, Trash2Icon } from "@/components/ui/icons";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { formatBytes } from "@/features/containers/components/container-utils";
import { ContainersPagination } from "@/features/containers/components/containers-pagination";
import { Meter } from "@/features/containers/components/meter";
import type { PurgeHistoryTarget } from "@/features/containers/components/purge-history-dialog";
import { PurgeHistoryDialog } from "@/features/containers/components/purge-history-dialog";
import {
	useDeleteHistoryContainer,
	useDeleteRemovedHistory,
} from "@/features/containers/hooks/use-delete-history-container";
import { useDebouncedValue } from "@/features/containers/hooks/use-debounced-value";
import { useStoredContainersPage } from "@/features/containers/hooks/use-history-containers";
import { useHistoryStatus } from "@/features/containers/hooks/use-history-status";

import type { UpdateLogStoragePayload } from "../api/update-log-storage";
import { useUpdateLogStorage } from "../hooks/use-settings";
import type { LogStoreConfig } from "../types";
import {
	validateRemovedDays,
	validateRetentionCaps,
} from "./log-storage-utils";
import { showResultToast } from "./mutation-toast";
import { SaveButton } from "./save-button";
import {
	EnvBadge,
	ErrorNote,
	Field,
	Note,
	SettingsSection,
	SettingsSubsection,
	SettingsTable,
	TBody,
	Td,
	Th,
	THead,
} from "./settings-ui";

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

/** Label beside its reading — the same strip the container header uses. */
function Stat({
	label,
	value,
	children,
}: {
	label: string;
	value: string;
	children?: React.ReactNode;
}) {
	return (
		<div className="flex min-w-0 items-center gap-2.5">
			<span className="shrink-0 font-mono text-[0.625rem] uppercase tracking-wide text-muted-foreground">
				{label}
			</span>
			<span className="truncate text-sm font-medium tabular-nums">{value}</span>
			{children}
		</div>
	);
}

// The caps live in the settings payload, where each one carries its own source:
// a cap pinned by an environment variable is shown but cannot be edited here.
function RetentionCapsForm({ config }: { config: LogStoreConfig }) {
	const [perContainerMB, setPerContainerMB] = useState(
		String(config.perContainerMB),
	);
	const [totalMB, setTotalMB] = useState(String(config.totalMB));
	const [removedDays, setRemovedDays] = useState(String(config.removedDays));
	const updateMutation = useUpdateLogStorage();

	const perContainerIsEnv = config.perContainerMBSource === "env";
	const totalIsEnv = config.totalMBSource === "env";
	const removedIsEnv = config.removedDaysSource === "env";
	const hasChanges =
		(!perContainerIsEnv && perContainerMB !== String(config.perContainerMB)) ||
		(!totalIsEnv && totalMB !== String(config.totalMB)) ||
		(!removedIsEnv && removedDays !== String(config.removedDays));

	function handleSave() {
		const error =
			validateRetentionCaps(perContainerMB, totalMB) ??
			validateRemovedDays(removedDays);
		if (error) {
			toast.error(error);
			return;
		}
		const payload: UpdateLogStoragePayload = {};
		if (!perContainerIsEnv) payload.perContainerMB = Number(perContainerMB);
		if (!totalIsEnv) payload.totalMB = Number(totalMB);
		if (!removedIsEnv) payload.removedDays = Number(removedDays);
		updateMutation.mutate(payload, showResultToast);
	}

	return (
		<SettingsSubsection title="Retention">
			<div className="space-y-4">
				<div className="grid max-w-2xl gap-4 sm:grid-cols-3">
					<Field
						id="log-store-per-container"
						label={
							<span className="inline-flex flex-wrap items-center gap-2">
								Per container (MB)
								{perContainerIsEnv && <EnvBadge />}
							</span>
						}
					>
						<Input
							id="log-store-per-container"
							name="perContainerMB"
							type="number"
							min={1}
							value={perContainerMB}
							disabled={perContainerIsEnv}
							onChange={(e) => setPerContainerMB(e.target.value)}
						/>
					</Field>
					<Field
						id="log-store-total"
						label={
							<span className="inline-flex flex-wrap items-center gap-2">
								Total (MB)
								{totalIsEnv && <EnvBadge />}
							</span>
						}
					>
						<Input
							id="log-store-total"
							name="totalMB"
							type="number"
							min={1}
							value={totalMB}
							disabled={totalIsEnv}
							onChange={(e) => setTotalMB(e.target.value)}
						/>
					</Field>
					<Field
						id="log-store-removed-days"
						label={
							<span className="inline-flex flex-wrap items-center gap-2">
								Keep removed for (days)
								{removedIsEnv && <EnvBadge />}
							</span>
						}
					>
						<Input
							id="log-store-removed-days"
							name="removedDays"
							type="number"
							min={0}
							value={removedDays}
							disabled={removedIsEnv}
							onChange={(e) => setRemovedDays(e.target.value)}
						/>
					</Field>
				</div>
				<Note>
					Lowering a cap evicts the oldest stored logs on the next retention
					pass. A removed container's logs are dropped once it has been gone
					that many days; 0 keeps them until a cap evicts them.
				</Note>
				{hasChanges && (
					<SaveButton
						isPending={updateMutation.isPending}
						onClick={handleSave}
					/>
				)}
			</div>
		</SettingsSubsection>
	);
}

interface LogStorageSectionProps {
	/** Absent on servers that do not report the log store settings. */
	config?: LogStoreConfig;
}

export function LogStorageSection({ config }: LogStorageSectionProps) {
	const { data: status } = useHistoryStatus();
	const isEnabled = status?.enabled === true;
	const [search, setSearch] = useState("");
	const [page, setPage] = useState(1);
	const [pageSize, setPageSize] = useState(10);
	const needle = useDebouncedValue(search.trim(), 250);
	const {
		data: stored,
		isLoading,
		error,
	} = useStoredContainersPage(
		{
			search: needle,
			sort: "size",
			limit: pageSize,
			offset: (page - 1) * pageSize,
		},
		isEnabled,
	);
	const purgeHistory = useDeleteHistoryContainer();
	const purgeRemoved = useDeleteRemovedHistory();
	const [purgeTarget, setPurgeTarget] = useState<PurgeHistoryTarget | null>(
		null,
	);
	const [isPurgeRemovedOpen, setIsPurgeRemovedOpen] = useState(false);

	// Persistence is off: there is nothing to report or reclaim.
	if (!isEnabled) {
		return (
			<SettingsSection
				title="Log storage"
				description="Log persistence is off, so nothing is kept on disk. Enable it in the environment and restart LogDeck to keep logs readable after a container is removed."
			/>
		);
	}

	const pageItems = stored?.containers ?? [];
	const total = stored?.total ?? 0;
	const removedCount = stored?.removedCount ?? 0;
	const totalPages = Math.max(1, Math.ceil(total / pageSize));
	const startIndex = (page - 1) * pageSize;
	// Deleting the last row of the last page leaves the page past the end.
	if (stored && page > totalPages) setPage(totalPages);

	const usedBytes = status.dbSizeBytes ?? 0;
	const totalBytes = status.totalMB ? status.totalMB * MB : 0;

	function handleConfirmPurge() {
		if (!purgeTarget) return;
		purgeHistory.mutate(
			{ name: purgeTarget.name, host: purgeTarget.host },
			{ onSettled: () => setPurgeTarget(null) },
		);
	}

	return (
		<SettingsSection
			title="Log storage"
			description="Logs are kept on disk so they stay readable after a container is removed. Deleting a container's history frees its space at once and cannot be undone."
		>
			<div className="space-y-8">
				<div className="flex flex-wrap items-center gap-x-8 gap-y-3">
					<Stat label="Used" value={formatBytes(usedBytes)}>
						{totalBytes > 0 && (
							<>
								<Meter used={usedBytes} limit={totalBytes} className="w-24" />
								<span className="shrink-0 text-sm text-muted-foreground tabular-nums">
									of {status.totalMB} MB
								</span>
							</>
						)}
					</Stat>
					{status.perContainerMB ? (
						<Stat label="Per container" value={`${status.perContainerMB} MB`} />
					) : null}
					{stored && (
						<Stat label="Stored" value={String(stored.storedCount)}>
							<span className="shrink-0 text-sm text-muted-foreground">
								{stored.storedCount === 1 ? "container" : "containers"}
							</span>
						</Stat>
					)}
				</div>

				{config && (
					<RetentionCapsForm
						key={`${config.perContainerMB}-${config.totalMB}-${config.removedDays}`}
						config={config}
					/>
				)}

				<SettingsSubsection
					title="Stored containers"
					action={
						removedCount > 0 && (
							<Button
								variant="ghost"
								size="sm"
								disabled={purgeRemoved.isPending}
								onClick={() => setIsPurgeRemovedOpen(true)}
								className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
							>
								<Trash2Icon className="size-4" />
								Delete removed ({removedCount})
							</Button>
						)
					}
				>
					<div className="space-y-4">
						{(stored?.storedCount ?? 0) > 0 && (
							<div className="relative max-w-xs">
								<SearchIcon className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
								<Input
									type="search"
									name="search"
									aria-label="Filter stored containers"
									placeholder="Filter by name, host or project"
									value={search}
									onChange={(e) => {
										setSearch(e.target.value);
										setPage(1);
									}}
									className="pl-8"
								/>
							</div>
						)}

						{isLoading && <Spinner className="size-4" />}
						{error && (
							<ErrorNote>
								Failed to load stored containers: {error.message}
							</ErrorNote>
						)}

						{!isLoading && !error && total === 0 && (
							<Note>
								{needle
									? "No stored containers match."
									: "No container logs stored yet."}
							</Note>
						)}

						{pageItems.length > 0 && (
							<SettingsTable>
								<THead>
									<Th>Container</Th>
									<Th>Host</Th>
									<Th>Project</Th>
									<Th>Stored</Th>
									<Th>Time span</Th>
									<Th className="text-right">
										<span className="sr-only">Actions</span>
									</Th>
								</THead>
								<TBody>
									{pageItems.map((container) => (
										<tr key={`${container.host}/${container.name}`}>
											<Td className="font-medium">
												<div className="flex items-center gap-2">
													{container.name}
													{container.removed && (
														<Badge className="border-transparent bg-muted font-normal text-muted-foreground">
															Removed
														</Badge>
													)}
												</div>
											</Td>
											<Td className="text-muted-foreground">
												{container.host}
											</Td>
											<Td className="text-muted-foreground">
												{container.composeProject ?? "—"}
											</Td>
											<Td className="font-mono tabular-nums">
												{formatBytes(container.storedBytes)}
											</Td>
											<Td className="text-muted-foreground tabular-nums">
												{formatSpan(
													container.oldestTs,
													container.newestTs,
													container.storedBytes,
												)}
											</Td>
											<Td className="text-right">
												<Button
													variant="ghost"
													size="icon-sm"
													disabled={purgeHistory.isPending}
													onClick={() =>
														setPurgeTarget({
															name: container.name,
															host: container.host,
															removed: container.removed,
														})
													}
													className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
													aria-label={`Delete stored logs for ${container.name}`}
												>
													<Trash2Icon className="size-4" />
												</Button>
											</Td>
										</tr>
									))}
								</TBody>
							</SettingsTable>
						)}

						{total > pageSize && (
							<ContainersPagination
								totalItems={total}
								startIndex={startIndex + 1}
								endIndex={startIndex + pageItems.length}
								page={page}
								totalPages={totalPages}
								pageSize={pageSize}
								onPageChange={setPage}
								onPageSizeChange={(size) => {
									setPageSize(size);
									setPage(1);
								}}
							/>
						)}
					</div>
				</SettingsSubsection>
			</div>

			<AlertDialog
				open={isPurgeRemovedOpen}
				onOpenChange={setIsPurgeRemovedOpen}
			>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>
							Delete logs of removed containers?
						</AlertDialogTitle>
						<AlertDialogDescription>
							This permanently deletes the stored history of {removedCount}{" "}
							{removedCount === 1 ? "container" : "containers"} that no longer
							exist on any host. Those logs cannot be recovered afterwards.
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>Cancel</AlertDialogCancel>
						<AlertDialogAction
							onClick={() => purgeRemoved.mutate()}
							className="bg-destructive text-white hover:bg-destructive/90"
						>
							Delete logs
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>

			<PurgeHistoryDialog
				target={purgeTarget}
				isPending={purgeHistory.isPending}
				onConfirm={handleConfirmPurge}
				onOpenChange={(open) => {
					if (!open) setPurgeTarget(null);
				}}
			/>
		</SettingsSection>
	);
}
