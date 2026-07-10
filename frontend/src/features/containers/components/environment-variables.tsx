import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Edit2, Plus, Save, Trash2, Upload, X } from "lucide-react";
import { useRef, useState } from "react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
	Tooltip,
	TooltipContent,
	TooltipProvider,
	TooltipTrigger,
} from "@/components/ui/tooltip";

import { getContainerEnvVariables } from "../api/get-container-env-variables";
import { updateContainerEnvVariables } from "../api/update-container-env-variables";
import { EnvUpdateConfirmDialog } from "./env-update-confirm-dialog";
import { EnvUploadPreviewDialog } from "./env-upload-preview-dialog";

interface EnvironmentVariablesProps {
	containerId: string;
	containerHost: string;
	isReadOnly?: boolean;
	isCoolifyManaged?: boolean;
	onContainerIdChange?: (newContainerId: string) => void;
}

export function parseEnvFile(content: string): Record<string, string> {
	const env: Record<string, string> = {};
	const lines = content.split("\n");

	for (const line of lines) {
		const trimmed = line.trim();

		if (!trimmed || trimmed.startsWith("#")) {
			continue;
		}

		const equalIndex = trimmed.indexOf("=");
		if (equalIndex === -1) {
			continue;
		}

		const key = trimmed.substring(0, equalIndex).trim();
		let value = trimmed.substring(equalIndex + 1).trim();

		if (
			(value.startsWith('"') && value.endsWith('"')) ||
			(value.startsWith("'") && value.endsWith("'"))
		) {
			value = value.slice(1, -1);
		}

		if (key) {
			env[key] = value;
		}
	}

	return env;
}

export function EnvironmentVariables({
	containerId,
	containerHost,
	isReadOnly = false,
	isCoolifyManaged = false,
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
	const [modifiedKeys, setModifiedKeys] = useState<Set<string>>(new Set());
	const [showUploadPreview, setShowUploadPreview] = useState(false);
	const [parsedEnvFile, setParsedEnvFile] = useState<Record<string, string>>(
		{},
	);
	const fileInputRef = useRef<HTMLInputElement>(null);
	const newValueInputRef = useRef<HTMLInputElement>(null);

	const {
		data: envVariables,
		isLoading,
		error,
	} = useQuery({
		queryKey: ["container-env", containerId, containerHost],
		queryFn: () => getContainerEnvVariables(containerId, containerHost),
		enabled: !!containerId && !!containerHost,
	});

	const updateMutation = useMutation({
		mutationFn: (env: Record<string, string>) =>
			updateContainerEnvVariables(containerId, containerHost, env),
		onSuccess: (result) => {
			// Invalidate queries for BOTH old and new container IDs
			queryClient.invalidateQueries({
				queryKey: ["container-env", containerId],
			});
			queryClient.invalidateQueries({
				queryKey: ["container-env", result.newContainerId],
			});
			queryClient.invalidateQueries({
				queryKey: ["containers"],
			});

			onContainerIdChange?.(result.newContainerId);

			setIsEditing(false);
			setEditedEnv({});
			setDeletedKeys(new Set());
			setModifiedKeys(new Set());

			if (result.coolifySynced === true) {
				toast.success("Environment variables updated", {
					description: "Container recreated and changes synced to Coolify.",
				});
			} else if (result.coolifySynced === false) {
				toast.warning("Updated, but Coolify sync failed", {
					description:
						result.coolifyError ||
						"Container recreated, but Coolify was not updated. Changes may be lost on redeployment.",
				});
			} else {
				toast.success("Environment variables updated", {
					description:
						"Container recreated with the new environment variables.",
				});
			}
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
		setModifiedKeys(new Set());
		setIsEditing(true);
	};

	const handleCancel = () => {
		setIsEditing(false);
		setEditedEnv({});
		setDeletedKeys(new Set());
		setModifiedKeys(new Set());
		setShowAddNew(false);
		setNewKey("");
		setNewValue("");
	};

	const handleSave = () => {
		setShowConfirmDialog(true);
	};

	const handleConfirmUpdate = () => {
		const finalEnv = { ...editedEnv };
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

	const handleNewKeyChange = (rawValue: string) => {
		const equalIndex = rawValue.indexOf("=");
		if (equalIndex === -1) {
			setNewKey(rawValue);
			return;
		}

		const key = rawValue.substring(0, equalIndex).trim();
		const rawTail = rawValue.substring(equalIndex + 1);
		let value = rawTail;
		if (
			(value.startsWith('"') && value.endsWith('"')) ||
			(value.startsWith("'") && value.endsWith("'"))
		) {
			value = value.slice(1, -1);
		}

		setNewKey(key);
		// Only overwrite an existing value when the pasted segment after `=` is
		// non-empty; otherwise we'd wipe text the user already typed in Value.
		// Check rawTail (pre-strip) so explicit empty quotes like KEY="" still
		// clear the value field.
		if (rawTail.length > 0) {
			setNewValue(value);
		}
		// Move focus to the value field so the user can keep editing if needed.
		requestAnimationFrame(() => {
			const input = newValueInputRef.current;
			if (!input) return;
			input.focus();
			const caret = input.value.length;
			input.setSelectionRange(caret, caret);
		});
	};

	// Pasting a whole .env (multiple KEY=value lines) into the key field
	// imports every pair at once — new keys added, existing keys updated.
	// Must intercept the paste: text inputs strip newlines before onChange.
	const handleKeyPaste = (event: React.ClipboardEvent<HTMLInputElement>) => {
		const text = event.clipboardData.getData("text");
		if (!text.includes("\n")) return;

		event.preventDefault();
		const parsed = parseEnvFile(text);
		const count = Object.keys(parsed).length;
		if (count === 0) {
			toast.error("No KEY=value pairs found in pasted text");
			return;
		}

		setEditedEnv((prev) => ({ ...prev, ...parsed }));
		setModifiedKeys((prev) => new Set([...prev, ...Object.keys(parsed)]));
		setNewKey("");
		setNewValue("");
		setShowAddNew(false);
		toast.success(
			`Imported ${count} variable${count === 1 ? "" : "s"} from pasted text`,
		);
	};

	const handleAddNew = () => {
		if (newKey.trim() && !editedEnv[newKey]) {
			setEditedEnv((prev) => ({ ...prev, [newKey]: newValue }));
			setModifiedKeys((prev) => new Set(prev).add(newKey));
			setNewKey("");
			setNewValue("");
			setShowAddNew(false);
		} else if (editedEnv[newKey]) {
			toast.error("Key already exists");
		} else {
			toast.error("Key cannot be empty");
		}
	};

	const handleFileUpload = (event: React.ChangeEvent<HTMLInputElement>) => {
		const file = event.target.files?.[0];
		if (!file) return;

		const reader = new FileReader();
		reader.onload = (e) => {
			const content = e.target?.result;
			if (typeof content !== "string") {
				toast.error("Failed to parse .env file");
				return;
			}
			setParsedEnvFile(parseEnvFile(content));
			setShowUploadPreview(true);
		};
		reader.readAsText(file);

		// Reset the input so the same file can be selected again
		event.target.value = "";
	};

	const handleConfirmUpload = () => {
		setEditedEnv((prev) => ({ ...prev, ...parsedEnvFile }));
		setModifiedKeys(
			(prev) => new Set([...prev, ...Object.keys(parsedEnvFile)]),
		);

		setShowUploadPreview(false);
		setParsedEnvFile({});

		toast.success(
			`Imported ${Object.keys(parsedEnvFile).length} variables from .env file`,
		);
	};

	const handleCancelUpload = () => {
		setShowUploadPreview(false);
		setParsedEnvFile({});
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
		([key]) => !deletedKeys.has(key),
	);
	if (isEditing) {
		// Surface modified keys first; entries with equal status keep their order.
		envEntries.sort(([keyA], [keyB]) => {
			const aIsModified = modifiedKeys.has(keyA);
			const bIsModified = modifiedKeys.has(keyB);
			if (aIsModified && !bIsModified) return -1;
			if (!aIsModified && bIsModified) return 1;
			return 0;
		});
	}

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
			<input
				type="file"
				ref={fileInputRef}
				onChange={handleFileUpload}
				accept=".env"
				className="hidden"
			/>

			<div className="flex items-center gap-2 justify-end border-b pb-2">
				{!isEditing ? (
					<TooltipProvider>
						<Tooltip>
							<TooltipTrigger asChild>
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
							</TooltipTrigger>
							{isReadOnly && (
								<TooltipContent>Cannot edit in read-only mode</TooltipContent>
							)}
						</Tooltip>
					</TooltipProvider>
				) : (
					<>
						<Button
							variant="outline"
							size="sm"
							onClick={() => fileInputRef.current?.click()}
							className="h-8"
						>
							<Upload className="mr-2 h-3.5 w-3.5" />
							Upload .env
						</Button>
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

			{/* Variables list: one header row, then dense key/value rows */}
			{(envEntries.length > 0 || showAddNew) && (
				<div>
					<div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1.5fr)_2rem] items-center gap-2 border-b pb-1.5 text-xs font-medium text-muted-foreground">
						<span>Key</span>
						<span>Value</span>
						<span />
					</div>

					{showAddNew && (
						<div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1.5fr)_2rem] items-center gap-2 border-b py-1.5">
							<Input
								value={newKey}
								onChange={(e) => handleNewKeyChange(e.target.value)}
								onPaste={handleKeyPaste}
								placeholder="VARIABLE_NAME, KEY=value, or paste a .env"
								aria-label="Key"
								className="h-8 font-mono text-xs"
							/>
							<Input
								ref={newValueInputRef}
								value={newValue}
								onChange={(e) => setNewValue(e.target.value)}
								onKeyDown={(e) => {
									if (e.key === "Enter") handleAddNew();
								}}
								placeholder="value"
								aria-label="Value"
								className="h-8 font-mono text-xs"
							/>
							<div className="flex gap-0.5">
								<Button
									variant="ghost"
									size="icon"
									onClick={handleAddNew}
									className="h-8 w-8 text-primary hover:text-primary"
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
									className="h-8 w-8 text-muted-foreground"
									title="Cancel"
								>
									<X className="h-3.5 w-3.5" />
								</Button>
							</div>
						</div>
					)}

					{envEntries.map(([key, value]) => {
						const isModified = modifiedKeys.has(key);
						return (
							<div
								key={key}
								className={`grid grid-cols-[minmax(0,1fr)_minmax(0,1.5fr)_2rem] items-center gap-2 border-b py-1.5 transition-colors last:border-b-0 ${
									isModified && isEditing ? "bg-primary/5" : "hover:bg-muted/30"
								}`}
							>
								<div className="flex min-w-0 items-center gap-2">
									<span
										className="truncate font-mono text-xs font-medium"
										title={key}
									>
										{key}
									</span>
									{isModified && isEditing && (
										<Badge variant="default" className="h-4 px-1.5 text-[10px]">
											New
										</Badge>
									)}
								</div>
								{isEditing ? (
									<Input
										value={value}
										onChange={(e) => handleValueChange(key, e.target.value)}
										aria-label={`Value for ${key}`}
										className="h-8 font-mono text-xs"
									/>
								) : (
									<span
										className="truncate font-mono text-xs text-muted-foreground"
										title={value}
									>
										{value}
									</span>
								)}
								{isEditing ? (
									<Button
										variant="ghost"
										size="icon"
										onClick={() => handleDelete(key)}
										className="h-8 w-8 text-muted-foreground hover:text-destructive"
										title="Remove variable"
									>
										<Trash2 className="h-3.5 w-3.5" />
									</Button>
								) : (
									<span />
								)}
							</div>
						);
					})}
				</div>
			)}

			{envEntries.length === 0 && isEditing && !showAddNew && (
				<div className="flex items-center justify-center py-4 text-sm text-muted-foreground">
					No environment variables. Click "Add" to create one.
				</div>
			)}

			<EnvUploadPreviewDialog
				open={showUploadPreview}
				onOpenChange={setShowUploadPreview}
				parsedEnv={parsedEnvFile}
				currentEnv={editedEnv}
				onConfirm={handleConfirmUpload}
				onCancel={handleCancelUpload}
			/>

			<EnvUpdateConfirmDialog
				open={showConfirmDialog}
				onOpenChange={setShowConfirmDialog}
				isCoolifyManaged={isCoolifyManaged}
				onConfirm={handleConfirmUpdate}
			/>
		</div>
	);
}
