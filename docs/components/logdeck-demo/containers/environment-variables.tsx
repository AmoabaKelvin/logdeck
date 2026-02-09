import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Edit2, Save, X } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

import { Badge } from "@/components/logdeck-demo/ui/badge";
import { Button } from "@/components/logdeck-demo/ui/button";
import { Input } from "@/components/logdeck-demo/ui/input";

import {
  getContainerEnvVariablesDemo,
  updateContainerEnvVariablesDemo,
} from "@/lib/logdeck-demo/demo-api";

interface EnvironmentVariablesProps {
  containerId: string;
  containerHost: string;
  isReadOnly?: boolean;
  onContainerIdChange?: (newContainerId: string) => void;
}

export function EnvironmentVariables({
  containerId,
  containerHost: _containerHost,
  isReadOnly = false,
  onContainerIdChange,
}: EnvironmentVariablesProps) {
  const queryClient = useQueryClient();
  const [isEditing, setIsEditing] = useState(false);
  const [editedEnv, setEditedEnv] = useState<Record<string, string>>({});

  const { data: envVariables, isLoading } = useQuery({
    queryKey: ["container-env", containerId],
    queryFn: () => getContainerEnvVariablesDemo(containerId),
    enabled: !!containerId,
  });

  const updateMutation = useMutation({
    mutationFn: (env: Record<string, string>) =>
      updateContainerEnvVariablesDemo(containerId, env),
    onSuccess: (newContainerId) => {
      queryClient.invalidateQueries({ queryKey: ["container-env", containerId] });
      toast.success("Demo env variables updated");
      setIsEditing(false);
      onContainerIdChange?.(newContainerId);
    },
    onError: (error: Error) => {
      toast.error(error.message || "Failed to update env variables");
    },
  });

  const displayEnv = isEditing ? editedEnv : envVariables ?? {};
  const entries = Object.entries(displayEnv);

  const handleEdit = () => {
    setEditedEnv({ ...(envVariables ?? {}) });
    setIsEditing(true);
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-4 text-sm text-muted-foreground">
        Loading environment variables...
      </div>
    );
  }

  return (
    <div className="space-y-3 pt-2">
      <div className="flex items-center justify-end gap-2 border-b pb-2">
        {!isEditing ? (
          <Button
            variant="outline"
            size="sm"
            onClick={handleEdit}
            disabled={isReadOnly}
            className="h-8"
          >
            <Edit2 className="mr-2 h-3.5 w-3.5" />
            Edit
          </Button>
        ) : (
          <>
            <Button
              variant="outline"
              size="sm"
              className="h-8"
              onClick={() => setIsEditing(false)}
            >
              <X className="mr-2 h-3.5 w-3.5" />
              Cancel
            </Button>
            <Button
              size="sm"
              className="h-8"
              onClick={() => updateMutation.mutate(editedEnv)}
              disabled={updateMutation.isPending}
            >
              <Save className="mr-2 h-3.5 w-3.5" />
              Save
            </Button>
          </>
        )}
      </div>

      {entries.length === 0 ? (
        <div className="text-sm text-muted-foreground">No environment variables configured</div>
      ) : (
        <div className="space-y-2">
          {entries.map(([key, value]) => (
            <div
              key={key}
              className="grid grid-cols-[minmax(180px,220px)_1fr] gap-2 items-center"
            >
              <Badge variant="outline" className="justify-start font-mono">
                {key}
              </Badge>
              {isEditing ? (
                <Input
                  value={value}
                  onChange={(event) =>
                    setEditedEnv((prev) => ({ ...prev, [key]: event.target.value }))
                  }
                  className="font-mono text-xs"
                />
              ) : (
                <div className="rounded-md border bg-muted/30 px-3 py-1.5 font-mono text-xs break-all">
                  {value}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
