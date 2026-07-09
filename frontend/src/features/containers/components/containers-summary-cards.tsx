import { Card, CardContent } from "@/components/ui/card";

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
  systemHistory: SystemUsageSample[];
}

export function ContainersSummaryCards({
  totalContainers,
  hostInfo,
  systemUsage,
  systemHistory,
}: ContainersSummaryCardsProps) {
  return (
    <section className="grid gap-3 md:grid-cols-3">
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
            <p className="text-sm text-muted-foreground">System</p>
            <div className="space-y-1.5">
              <div className="flex items-center justify-between text-xs">
                <span className="text-muted-foreground">CPU</span>
                <span className="flex items-center gap-2">
                  <Sparkline values={systemHistory.map((s) => s.cpuPercent)} />
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
    </section>
  );
}
