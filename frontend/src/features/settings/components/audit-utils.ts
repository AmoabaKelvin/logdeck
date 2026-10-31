import type { AuditEntry } from "../api/get-audit-log";

export const AUDIT_RANGES = ["all", "1h", "24h", "7d", "30d"] as const;
export type AuditRange = (typeof AUDIT_RANGES)[number];

export const AUDIT_RANGE_LABELS = {
	all: "Any time",
	"1h": "Last hour",
	"24h": "Last 24 hours",
	"7d": "Last 7 days",
	"30d": "Last 30 days",
} satisfies Record<AuditRange, string>;

const RANGE_MS = {
	"1h": 3_600_000,
	"24h": 86_400_000,
	"7d": 7 * 86_400_000,
	"30d": 30 * 86_400_000,
} satisfies Record<Exclude<AuditRange, "all">, number>;

/** The start of a time range as RFC 3339, or undefined for any time. */
export function auditSince(range: AuditRange, now: number): string | undefined {
	if (range === "all") return undefined;
	return new Date(now - RANGE_MS[range]).toISOString();
}

/** Who made a change, in words. */
export function describeActor(entry: AuditEntry): string {
	if (!entry.actor) return "Anyone (auth off)";
	if (entry.actor.startsWith("token:")) {
		return `API token ${entry.actor.slice("token:".length)}`;
	}
	return entry.actor;
}

/** What a change applied to: target@host, shortening container IDs. */
export function describeTarget(entry: AuditEntry): string {
	let target = entry.target ?? "";
	if (/^[0-9a-f]{64}$/.test(target)) target = target.slice(0, 12);
	if (target && entry.host) return `${target}@${entry.host}`;
	return target || entry.host || "—";
}

/** Whether a change worked, with a short label for the result column. */
export function describeResult(entry: AuditEntry) {
	if (entry.status === 101) return { ok: true, label: "Opened" };
	if (entry.status > 0 && entry.status < 400) {
		return { ok: true, label: "Succeeded" };
	}
	if (entry.status === 401 || entry.status === 403) {
		return { ok: false, label: `Refused (${entry.status})` };
	}
	return { ok: false, label: `Failed (${entry.status})` };
}
