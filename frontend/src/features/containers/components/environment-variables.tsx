import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Edit2, Plus, Save, Trash2, X } from "lucide-react";
import { useId, useState } from "react";
import { toast } from "sonner";

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

import { getContainerEnvVariables } from "../api/get-container-env-variables";
import { updateContainerEnvVariables } from "../api/update-container-env-variables";

interface EnvironmentVariablesProps {
  containerId: string;
  onContainerIdChange?: (newContainerId: string) => void;
}

export function EnvironmentVariables({
  containerId,
  onContainerIdChange,
}: EnvironmentVariablesProps) {
  const queryClient = useQueryClient();
  const [isEditing, setIsEditing] = useState(false);
  const [editedEnv, setEditedEnv] = useState<Record<string, string>>({});
  const [deletedKeys, setDeletedKeys] = useState<Set<string>>(new Set());
  const [newKey, setNewKey] = useState("");
  const [newValue, setNewValue] = useState("");
  const [showAddNew, setShowAddNew] = useState(false);
  const [showConfirmDialog, setShowConfirmDialog] = useState(false);
  const newKeyId = useId();
  const newValueId = useId();

  const {
    data: envVariables,
    isLoading,
    error,
  } = useQuery({
    queryKey: ["container-env", containerId],
    queryFn: () => getContainerEnvVariables(containerId),
    enabled: !!containerId,
  });

  const updateMutation = useMutation({
    mutationFn: (env: Record<string, string>) =>
      updateContainerEnvVariables(containerId, env),
    onSuccess: (newContainerId) => {
      // Invalidate queries for BOTH old and new container IDs
      queryClient.invalidateQueries({
        queryKey: ["container-env", containerId],
      });
      queryClient.invalidateQueries({
        queryKey: ["container-env", newContainerId],
      });
      queryClient.invalidateQueries({
        queryKey: ["containers"],
      });

      // Notify parent of new container ID
      onContainerIdChange?.(newContainerId);

      setIsEditing(false);
      setEditedEnv({});
      setDeletedKeys(new Set());
      toast.success("Environment variables updated successfully", {
        description:
          "The container has been recreated with the new environment variables.",
      });
    },
    onError: (error: Error) => {
      toast.error("Failed to update environment variables", {
        description: error.message,
      });
    },
  });

  const handleEdit = () => {
    setEditedEnv({ ...envVariables });
    setDeletedKeys(new Set());
    setIsEditing(true);
  };

  const handleCancel = () => {
    setIsEditing(false);
    setEditedEnv({});
    setDeletedKeys(new Set());
    setShowAddNew(false);
    setNewKey("");
    setNewValue("");
  };

  const handleSave = () => {
    setShowConfirmDialog(true);
  };

  const handleConfirmUpdate = () => {
    const finalEnv = { ...editedEnv };
    // Remove deleted keys
    deletedKeys.forEach((key) => {
      delete finalEnv[key];
    });
    updateMutation.mutate(finalEnv);
    setShowConfirmDialog(false);
  };

  const handleDelete = (key: string) => {
    setDeletedKeys((prev) => new Set(prev).add(key));
  };

  const handleValueChange = (key: string, value: string) => {
    setEditedEnv((prev) => ({ ...prev, [key]: value }));
  };

  const handleAddNew = () => {
    if (newKey.trim() && !editedEnv[newKey]) {
      setEditedEnv((prev) => ({ ...prev, [newKey]: newValue }));
      setNewKey("");
      setNewValue("");
      setShowAddNew(false);
    } else if (editedEnv[newKey]) {
      toast.error("Key already exists");
    } else {
      toast.error("Key cannot be empty");
    }
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-4 text-sm text-muted-foreground">
        Loading environment variables...
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center py-4 text-sm text-destructive">
        Failed to load environment variables
      </div>
    );
  }

  const displayEnv = isEditing ? editedEnv : envVariables || {};
  const envEntries = Object.entries(displayEnv).filter(
    ([key]) => !deletedKeys.has(key)
  );

  if (envEntries.length === 0 && !isEditing) {
    return (
      <div className="space-y-3">
        <div className="flex items-center justify-center py-4 text-sm text-muted-foreground">
          No environment variables configured
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-3 pt-2">
      {/* Edit/Save/Cancel buttons */}
      <div className="flex items-center gap-2 justify-end border-b pb-2">
        {!isEditing ? (
          <Button
            variant="outline"
            size="sm"
            onClick={handleEdit}
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
              onClick={() => setShowAddNew(true)}
              className="h-8"
            >
              <Plus className="mr-2 h-3.5 w-3.5" />
              Add
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={handleCancel}
              className="h-8"
            >
              <X className="mr-2 h-3.5 w-3.5" />
              Cancel
            </Button>
            <Button
              variant="default"
              size="sm"
              onClick={handleSave}
              disabled={updateMutation.isPending}
              className="h-8"
            >
              <Save className="mr-2 h-3.5 w-3.5" />
              {updateMutation.isPending ? "Saving..." : "Save"}
            </Button>
          </>
        )}
      </div>

      {/* Add new variable form */}
      {showAddNew && (
        <div className="rounded-lg border border-primary p-3 transition-colors bg-muted/30">
          <div className="flex items-end gap-3">
            <div className="flex-1 space-y-2">
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div className="space-y-1.5">
                  <Label htmlFor={newKeyId} className="text-xs font-medium">
                    Key
                  </Label>
                  <Input
                    id={newKeyId}
                    value={newKey}
                    onChange={(e) => setNewKey(e.target.value)}
                    placeholder="VARIABLE_NAME"
                    className="font-mono text-xs h-8"
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor={newValueId} className="text-xs font-medium">
                    Value
                  </Label>
                  <Input
                    id={newValueId}
                    value={newValue}
                    onChange={(e) => setNewValue(e.target.value)}
                    placeholder="value"
                    className="font-mono text-xs h-8"
                  />
                </div>
              </div>
            </div>
            <div className="flex gap-1">
              <Button
                variant="ghost"
                size="icon"
                onClick={handleAddNew}
                className="mb-0.5 h-8 w-8 text-primary hover:text-primary"
                title="Add variable"
              >
                <Plus className="h-3.5 w-3.5" />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                onClick={() => {
                  setShowAddNew(false);
                  setNewKey("");
                  setNewValue("");
                }}
                className="mb-0.5 h-8 w-8 text-muted-foreground"
                title="Cancel"
              >
                <X className="h-3.5 w-3.5" />
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Existing environment variables */}
      {envEntries.map(([key, value]) => (
        <div
          key={key}
          className={`flex items-end gap-3 rounded-lg border p-3 transition-colors ${
            deletedKeys.has(key)
              ? "opacity-50 bg-destructive/10"
              : "hover:bg-muted/50"
          }`}
        >
          <div className="flex-1 space-y-2">
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <div className="space-y-1.5">
                <Label htmlFor={`key-${key}`} className="text-xs font-medium">
                  Key
                </Label>
                <Input
                  id={`key-${key}`}
                  value={key}
                  disabled
                  className="font-mono text-xs h-8"
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor={`value-${key}`} className="text-xs font-medium">
                  Value
                </Label>
                <Input
                  id={`value-${key}`}
                  value={value}
                  onChange={(e) => handleValueChange(key, e.target.value)}
                  disabled={!isEditing}
                  className="font-mono text-xs h-8"
                />
              </div>
            </div>
          </div>
          <Button
            variant="ghost"
            size="icon"
            onClick={() => handleDelete(key)}
            className="mb-0.5 h-8 w-8 text-muted-foreground hover:text-destructive"
            disabled={!isEditing}
            title={isEditing ? "Mark for deletion" : "Edit to enable deletion"}
          >
            <Trash2 className="h-3.5 w-3.5" />
          </Button>
        </div>
      ))}

      {envEntries.length === 0 && isEditing && (
        <div className="flex items-center justify-center py-4 text-sm text-muted-foreground">
          No environment variables. Click "Add" to create one.
        </div>
      )}

      {/* Confirmation Dialog */}
      <AlertDialog open={showConfirmDialog} onOpenChange={setShowConfirmDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Update environment variables?</AlertDialogTitle>
            <AlertDialogDescription>
              Changing environment variables requires recreating the container.
              This will cause a brief downtime. Are you sure you want to
              continue?
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={handleConfirmUpdate}>
              Confirm & Update
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
