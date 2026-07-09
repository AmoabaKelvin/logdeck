import { authenticatedFetch } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

export interface HostStats {
	host: string;
	available: boolean;
	error?: string;
	name?: string;
	operating_system?: string;
	architecture?: string;
	server_version?: string;
	ncpu: number;
	mem_total: number;
	containers_running: number;
	containers_paused: number;
	containers_stopped: number;
	images: number;
}

export interface GetHostsStatsResponse {
	hosts: HostStats[];
}

export async function getHostsStats(): Promise<GetHostsStatsResponse> {
	const response = await authenticatedFetch(
		`${API_BASE_URL}/api/v1/hosts/stats`,
	);

	if (!response.ok) {
		throw new Error("Failed to fetch hosts stats");
	}

	return response.json();
}
