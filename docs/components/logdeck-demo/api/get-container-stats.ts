import { getContainerById, state } from "@/lib/logdeck-demo/store";

import type { ContainerStats } from "../types";

export interface GetContainerStatsResponse {
	stats: ContainerStats[];
}

// Each poll nudges the numbers with a bounded random walk so sparklines and
// usage columns move like a live host.
export async function getContainerStats(): Promise<GetContainerStatsResponse> {
	const next = state.stats.map((stat) => {
		const container = getContainerById(stat.id);
		if (!container || container.state !== "running") {
			return { ...stat, cpu_percent: 0, memory_percent: 0, memory_used: 0 };
		}

		const cpuJitter = (Math.random() - 0.5) * 4;
		const memJitter = (Math.random() - 0.5) * 2.4;

		const cpu = Math.max(0.4, Math.min(97, stat.cpu_percent + cpuJitter));
		const memory = Math.max(1, Math.min(95, stat.memory_percent + memJitter));

		return {
			...stat,
			cpu_percent: cpu,
			memory_percent: memory,
			memory_used: Math.round((memory / 100) * stat.memory_limit),
		};
	});

	state.stats = next;
	return { stats: structuredClone(next) };
}
