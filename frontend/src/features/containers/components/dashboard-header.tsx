import { ChevronDownIcon } from "@/components/ui/icons";

import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "@/components/ui/popover";

import type { HostStats } from "../api/get-hosts-stats";
import { formatBytes } from "./container-utils";
import { Sparkline } from "./sparkline";

interface SystemUsageSample {
	cpuPercent: number;
	memoryPercent: number;
}

interface DashboardHeaderProps {
	totalContainers: number;
	hostname: string;
	platform: string;
	kernel: string;
	cpuPercent: number;
	memoryPercent: number;
	systemHistory: SystemUsageSample[];
	hostsStats: HostStats[] | undefined;
}

function Vital({
	label,
	percent,
	history,
}: {
	label: string;
	percent: number;
	history: number[];
}) {
	return (
		<div className="flex min-w-0 items-center gap-2.5">
			<span className="font-mono text-[0.625rem] uppercase tracking-wide text-muted-foreground">
				{label}
			</span>
			<span className="text-sm font-medium tabular-nums">{percent}%</span>
			<Sparkline values={history} />
		</div>
	);
}

function HostRow({ host }: { host: HostStats }) {
	return (
		<div className="border-b border-border/60 py-2.5 last:border-b-0 first:pt-0 last:pb-0">
			<div className="flex items-baseline justify-between gap-3">
				<p className="truncate text-sm font-medium" title={host.host}>
					{host.host}
				</p>
				{host.available ? (
					<span className="shrink-0 font-mono text-xs text-muted-foreground">
						v{host.server_version}
					</span>
				) : (
					<span className="shrink-0 text-xs font-medium text-rose-600 dark:text-rose-400">
						Unreachable
					</span>
				)}
			</div>
			<p
				className="truncate text-xs text-muted-foreground"
				title={host.available ? undefined : host.error}
			>
				{host.available
					? `${host.ncpu} CPUs · ${formatBytes(host.mem_total)} · ${host.containers_running} running / ${host.containers_stopped} stopped`
					: (host.error ?? "Host could not be reached")}
			</p>
		</div>
	);
}

export function DashboardHeader({
	totalContainers,
	hostname,
	platform,
	kernel,
	cpuPercent,
	memoryPercent,
	systemHistory,
	hostsStats,
}: DashboardHeaderProps) {
	const multiHost = hostsStats !== undefined && hostsStats.length > 1;
	const containerCount = `${totalContainers} ${totalContainers === 1 ? "container" : "containers"}`;

	return (
		<div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between sm:gap-8">
			<div className="min-w-0">
				<h1 className="text-2xl font-semibold tracking-tight">Containers</h1>
				<p className="mt-1 truncate text-base/6 text-muted-foreground sm:text-sm/6">
					{multiHost ? (
						<>
							{containerCount} across{" "}
							<Popover>
								<PopoverTrigger className="inline-flex items-center gap-0.5 rounded-sm underline decoration-dotted underline-offset-4 hover:text-foreground">
									{hostsStats.length} hosts
									<ChevronDownIcon className="size-3.5 shrink-0" />
								</PopoverTrigger>
								<PopoverContent align="start" className="w-80 p-3">
									{hostsStats.map((host) => (
										<HostRow key={host.host} host={host} />
									))}
								</PopoverContent>
							</Popover>
							{" · LogDeck on "}
							{hostname}
						</>
					) : (
						<>
							{containerCount} on {hostname} · {platform} {kernel}
						</>
					)}
				</p>
			</div>

			<div className="flex shrink-0 items-center gap-5 sm:gap-6">
				<Vital
					label="CPU"
					percent={cpuPercent}
					history={systemHistory.map((sample) => sample.cpuPercent)}
				/>
				<span className="h-8 w-px bg-border" aria-hidden="true" />
				<Vital
					label="Mem"
					percent={memoryPercent}
					history={systemHistory.map((sample) => sample.memoryPercent)}
				/>
			</div>
		</div>
	);
}
