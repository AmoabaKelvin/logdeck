import { Card, CardContent } from "@/components/ui/card";

interface HostInfo {
  host: string;
  dockerVersion: string;
}

interface SystemUsage {
  cpu: number;
  memory: number;
}

interface ContainersSummaryCardsProps {
  totalContainers: number;
  hostInfo: HostInfo;
  systemUsage: SystemUsage;
}

export function ContainersSummaryCards({
  totalContainers,
  hostInfo,
  systemUsage,
}: ContainersSummaryCardsProps) {
  return (
    <section className="grid gap-3 md:grid-cols-3">
      <Card className="py-4">
        <CardContent className="px-6 py-0">
          <div className="space-y-1">
            <p className="text-sm text-muted-foreground">Host</p>
            <p className="text-2xl font-semibold">{hostInfo.host}</p>
            <p className="text-xs text-muted-foreground">
              Docker {hostInfo.dockerVersion}
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
    </section>
  );
}
