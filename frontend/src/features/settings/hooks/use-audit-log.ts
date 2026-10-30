import { keepPreviousData, useQuery } from "@tanstack/react-query";

import { type AuditLogParams, getAuditLog } from "../api/get-audit-log";
import { type AuditRange, auditSince } from "../components/audit-utils";

/**
 * One page of the audit log. The time range is turned into a start time on
 * every fetch, so "last hour" keeps sliding while the view stays open.
 */
export function useAuditLog(
	params: Omit<AuditLogParams, "since"> & { range: AuditRange },
) {
	return useQuery({
		queryKey: ["settings", "audit", params],
		queryFn: () => {
			const { range, ...rest } = params;
			return getAuditLog({ ...rest, since: auditSince(range, Date.now()) });
		},
		placeholderData: keepPreviousData,
		refetchInterval: 30_000,
		refetchOnWindowFocus: true,
	});
}
