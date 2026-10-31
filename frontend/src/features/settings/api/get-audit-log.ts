import { authenticatedFetch, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

const ENDPOINT = `${API_BASE_URL}/api/v1/settings/audit`;

export interface AuditEntry {
	time: string;
	/** Username, "token:<name>" for an API token, or "" with auth disabled. */
	actor: string;
	role?: string;
	action: string;
	method: string;
	route: string;
	host?: string;
	target?: string;
	status: number;
	durationMs: number;
}

export interface AuditLogPage {
	entries: AuditEntry[];
	total: number;
}

export interface AuditLogParams {
	actor?: string;
	action?: string;
	target?: string;
	result?: "success" | "failure";
	since?: string;
	limit: number;
	offset: number;
}

export async function getAuditLog(
	params: AuditLogParams,
): Promise<AuditLogPage> {
	const query = new URLSearchParams();
	for (const [key, value] of Object.entries(params)) {
		if (value !== undefined && value !== "") query.set(key, String(value));
	}
	const response = await authenticatedFetch(`${ENDPOINT}?${query}`);

	if (!response.ok) {
		const message = await response.text();
		throw new Error(message || "Failed to load the audit log");
	}

	return readJson<AuditLogPage>(response);
}
