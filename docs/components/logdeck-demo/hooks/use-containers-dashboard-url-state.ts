import { useCallback, useState } from "react";

import type { DateRange } from "react-day-picker";
import type {
	GroupByOption,
	SortDirection,
} from "../containers/container-utils";

/**
 * Same return shape as the real app's URL-backed dashboard state, but held in
 * component state: the demo lives inside a docs page and should not rewrite
 * the docs site's URL.
 */
export function useContainersDashboardUrlState() {
	const [searchTerm, setSearchTermState] = useState("");
	const [stateFilter, setStateFilterState] = useState("all");
	const [hostFilter, setHostFilterState] = useState("all");
	const [sortDirection, setSortDirection] = useState<SortDirection>("desc");
	const [groupBy, setGroupByState] = useState<GroupByOption>("none");
	const [dateRange, setDateRangeState] = useState<DateRange | undefined>(
		undefined,
	);
	const [page, setPageState] = useState(1);
	const [pageSize, setPageSizeState] = useState(10);

	const setSearchTerm = useCallback((value: string) => {
		setSearchTermState(value);
		setPageState(1);
	}, []);

	const setStateFilter = useCallback((value: string) => {
		setStateFilterState(value || "all");
		setPageState(1);
	}, []);

	const setHostFilter = useCallback((value: string) => {
		setHostFilterState(value || "all");
		setPageState(1);
	}, []);

	const setGroupBy = useCallback((value: GroupByOption) => {
		setGroupByState(value);
		setPageState(1);
	}, []);

	const setDateRange = useCallback((range: DateRange | undefined) => {
		setDateRangeState(range);
		setPageState(1);
	}, []);

	const clearDateRange = useCallback(() => {
		setDateRangeState(undefined);
		setPageState(1);
	}, []);

	const setPage = useCallback((value: number) => {
		setPageState(Math.max(1, Math.floor(value)));
	}, []);

	const setPageSize = useCallback((value: number) => {
		setPageSizeState(Math.max(1, Math.floor(value)));
		setPageState(1);
	}, []);

	return {
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
		page,
		setPage,
		pageSize,
		setPageSize,
	};
}
