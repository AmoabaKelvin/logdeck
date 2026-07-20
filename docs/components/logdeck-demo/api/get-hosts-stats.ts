import { demoHosts, demoImages, state } from "@/lib/logdeck-demo/store";

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

const HOST_HARDWARE: Record<string, { ncpu: number; mem: number; os: string }> =
	{
		"local-dev": { ncpu: 8, mem: 16, os: "Ubuntu 24.04.1 LTS" },
		"staging-eu": { ncpu: 4, mem: 8, os: "Debian GNU/Linux 12" },
		"edge-us": { ncpu: 2, mem: 4, os: "Alpine Linux v3.20" },
	};

export async function getHostsStats(): Promise<GetHostsStatsResponse> {
	const hosts = demoHosts.map((host) => {
		const containers = state.containers.filter((c) => c.host === host.name);
		const hardware = HOST_HARDWARE[host.name] ?? {
			ncpu: 4,
			mem: 8,
			os: "Linux",
		};
		return {
			host: host.name,
			available: true,
			name: host.name,
			operating_system: hardware.os,
			architecture: "x86_64",
			server_version: "27.3.1",
			ncpu: hardware.ncpu,
			mem_total: hardware.mem * 1024 * 1024 * 1024,
			containers_running: containers.filter((c) => c.state === "running")
				.length,
			containers_paused: containers.filter((c) => c.state === "paused").length,
			containers_stopped: containers.filter(
				(c) => c.state === "exited" || c.state === "dead",
			).length,
			images: demoImages.filter((image) => image.host === host.name).length,
		};
	});

	return { hosts };
}
