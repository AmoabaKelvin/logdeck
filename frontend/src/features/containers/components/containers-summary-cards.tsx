import { Card, CardContent } from "@/components/ui/card";

import type { HostStats } from "../api/get-hosts-stats";

import { formatBytes } from "./container-utils";

interface HostInfo {
  hostname: string;
  os: string;
  kernel: string;
}

interface SystemUsage {
  cpu: number;
  memory: number;
}

interface ContainersSummaryCardsProps {
  totalContainers: number;
  hostInfo: HostInfo;
  systemUsage: SystemUsage;
  hostsStats?: HostStats[];
}

export function ContainersSummaryCards({
  totalContainers,
  hostInfo,
  systemUsage,
  hostsStats,
}: ContainersSummaryCardsProps) {
  return (
    <section className="space-y-3">
      <div className="grid gap-3 md:grid-cols-3">
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
                  <span className="font-medium">{systemUsage.cpu}%</span>
                </div>
                <div className="h-1.5 w-full rounded-full bg-muted">
                  <div
                    className="h-1.5 rounded-full bg-foreground"
                    style={{ width: `${systemUsage.cpu}%` }}
                  />
                </div>
                <div className="flex items-center justify-between text-xs">
                  <span className="text-muted-foreground">Memory</span>
                  <span className="font-medium">{systemUsage.memory}%</span>
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

      {hostsStats && hostsStats.length > 1 && (
        <div className="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
          {hostsStats.map((host) => (
            <Card key={host.host} className="py-3">
              <CardContent className="px-6 py-0">
                <div className="space-y-1">
                  <div className="flex items-center justify-between gap-2">
                    <p
                      className="text-sm font-medium truncate"
                      title={host.host}
                    >
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
                      {host.containers_running} running /{" "}
                      {host.containers_stopped} stopped
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
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </section>
  );
}
