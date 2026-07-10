import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";

import { ResourceNav } from "@/features/resources/components/resource-nav";
import type { GetContainersResponse } from "../api/get-containers";
import { useContainerActions } from "../hooks/use-container-actions";
import { useContainerStats } from "../hooks/use-container-stats";
import { useContainersDashboardUrlState } from "../hooks/use-containers-dashboard-url-state";
import { useHostsStats } from "../hooks/use-hosts-stats";
import { useLiveContainersQuery } from "../hooks/use-live-containers-query";
import { useSystemUsageHistory } from "../hooks/use-stats-history";
import { useSystemStats } from "../hooks/use-system-stats";
import type { ContainerInfo } from "../types";
import { ConfirmActionDialog } from "./confirm-action-dialog";
import { getInitialStateCounts, groupByCompose } from "./container-utils";
import { ContainersLogsSheet } from "./containers-logs-sheet";
import { ContainersPagination } from "./containers-pagination";
import { ContainersStateSummary } from "./containers-state-summary";
import { ContainersSummaryCards } from "./containers-summary-cards";
import { ContainersTable } from "./containers-table";
import { ContainersToolbar } from "./containers-toolbar";

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

	useEffect(() => {
		for (const he of hostErrors) {
			toast.warning(`Could not reach host "${he.host}"`, {
				id: `host-error-${he.host}`,
				description: "Containers from this host could not be loaded.",
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

	const {
		pendingAction,
		pendingComposeAction,
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
		return Array.from(unique).sort();
	}, [containers]);

	const filteredContainers = useMemo(() => {
		const filtered = containers.filter((container) =>
			matchesFilters(container, { includeStateFilter: true }),
		);

		return filtered.sort((a, b) =>
			sortDirection === "desc" ? b.created - a.created : a.created - b.created,
		);
	}, [containers, matchesFilters, sortDirection]);

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

	const stateCounts = useMemo(() => {
		const counts = getInitialStateCounts();

		// Filter by host, search, and date - but NOT by state filter
		// This way state counts reflect the current host selection
		containers.forEach((container) => {
			if (matchesFilters(container, { includeStateFilter: false })) {
				const state = container.state.toLowerCase();
				if (state === "running") counts.running++;
				else if (state === "exited") counts.exited++;
				else if (state === "paused") counts.paused++;
				else if (state === "restarting") counts.restarting++;
				else if (state === "dead") counts.dead++;
				else counts.other++;
			}
		});

		return counts;
	}, [containers, matchesFilters]);

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

				<ContainersStateSummary stateCounts={stateCounts} />

				<ContainersTable
					isLoading={isLoading}
					isError={isError}
					error={error}
					groupBy={groupBy}
					filteredContainers={filteredContainers}
					groupedItems={groupedItems}
					pageItems={pageItems}
					pendingAction={pendingAction}
					pendingComposeAction={pendingComposeAction}
					isReadOnly={isReadOnly}
					statsMap={statsMap}
					statsHistory={statsHistory}
					onStart={startContainerAction}
					onStop={stopContainerAction}
					onRestart={restartContainerAction}
					onDelete={deleteContainerAction}
					onComposeAction={composeAction}
					onViewLogs={handleViewLogs}
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
