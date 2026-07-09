import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Save } from "lucide-react";
import { useEffect, useId, useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";

import { getContainerResources } from "../api/get-container-resources";
import {
  type UpdateResourcesRequest,
  updateContainerResources,
} from "../api/update-container-resources";
import { formatMemoryBytes, parseMemoryInput } from "./parse-memory";

interface ResourceLimitsProps {
  containerId: string;
  containerHost: string;
  isReadOnly?: boolean;
}

const RESTART_POLICIES = [
  { value: "no", label: "No" },
  { value: "always", label: "Always" },
  { value: "unless-stopped", label: "Unless stopped" },
  { value: "on-failure", label: "On failure" },
];

export function ResourceLimits({
  containerId,
  containerHost,
  isReadOnly = false,
}: ResourceLimitsProps) {
  const queryClient = useQueryClient();
  const [memory, setMemory] = useState("");
  const [cpus, setCpus] = useState("");
  const [restartPolicy, setRestartPolicy] = useState("no");
  const [maxRetries, setMaxRetries] = useState("");
  const memoryId = useId();
  const cpusId = useId();
  const maxRetriesId = useId();

  const {
    data: resources,
    isLoading,
    error,
  } = useQuery({
    queryKey: ["container-resources", containerId, containerHost],
    queryFn: () => getContainerResources(containerId, containerHost),
    enabled: !!containerId && !!containerHost,
  });

  useEffect(() => {
    if (!resources) return;
    setMemory(formatMemoryBytes(resources.memoryBytes));
    setCpus(resources.nanoCPUs > 0 ? String(resources.nanoCPUs / 1e9) : "");
    setRestartPolicy(resources.restartPolicy.name || "no");
    setMaxRetries(
      resources.restartPolicy.maximumRetryCount > 0
        ? String(resources.restartPolicy.maximumRetryCount)
        : ""
    );
  }, [resources]);

  const updateMutation = useMutation({
    mutationFn: (request: UpdateResourcesRequest) =>
      updateContainerResources(containerId, containerHost, request),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ["container-resources", containerId, containerHost],
      });
      toast.success("Resource limits updated", {
        description: "Applied live — no container restart needed.",
      });
    },
    onError: (error: Error) => {
      toast.error("Failed to update resource limits", {
        description: error.message,
      });
    },
  });

  const handleSave = () => {
    const memoryBytes = parseMemoryInput(memory);
    if (memoryBytes === null) {
      toast.error("Invalid memory limit", {
        description: 'Use a number with an optional unit, e.g. "512m" or "1g".',
      });
      return;
    }

    const cpuValue = cpus.trim() === "" ? 0 : Number.parseFloat(cpus);
    if (Number.isNaN(cpuValue) || cpuValue < 0) {
      toast.error("Invalid CPU limit", {
        description: "Use a positive number, e.g. 0.5 or 2.",
      });
      return;
    }

    const retryCount =
      restartPolicy === "on-failure"
        ? Number.parseInt(maxRetries, 10) || 0
        : 0;

    updateMutation.mutate({
      memoryBytes,
      nanoCPUs: Math.round(cpuValue * 1e9),
      restartPolicy: {
        name: restartPolicy,
        maximumRetryCount: retryCount,
      },
    });
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-4 text-sm text-muted-foreground">
        Loading resource limits...
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center py-4 text-sm text-destructive">
        Failed to load resource limits
      </div>
    );
  }

  return (
    <div className="space-y-3 pt-2">
      <div className="flex items-center gap-2 justify-end border-b pb-2">
        <TooltipProvider>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="default"
                size="sm"
                onClick={handleSave}
                disabled={isReadOnly || updateMutation.isPending}
                className="h-8"
              >
                <Save className="mr-2 h-3.5 w-3.5" />
                {updateMutation.isPending ? "Saving..." : "Save"}
              </Button>
            </TooltipTrigger>
            {isReadOnly && (
              <TooltipContent>Cannot edit in read-only mode</TooltipContent>
            )}
          </Tooltip>
        </TooltipProvider>
      </div>

      <div className="rounded-lg border p-3">
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <div className="space-y-1.5">
            <Label htmlFor={memoryId} className="text-xs font-medium">
              Memory limit
            </Label>
            <Input
              id={memoryId}
              value={memory}
              onChange={(e) => setMemory(e.target.value)}
              disabled={isReadOnly}
              placeholder="e.g. 512m, 1g (empty = unlimited)"
              className="font-mono text-xs h-8"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor={cpusId} className="text-xs font-medium">
              CPUs
            </Label>
            <Input
              id={cpusId}
              type="number"
              min="0"
              step="0.1"
              value={cpus}
              onChange={(e) => setCpus(e.target.value)}
              disabled={isReadOnly}
              placeholder="e.g. 1.5 (empty = unlimited)"
              className="font-mono text-xs h-8"
            />
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs font-medium">Restart policy</Label>
            <Select
              value={restartPolicy}
              onValueChange={setRestartPolicy}
              disabled={isReadOnly}
            >
              <SelectTrigger className="h-8 text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {RESTART_POLICIES.map((policy) => (
                  <SelectItem key={policy.value} value={policy.value}>
                    {policy.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          {restartPolicy === "on-failure" && (
            <div className="space-y-1.5">
              <Label htmlFor={maxRetriesId} className="text-xs font-medium">
                Max retries
              </Label>
              <Input
                id={maxRetriesId}
                type="number"
                min="0"
                step="1"
                value={maxRetries}
                onChange={(e) => setMaxRetries(e.target.value)}
                disabled={isReadOnly}
                placeholder="0 = unlimited retries"
                className="font-mono text-xs h-8"
              />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
