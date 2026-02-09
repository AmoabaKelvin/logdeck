import { FileTextIcon, PlayIcon, RotateCwIcon, SquareIcon, Trash2Icon } from "lucide-react";
import { Fragment } from "react";
import type { LucideIcon } from "lucide-react";

import { Badge } from "@/components/logdeck-demo/ui/badge";
import { Button } from "@/components/logdeck-demo/ui/button";
import { Spinner } from "@/components/logdeck-demo/ui/spinner";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/logdeck-demo/ui/table";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/logdeck-demo/ui/tooltip";

import {
  formatContainerName,
  formatCPUPercent,
  formatCreatedDate,
  formatMemoryStats,
  formatUptime,
  getStateBadgeClass,
  toTitleCase,
} from "./container-utils";

import type { ContainerInfo, ContainerStatsMap } from "./types";

import type {
  ContainerActionType,
  GroupByOption,
  GroupedContainers,
} from "./container-utils";

interface ActionButtonProps {
  icon: LucideIcon;
  action: ContainerActionType;
  containerId: string;
  onClick: () => void;
  isPending: (action: ContainerActionType, id: string) => boolean;
  busy: boolean;
  isReadOnly: boolean;
  variant?: "destructive";
}

function ActionButton({
  icon: Icon,
  action,
  containerId,
  onClick,
  isPending,
  busy,
  isReadOnly,
  variant,
}: ActionButtonProps) {
  const pending = isPending(action, containerId);
  const label = action.charAt(0).toUpperCase() + action.slice(1);
  const tooltip = isReadOnly ? `${label} (Read-only mode)` : label;

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="inline-block">
          <Button
            variant="outline"
            size="icon"
            className={`h-8 w-8 ${variant === "destructive" ? "text-destructive hover:bg-destructive hover:text-white" : ""}`}
            onClick={onClick}
            disabled={busy || isReadOnly}
          >
            {pending ? <Spinner className="size-4" /> : <Icon className="size-4" />}
          </Button>
        </span>
      </TooltipTrigger>
      <TooltipContent>{tooltip}</TooltipContent>
    </Tooltip>
  );
}

interface ContainersTableProps {
  isLoading: boolean;
  isError: boolean;
  error: unknown;
  groupBy: GroupByOption;
  filteredContainers: ContainerInfo[];
  groupedItems: GroupedContainers[] | null;
  pageItems: ContainerInfo[];
  pendingAction: { id: string; type: ContainerActionType } | null;
  isReadOnly: boolean;
  statsMap: ContainerStatsMap;
  onStart: (container: ContainerInfo) => void;
  onStop: (container: ContainerInfo) => void;
  onRestart: (container: ContainerInfo) => void;
  onDelete: (container: ContainerInfo) => void;
  onViewLogs: (container: ContainerInfo) => void;
  onRetry: () => void;
}

export function ContainersTable({
  isLoading,
  isError,
  error,
  groupBy,
  filteredContainers,
  groupedItems,
  pageItems,
  pendingAction,
  isReadOnly,
  statsMap,
  onStart,
  onStop,
  onRestart,
  onDelete,
  onViewLogs,
  onRetry,
}: ContainersTableProps) {
  const isPending = (action: ContainerActionType, id: string) =>
    pendingAction?.id === id && pendingAction.type === action;

  const isBusy = (id: string) => pendingAction?.id === id;

  const renderContainerRow = (container: ContainerInfo) => {
    const state = container.state.toLowerCase();
    const busy = isBusy(container.id);

    return (
      <TableRow key={container.id} className="hover:bg-muted/50">
        <TableCell className="h-16 px-4 font-medium">
          {formatContainerName(container.names)}
        </TableCell>
        <TableCell className="h-16 px-4 text-sm text-muted-foreground">
          {container.image}
        </TableCell>
        <TableCell className="h-16 px-4">
          <Badge
            className={`${getStateBadgeClass(container.state)} border-0`}
          >
            {toTitleCase(container.state)}
          </Badge>
        </TableCell>
        <TableCell className="h-16 px-4 text-sm">
          {state !== "running" ? (
            <span className="text-muted-foreground">—</span>
          ) : (
            <div className="space-y-0.5 font-mono text-xs">
              <div>
                <span className="text-muted-foreground">CPU: </span>
                {formatCPUPercent(statsMap[container.id]?.cpu_percent)}
              </div>
              <div>
                <span className="text-muted-foreground">Mem: </span>
                {formatMemoryStats(statsMap[container.id])}
              </div>
            </div>
          )}
        </TableCell>
        <TableCell className="h-16 px-4 text-sm text-muted-foreground">
          {formatUptime(container.created)}
        </TableCell>
        <TableCell className="h-16 px-4 text-sm text-muted-foreground">
          {formatCreatedDate(container.created)}
        </TableCell>
        <TableCell className="h-16 px-4 max-w-[300px] text-sm text-muted-foreground">
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger asChild>
                <span className="block cursor-help truncate">
                  {container.command}
                </span>
              </TooltipTrigger>
              <TooltipContent className="max-w-md">
                {container.command}
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </TableCell>
        <TableCell className="h-16 px-4">
          <TooltipProvider>
            <div className="flex items-center gap-1">
              {state === "exited" && (
                <ActionButton
                  icon={PlayIcon}
                  action="start"
                  containerId={container.id}
                  onClick={() => onStart(container)}
                  isPending={isPending}
                  busy={busy}
                  isReadOnly={isReadOnly}
                />
              )}
              {state === "running" && (
                <ActionButton
                  icon={SquareIcon}
                  action="stop"
                  containerId={container.id}
                  onClick={() => onStop(container)}
                  isPending={isPending}
                  busy={busy}
                  isReadOnly={isReadOnly}
                />
              )}
              <ActionButton
                icon={RotateCwIcon}
                action="restart"
                containerId={container.id}
                onClick={() => onRestart(container)}
                isPending={isPending}
                busy={busy}
                isReadOnly={isReadOnly}
              />
              <ActionButton
                icon={Trash2Icon}
                action="remove"
                containerId={container.id}
                onClick={() => onDelete(container)}
                isPending={isPending}
                busy={busy}
                isReadOnly={isReadOnly}
                variant="destructive"
              />
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="outline"
                    size="icon"
                    className="h-8 w-8"
                    onClick={() => onViewLogs(container)}
                    disabled={busy}
                  >
                    <FileTextIcon className="size-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>View Logs</TooltipContent>
              </Tooltip>
            </div>
          </TooltipProvider>
        </TableCell>
      </TableRow>
    );
  };

  return (
    <div className="rounded-lg border bg-card">
      <Table>
        <TableHeader>
          <TableRow className="hover:bg-transparent border-b">
            <TableHead className="h-12 px-4 font-medium">Name</TableHead>
            <TableHead className="h-12 px-4 font-medium">Image</TableHead>
            <TableHead className="h-12 px-4 font-medium w-[120px]">
              State
            </TableHead>
            <TableHead className="h-12 px-4 font-medium w-[160px]">
              Metrics
            </TableHead>
            <TableHead className="h-12 px-4 font-medium">Uptime</TableHead>
            <TableHead className="h-12 px-4 font-medium">Created</TableHead>
            <TableHead className="h-12 px-4 font-medium">Command</TableHead>
            <TableHead className="h-12 px-4 font-medium w-[120px]">
              Actions
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {isLoading ? (
            <TableRow>
              <TableCell colSpan={8} className="h-32">
                <div className="flex items-center justify-center text-sm text-muted-foreground">
                  <Spinner className="mr-2" />
                  Loading containers…
                </div>
              </TableCell>
            </TableRow>
          ) : isError ? (
            <TableRow>
              <TableCell colSpan={8} className="h-32">
                <div className="flex flex-col items-center gap-3 text-center">
                  <p className="text-sm text-muted-foreground">
                    {(error as Error)?.message || "Unable to load containers."}
                  </p>
                  <Button size="sm" variant="outline" onClick={onRetry}>
                    Try again
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          ) : filteredContainers.length === 0 ? (
            <TableRow>
              <TableCell colSpan={8} className="h-32">
                <div className="text-center text-sm text-muted-foreground">
                  No containers found.
                </div>
              </TableCell>
            </TableRow>
          ) : groupBy === "compose" && groupedItems ? (
            groupedItems.map((group) => (
              <Fragment key={group.project}>
                <TableRow className="bg-muted/30 hover:bg-muted/30">
                  <TableCell
                    colSpan={8}
                    className="h-10 px-4 text-xs font-medium text-muted-foreground"
                  >
                    {group.project} · {group.items.length} container
                    {group.items.length === 1 ? "" : "s"}
                  </TableCell>
                </TableRow>
                {group.items.map(renderContainerRow)}
              </Fragment>
            ))
          ) : (
            pageItems.map(renderContainerRow)
          )}
        </TableBody>
      </Table>
    </div>
  );
}
