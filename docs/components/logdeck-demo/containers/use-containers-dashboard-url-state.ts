import { useCallback, useMemo, useState } from "react";

import type { DateRange } from "react-day-picker";
import type { GroupByOption, SortDirection } from "./container-utils";

export function useContainersDashboardUrlState() {
  const [searchTerm, setSearchTerm] = useState("");
  const [stateFilter, setStateFilter] = useState("all");
  const [hostFilter, setHostFilter] = useState("all");
  const [sortDirection, setSortDirection] = useState<SortDirection>("desc");
  const [groupBy, setGroupBy] = useState<GroupByOption>("none");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [dateRangeState, setDateRangeState] = useState<DateRange | undefined>(
    undefined
  );

  const dateRange = useMemo(() => dateRangeState, [dateRangeState]);

  const setSearchTermAndReset = useCallback((value: string) => {
    setSearchTerm(value);
    setPage(1);
  }, []);

  const setStateFilterAndReset = useCallback((value: string) => {
    setStateFilter(value || "all");
    setPage(1);
  }, []);

  const setHostFilterAndReset = useCallback((value: string) => {
    setHostFilter(value || "all");
    setPage(1);
  }, []);

  const setGroupByAndReset = useCallback((value: GroupByOption) => {
    setGroupBy(value);
    setPage(1);
  }, []);

  const setDateRange = useCallback((range: DateRange | undefined) => {
    setDateRangeState(range);
    setPage(1);
  }, []);

  const clearDateRange = useCallback(() => {
    setDateRangeState(undefined);
    setPage(1);
  }, []);

  const setPageSizeAndReset = useCallback((value: number) => {
    setPageSize(Math.max(1, Math.floor(value)));
    setPage(1);
  }, []);

  const setSafePage = useCallback((value: number) => {
    setPage(Math.max(1, Math.floor(value)));
  }, []);

  return {
    searchTerm,
    setSearchTerm: setSearchTermAndReset,
    stateFilter,
    setStateFilter: setStateFilterAndReset,
    hostFilter,
    setHostFilter: setHostFilterAndReset,
    sortDirection,
    setSortDirection,
    groupBy,
    setGroupBy: setGroupByAndReset,
    dateRange,
    setDateRange,
    clearDateRange,
    page,
    setPage: setSafePage,
    pageSize,
    setPageSize: setPageSizeAndReset,
  };
}
