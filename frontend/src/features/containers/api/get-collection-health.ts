import { authenticatedFetch, readJson } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

/**
 * "unsupported" and "excluded" are deliberate gaps, not failures: the engine
 * cannot read the container's logs, or persistence is off for it.
 */
export type CollectionState =
	| "ok"
	| "pending"
	| "failing"
	| "unsupported"
	| "excluded";

export interface CollectionObservation {
	state: CollectionState;
	since: string;
	lastOk?: string;
	error?: string;
}

export interface ContainerCollectionHealth {
	id: string;
	name: string;
	/** The background tail feeding alerts and stored logs; absent when nothing tails it. */
	live?: CollectionObservation;
	/** The log store's engine read; absent when persistence is off or has not read it yet. */
	stored?: CollectionObservation;
}

export interface HostCollectionHealth extends CollectionObservation {
	host: string;
	containers: ContainerCollectionHealth[];
}

export interface StorageCollectionHealth extends CollectionObservation {
	lowSpace: boolean;
	freeBytes: number;
	droppedLines: number;
	backlog: number;
	backlogCap: number;
	recovering: number;
}

export interface CollectionHealthReport {
	status: "ok" | "degraded";
	hosts: HostCollectionHealth[];
	/** Absent when log persistence is disabled. */
	storage?: StorageCollectionHealth;
}

export async function getCollectionHealth(): Promise<CollectionHealthReport> {
	const response = await authenticatedFetch(
		`${API_BASE_URL}/api/v1/health/collection`,
	);

	if (!response.ok) {
		throw new Error("Failed to fetch collection health");
	}

	return readJson<CollectionHealthReport>(response);
}
