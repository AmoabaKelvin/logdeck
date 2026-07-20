import { state } from "@/lib/logdeck-demo/store";

export interface SystemStats {
	hostInfo: {
		hostname: string;
		platform: string;
		platformVersion: string;
		kernelVersion: string;
		arch: string;
		uptime: number;
	};
	usage: {
		cpuPercent: number;
		memoryPercent: number;
		memoryTotal: number;
		memoryUsed: number;
	};
}

const MEMORY_TOTAL = 16 * 1024 * 1024 * 1024;

export async function getSystemStats(): Promise<SystemStats> {
	const running = state.containers.filter((c) => c.state === "running").length;
	const cpuTarget = 16 + running * 3;
	const memTarget = 24 + running * 2.5;

	// Random walk pulled gently toward a load-derived target.
	state.system.cpuPercent = Math.max(
		3,
		Math.min(
			94,
			state.system.cpuPercent +
				(cpuTarget - state.system.cpuPercent) * 0.15 +
				(Math.random() - 0.5) * 7,
		),
	);
	state.system.memoryPercent = Math.max(
		8,
		Math.min(
			92,
			state.system.memoryPercent +
				(memTarget - state.system.memoryPercent) * 0.1 +
				(Math.random() - 0.5) * 3,
		),
	);

	return {
		hostInfo: {
			hostname: "docs-demo-host",
			platform: "linux",
			platformVersion: "6.10",
			kernelVersion: "6.10.12-generic",
			arch: "x64",
			uptime: 940_000,
		},
		usage: {
			cpuPercent: state.system.cpuPercent,
			memoryPercent: state.system.memoryPercent,
			memoryTotal: MEMORY_TOTAL,
			memoryUsed: Math.round((state.system.memoryPercent / 100) * MEMORY_TOTAL),
		},
	};
}
