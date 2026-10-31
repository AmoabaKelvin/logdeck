import { formatBytes } from "@/features/containers/components/container-utils";

import type { BackupContents } from "../types";

function count(n: number, one: string, many: string): string | null {
	return n > 0 ? `${n} ${n === 1 ? one : many}` : null;
}

/** One line naming what a backup holds, skipping what it has none of. */
export function describeContents(c: BackupContents): string {
	const hosts = c.dockerHosts + c.coolifyHosts + c.dokployHosts;
	const parts = [
		count(hosts, "connection", "connections"),
		count(c.alertRules, "alert rule", "alert rules"),
		count(c.alertChannels, "channel", "channels"),
		count(c.alertHistory, "past alert", "past alerts"),
		count(c.auditEntries, "audit entry", "audit entries"),
		count(c.apiTokens, "API token", "API tokens"),
		c.logsBytes > 0 ? `${formatBytes(c.logsBytes)} of logs` : null,
	].filter((part) => part !== null);
	return parts.length > 0 ? parts.join(", ") : "Default settings only";
}

const MAX_INTERVAL_HOURS = 24 * 30;
const MAX_KEEP = 100;

/** Mirrors the server's checks; returns an error message or null. */
export function validateSchedule(
	intervalHours: string,
	keep: string,
): string | null {
	const interval =
		intervalHours.trim() === "" ? Number.NaN : Number(intervalHours);
	if (
		!Number.isInteger(interval) ||
		interval < 0 ||
		interval > MAX_INTERVAL_HOURS
	) {
		return `The interval must be a whole number of hours between 0 (off) and ${MAX_INTERVAL_HOURS}`;
	}
	const kept = keep.trim() === "" ? Number.NaN : Number(keep);
	if (!Number.isInteger(kept) || kept < 1 || kept > MAX_KEEP) {
		return `Keep between 1 and ${MAX_KEEP} scheduled backups`;
	}
	return null;
}
