import { Card, CardContent } from "@/components/logdeck-demo/ui/card";

import type { HostStats } from "../api/get-hosts-stats";

import { formatBytes } from "./container-utils";
import { Sparkline } from "./sparkline";

interface HostInfo {
	hostname: string;
	os: string;
	kernel: string;
}

interface SystemUsage {
	cpu: number;
	memory: number;
}

interface SystemUsageSample {
	cpuPercent: number;
	memoryPercent: number;
}

interface ContainersSummaryCardsProps {
	totalContainers: number;
	hostInfo: HostInfo;
	systemUsage: SystemUsage;
	hostsStats?: HostStats[];
	systemHistory: SystemUsageSample[];
}

function HostRow({ host }: { host: HostStats }) {
	return (
		<div className="space-y-0.5">
			<div className="flex items-center justify-between gap-2">
				<p className="text-sm font-medium truncate" title={host.host}>
					{host.host}
				</p>
				{host.available ? (
					<span className="text-xs text-muted-foreground shrink-0">
						v{host.server_version}
					</span>
				) : (
					<span className="text-xs font-medium text-destructive shrink-0">
						Unreachable
					</span>
				)}
			</div>
			{host.available ? (
				<p className="text-xs text-muted-foreground">
					{host.ncpu} CPUs • {formatBytes(host.mem_total)} •{" "}
					{host.containers_running} running / {host.containers_stopped} stopped
				</p>
			) : (
				<p
					className="text-xs text-muted-foreground truncate"
					title={host.error}
				>
					{host.error ?? "Host could not be reached"}
				</p>
			)}
		</div>
	);
}

export function ContainersSummaryCards({
	totalContainers,
	hostInfo,
	systemUsage,
	hostsStats,
	systemHistory,
}: ContainersSummaryCardsProps) {
	const multiHost = hostsStats && hostsStats.length > 1;

	return (
		<section className="space-y-3">
			<div className="grid gap-3 md:grid-cols-3">
				{multiHost ? (
					<Card className="py-4">
						<CardContent className="px-6 py-0">
							<div className="space-y-2">
								<p className="text-sm text-muted-foreground">Hosts</p>
								<div className="max-h-24 space-y-2 overflow-y-auto">
									{hostsStats.map((host) => (
										<HostRow key={host.host} host={host} />
									))}
								</div>
							</div>
						</CardContent>
					</Card>
				) : (
					<Card className="py-4">
						<CardContent className="px-6 py-0">
							<div className="space-y-1">
								<p className="text-sm text-muted-foreground">Host</p>
								<p
									className="text-2xl font-semibold truncate"
									title={hostInfo.hostname}
								>
									{hostInfo.hostname}
								</p>
								<p className="text-xs text-muted-foreground">
									{hostInfo.os} • {hostInfo.kernel}
								</p>
							</div>
						</CardContent>
					</Card>
				)}

				<Card className="py-4">
					<CardContent className="px-6 py-0">
						<div className="space-y-1">
							<p className="text-sm text-muted-foreground">Containers</p>
							<p className="text-2xl font-semibold">{totalContainers}</p>
						</div>
					</CardContent>
				</Card>

				<Card className="py-4">
					<CardContent className="px-6 py-0">
						<div className="space-y-2">
							<p className="text-sm text-muted-foreground">
								System (LogDeck host)
							</p>
							<div className="space-y-1.5">
								<div className="flex items-center justify-between text-xs">
									<span className="text-muted-foreground">CPU</span>
									<span className="flex items-center gap-2">
										<Sparkline
											values={systemHistory.map((s) => s.cpuPercent)}
										/>
										<span className="font-medium">{systemUsage.cpu}%</span>
									</span>
								</div>
								<div className="h-1.5 w-full rounded-full bg-muted">
									<div
										className="h-1.5 rounded-full bg-foreground"
										style={{ width: `${systemUsage.cpu}%` }}
									/>
								</div>
								<div className="flex items-center justify-between text-xs">
									<span className="text-muted-foreground">Memory</span>
									<span className="flex items-center gap-2">
										<Sparkline
											values={systemHistory.map((s) => s.memoryPercent)}
										/>
										<span className="font-medium">{systemUsage.memory}%</span>
									</span>
								</div>
								<div className="h-1.5 w-full rounded-full bg-muted">
									<div
										className="h-1.5 rounded-full bg-foreground"
										style={{ width: `${systemUsage.memory}%` }}
									/>
								</div>
							</div>
						</div>
					</CardContent>
				</Card>
			</div>
		</section>
	);
}
