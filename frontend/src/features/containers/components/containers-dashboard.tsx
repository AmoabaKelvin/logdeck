import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";

import { ResourceNav } from "@/features/resources/components/resource-nav";
import type { GetContainersResponse } from "../api/get-containers";
import { useContainerActions } from "../hooks/use-container-actions";
import { useContainerStats } from "../hooks/use-container-stats";
import { useContainersDashboardUrlState } from "../hooks/use-containers-dashboard-url-state";
import { useDeleteHistoryContainer } from "../hooks/use-delete-history-container";
import { useHistoryContainers } from "../hooks/use-history-containers";
import { useHistoryStatus } from "../hooks/use-history-status";
import { useHostsStats } from "../hooks/use-hosts-stats";
import { useLiveContainersQuery } from "../hooks/use-live-containers-query";
import { useSystemUsageHistory } from "../hooks/use-stats-history";
import { useSystemStats } from "../hooks/use-system-stats";
import type { ContainerInfo } from "../types";
import { ConfirmActionDialog } from "./confirm-action-dialog";
import {
	countContainerStates,
	getContainerUrlIdentifier,
	groupByCompose,
	REMOVED_STATE,
	selectVisibleContainers,
	synthesizeRemovedContainers,
} from "./container-utils";
import { ContainersLogsSheet } from "./containers-logs-sheet";
import { ContainersPagination } from "./containers-pagination";
import { ContainersStateSummary } from "./containers-state-summary";
import { ContainersSummaryCards } from "./containers-summary-cards";
import { ContainersTable } from "./containers-table";
import { ContainersToolbar } from "./containers-toolbar";
import type { PurgeHistoryTarget } from "./purge-history-dialog";
import { PurgeHistoryDialog } from "./purge-history-dialog";

export function ContainersDashboard() {
	const queryClient = useQueryClient();
	const { data, error, isError, isFetching, isLoading, refetch } =
		useLiveContainersQuery();
	const { data: systemStats } = useSystemStats();
	const systemHistory = useSystemUsageHistory(systemStats?.usage);
	const { statsMap, statsHistory } = useContainerStats();

	const containers = data?.containers ?? [];
	const isReadOnly = data?.readOnly ?? false;
	const hosts = data?.hosts ?? [];
	const hostErrors = data?.hostErrors ?? [];
	const { data: hostsStatsData } = useHostsStats(hosts.length > 1);

	// Containers that were removed but still have stored logs show up as an extra
	// dashboard state. Without log persistence there is nothing to synthesize.
	const { data: historyStatus } = useHistoryStatus();
	const isHistoryEnabled = historyStatus?.enabled === true;
	const { data: storedContainers } = useHistoryContainers(isHistoryEnabled);
	const removedContainers = useMemo(
		() =>
			isHistoryEnabled && storedContainers
				? synthesizeRemovedContainers(storedContainers, containers)
				: [],
		[isHistoryEnabled, storedContainers, containers],
	);

	useEffect(() => {
		for (const he of hostErrors) {
			toast.warning(`Could not reach host "${he.host}"`, {
				id: `host-error-${he.host}`,
				description:
					he.message || "Containers from this host could not be loaded.",
				duration: 8000,
			});
		}
	}, [hostErrors]);

	const hostInfo = {
		hostname: systemStats?.hostInfo.hostname ?? "Loading...",
		os: systemStats?.hostInfo.platform ?? "Unknown",
		kernel: systemStats?.hostInfo.kernelVersion ?? "Unknown",
	};

	const systemUsage = {
		cpu: Math.round(systemStats?.usage.cpuPercent ?? 0),
		memory: Math.round(systemStats?.usage.memoryPercent ?? 0),
	};

	const {
		searchTerm,
		setSearchTerm,
		stateFilter,
		setStateFilter,
		hostFilter,
		setHostFilter,
		sortDirection,
		setSortDirection,
		groupBy,
		setGroupBy,
		dateRange,
		setDateRange,
		clearDateRange,
		pageSize,
		setPageSize,
		page,
		setPage,
	} = useContainersDashboardUrlState();
	const [selectedContainer, setSelectedContainer] =
		useState<ContainerInfo | null>(null);
	const [isLogsSheetOpen, setIsLogsSheetOpen] = useState(false);
	const [purgeTarget, setPurgeTarget] = useState<PurgeHistoryTarget | null>(
		null,
	);
	const purgeHistory = useDeleteHistoryContainer();

	const {
		pendingActions,
		pendingComposeActions,
		confirmAction,
		isConfirmActionPending,
		startContainerAction,
		stopContainerAction,
		restartContainerAction,
		deleteContainerAction,
		composeAction,
		confirmPendingAction,
		handleConfirmDialogOpenChange,
	} = useContainerActions(refetch);

	const matchesFilters = useMemo(() => {
		const normalizedSearch = searchTerm.trim().toLowerCase();

		return (
			container: ContainerInfo,
			options: { includeStateFilter?: boolean } = {},
		) => {
			const matchesSearch =
				!normalizedSearch ||
				container.id.toLowerCase().startsWith(normalizedSearch) ||
				container.image.toLowerCase().includes(normalizedSearch) ||
				container.names.some((name) =>
					name.toLowerCase().includes(normalizedSearch),
				);

			const matchesHost = hostFilter === "all" || container.host === hostFilter;

			const containerDate = new Date(container.created * 1000);
			const matchesDateRange =
				!dateRange ||
				(dateRange.from &&
					dateRange.to &&
					containerDate >= dateRange.from &&
					containerDate <= dateRange.to) ||
				(dateRange.from && !dateRange.to && containerDate >= dateRange.from) ||
				(!dateRange.from && dateRange.to && containerDate <= dateRange.to);

			const matchesState = options.includeStateFilter
				? stateFilter === "all" || container.state.toLowerCase() === stateFilter
				: true;

			return matchesSearch && matchesHost && matchesDateRange && matchesState;
		};
	}, [searchTerm, hostFilter, dateRange, stateFilter]);

	const availableStates = useMemo(() => {
		const unique = new Set<string>();
		containers.forEach((container) => {
			if (container.state) {
				unique.add(container.state.toLowerCase());
			}
		});
		if (removedContainers.length > 0) {
			unique.add(REMOVED_STATE);
		}
		return Array.from(unique).sort();
	}, [containers, removedContainers]);

	const filteredContainers = useMemo(() => {
		const filtered = selectVisibleContainers(
			containers,
			removedContainers,
			stateFilter,
		).filter((container) =>
			matchesFilters(container, { includeStateFilter: true }),
		);

		return filtered.sort((a, b) =>
			sortDirection === "desc" ? b.created - a.created : a.created - b.created,
		);
	}, [
		containers,
		removedContainers,
		stateFilter,
		matchesFilters,
		sortDirection,
	]);

	const totalPages =
		filteredContainers.length === 0
			? 1
			: Math.ceil(filteredContainers.length / pageSize);

	useEffect(() => {
		if (page > totalPages) {
			setPage(totalPages);
		}
	}, [page, totalPages, setPage]);

	const startIndex = filteredContainers.length ? (page - 1) * pageSize + 1 : 0;
	const endIndex = filteredContainers.length
		? Math.min(page * pageSize, filteredContainers.length)
		: 0;

	const pageItems = useMemo(() => {
		const offset = (page - 1) * pageSize;
		return filteredContainers.slice(offset, offset + pageSize);
	}, [filteredContainers, page, pageSize]);

	const groupedItems = useMemo(() => {
		if (groupBy !== "compose") {
			return null;
		}
		return groupByCompose(pageItems);
	}, [pageItems, groupBy]);

	// Filter by host, search, and date - but NOT by state filter
	// This way state counts reflect the current host selection
	const stateCounts = useMemo(
		() =>
			countContainerStates(containers, removedContainers, (container) =>
				Boolean(matchesFilters(container, { includeStateFilter: false })),
			),
		[containers, removedContainers, matchesFilters],
	);

	const handleViewLogs = (container: ContainerInfo) => {
		setSelectedContainer(container);
		setIsLogsSheetOpen(true);
	};

	const handleLogsSheetOpenChange = (open: boolean) => {
		setIsLogsSheetOpen(open);
		if (!open) {
			setSelectedContainer(null);
		}
	};

	const handleConfirmPurge = () => {
		if (!purgeTarget) return;
		purgeHistory.mutate(
			{ name: purgeTarget.name, host: purgeTarget.host },
			{ onSettled: () => setPurgeTarget(null) },
		);
	};

	const handleContainerRecreated = async (newContainerId: string) => {
		await queryClient.refetchQueries({
			queryKey: ["containers"],
			exact: false,
		});

		const updatedData = queryClient.getQueryData<GetContainersResponse>([
			"containers",
		]);
		const newContainer = updatedData?.containers?.find(
			(c) => c.id === newContainerId,
		);

		if (newContainer) {
			setSelectedContainer(newContainer);
		}
	};

	return (
		<div className="w-full space-y-8">
			<ResourceNav />
			<ContainersSummaryCards
				totalContainers={containers.length}
				hostInfo={hostInfo}
				systemUsage={systemUsage}
				hostsStats={hostsStatsData?.hosts}
				systemHistory={systemHistory}
			/>

			<section className="space-y-4">
				<ContainersToolbar
					searchTerm={searchTerm}
					onSearchChange={setSearchTerm}
					stateFilter={stateFilter}
					onStateFilterChange={setStateFilter}
					availableStates={availableStates}
					hostFilter={hostFilter}
					onHostFilterChange={setHostFilter}
					availableHosts={hosts}
					sortDirection={sortDirection}
					onSortDirectionChange={setSortDirection}
					groupBy={groupBy}
					onGroupByChange={setGroupBy}
					dateRange={dateRange}
					onDateRangeChange={setDateRange}
					onDateRangeClear={clearDateRange}
					onRefresh={refetch}
					isFetching={isFetching}
				/>

				<ContainersStateSummary
					stateCounts={stateCounts}
					stateFilter={stateFilter}
					onStateFilterChange={setStateFilter}
				/>

				<ContainersTable
					isLoading={isLoading}
					isError={isError}
					error={error}
					groupBy={groupBy}
					emptyMessage={
						stateFilter === REMOVED_STATE
							? "No removed containers with stored logs."
							: "No containers found."
					}
					filteredContainers={filteredContainers}
					groupedItems={groupedItems}
					pageItems={pageItems}
					pendingActions={pendingActions}
					pendingComposeActions={pendingComposeActions}
					isReadOnly={isReadOnly}
					statsMap={statsMap}
					statsHistory={statsHistory}
					onStart={startContainerAction}
					onStop={stopContainerAction}
					onRestart={restartContainerAction}
					onDelete={deleteContainerAction}
					onComposeAction={composeAction}
					onViewLogs={handleViewLogs}
					onPurgeHistory={(container) =>
						setPurgeTarget({
							name: getContainerUrlIdentifier(container),
							host: container.host,
							removed: true,
						})
					}
					onRetry={() => {
						void refetch();
					}}
				/>

				<ContainersPagination
					totalItems={filteredContainers.length}
					startIndex={startIndex}
					endIndex={endIndex}
					page={page}
					totalPages={totalPages}
					pageSize={pageSize}
					onPageChange={setPage}
					onPageSizeChange={setPageSize}
				/>
			</section>

			<ConfirmActionDialog
				action={confirmAction}
				isPending={isConfirmActionPending}
				onConfirm={() => {
					void confirmPendingAction();
				}}
				onOpenChange={handleConfirmDialogOpenChange}
			/>

			<PurgeHistoryDialog
				target={purgeTarget}
				isPending={purgeHistory.isPending}
				onConfirm={handleConfirmPurge}
				onOpenChange={(open) => {
					if (!open) setPurgeTarget(null);
				}}
			/>

			<ContainersLogsSheet
				container={selectedContainer}
				isOpen={isLogsSheetOpen}
				isReadOnly={isReadOnly}
				onOpenChange={handleLogsSheetOpenChange}
				onContainerRecreated={handleContainerRecreated}
			/>
		</div>
	);
}
