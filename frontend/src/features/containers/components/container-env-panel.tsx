import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo, useRef, useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { CopyButton } from "@/components/ui/copy-button";
import {
	EyeIcon,
	EyeOffIcon,
	PencilIcon,
	PlusIcon,
	SearchIcon,
	Trash2Icon,
	UploadIcon,
	XIcon,
} from "@/components/ui/icons";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import {
	Tooltip,
	TooltipContent,
	TooltipTrigger,
} from "@/components/ui/tooltip";

import { getContainerEnvVariables } from "../api/get-container-env-variables";
import { updateContainerEnvVariables } from "../api/update-container-env-variables";
import { PanelError, PanelLoading, PanelNote } from "./container-panel-ui";
import { isSecretKey, MASKED_VALUE, parseEnvFile } from "./env-file";
import { EnvUpdateConfirmDialog } from "./env-update-confirm-dialog";
import { EnvUploadPreviewDialog } from "./env-upload-preview-dialog";

interface ContainerEnvPanelProps {
	containerId: string;
	containerHost: string;
	isReadOnly?: boolean;
	isCoolifyManaged?: boolean;
	onContainerIdChange?: (newContainerId: string) => void;
}

const rowClass =
	"grid grid-cols-[minmax(0,1fr)] items-center gap-x-3 gap-y-1 py-1.5 sm:grid-cols-[minmax(8rem,18rem)_minmax(0,1fr)_auto]";

export function ContainerEnvPanel({
	containerId,
	containerHost,
	isReadOnly = false,
	isCoolifyManaged = false,
	onContainerIdChange,
}: ContainerEnvPanelProps) {
	const queryClient = useQueryClient();
	const [filter, setFilter] = useState("");
	const [revealed, setRevealed] = useState<Set<string>>(new Set());
	const [isEditing, setIsEditing] = useState(false);
	const [draft, setDraft] = useState<Record<string, string>>({});
	const [deletedKeys, setDeletedKeys] = useState<Set<string>>(new Set());
	const [touchedKeys, setTouchedKeys] = useState<Set<string>>(new Set());
	const [newKey, setNewKey] = useState("");
	const [newValue, setNewValue] = useState("");
	const [showAddNew, setShowAddNew] = useState(false);
	const [showConfirmDialog, setShowConfirmDialog] = useState(false);
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
			// Invalidate for BOTH the old and new container IDs: the update
			// recreates the container under a new one.
			queryClient.invalidateQueries({
				queryKey: ["container-env", containerId],
			});
			queryClient.invalidateQueries({
				queryKey: ["container-env", result.newContainerId],
			});
			queryClient.invalidateQueries({ queryKey: ["containers"] });

			onContainerIdChange?.(result.newContainerId);
			resetEditing();

			if (result.coolifySynced === true) {
				toast.success("Environment updated", {
					description: "Container recreated and changes synced to Coolify.",
				});
			} else if (result.coolifySynced === false) {
				toast.warning("Updated, but Coolify sync failed", {
					description:
						result.coolifyError ||
						"Container recreated, but Coolify was not updated. Changes may be lost on redeployment.",
				});
			} else {
				toast.success("Environment updated", {
					description: "Container recreated with the new environment.",
				});
			}
		},
		onError: (mutationError: Error) => {
			toast.error("Failed to update environment", {
				description: mutationError.message,
			});
		},
	});

	function resetEditing() {
		setIsEditing(false);
		setDraft({});
		setDeletedKeys(new Set());
		setTouchedKeys(new Set());
		setShowAddNew(false);
		setNewKey("");
		setNewValue("");
	}

	const source = isEditing ? draft : (envVariables ?? {});
	const entries = useMemo(() => {
		const query = filter.trim().toLowerCase();
		const list = Object.entries(source).filter(
			([key]) => !deletedKeys.has(key),
		);
		const matched = query
			? list.filter(
					([key, value]) =>
						key.toLowerCase().includes(query) ||
						value.toLowerCase().includes(query),
				)
			: list;
		// Touched rows float to the top while editing so a long list does not
		// hide what you just changed.
		return matched.sort(([a], [b]) => {
			if (!isEditing) return a.localeCompare(b);
			const aTouched = touchedKeys.has(a);
			const bTouched = touchedKeys.has(b);
			if (aTouched !== bTouched) return aTouched ? -1 : 1;
			return a.localeCompare(b);
		});
	}, [source, deletedKeys, filter, isEditing, touchedKeys]);

	const total = Object.keys(source).filter(
		(key) => !deletedKeys.has(key),
	).length;
	const anySecret = entries.some(([key]) => isSecretKey(key));
	const allRevealed = entries
		.filter(([key]) => isSecretKey(key))
		.every(([key]) => revealed.has(key));

	const handleAddNew = () => {
		const key = newKey.trim();
		if (!key) {
			toast.error("Key cannot be empty");
			return;
		}
		if (draft[key] !== undefined && !deletedKeys.has(key)) {
			toast.error("Key already exists");
			return;
		}
		setDraft((prev) => ({ ...prev, [key]: newValue }));
		setTouchedKeys((prev) => new Set(prev).add(key));
		setDeletedKeys((prev) => {
			const next = new Set(prev);
			next.delete(key);
			return next;
		});
		setNewKey("");
		setNewValue("");
		setShowAddNew(false);
	};

	// Pasting a whole .env into the key field imports every pair at once. The
	// paste must be intercepted: text inputs strip newlines before onChange.
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

		setDraft((prev) => ({ ...prev, ...parsed }));
		setTouchedKeys((prev) => new Set([...prev, ...Object.keys(parsed)]));
		setNewKey("");
		setNewValue("");
		setShowAddNew(false);
		toast.success(`Imported ${count} variable${count === 1 ? "" : "s"}`);
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
		// Only overwrite the value field when the pasted tail is non-empty;
		// otherwise typing "FOO=" would wipe text already entered there.
		if (rawTail.length > 0) {
			setNewValue(value);
		}
		requestAnimationFrame(() => {
			const input = newValueInputRef.current;
			if (!input) return;
			input.focus();
			const caret = input.value.length;
			input.setSelectionRange(caret, caret);
		});
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

		// Reset the input so the same file can be picked again.
		event.target.value = "";
	};

	if (isLoading && !envVariables) {
		return <PanelLoading label="Reading environment…" />;
	}
	if (error) {
		return (
			<PanelError>Could not read this container's environment.</PanelError>
		);
	}

	return (
		<div>
			<input
				type="file"
				ref={fileInputRef}
				onChange={handleFileUpload}
				accept=".env"
				className="hidden"
			/>

			<div className="flex flex-wrap items-center gap-2">
				<div className="relative min-w-48 flex-1 sm:max-w-xs">
					<SearchIcon className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
					<Input
						type="search"
						name="env-filter"
						aria-label="Filter environment variables"
						placeholder="Filter…"
						value={filter}
						onChange={(e) => setFilter(e.target.value)}
						className="h-10 pl-8 sm:h-9"
					/>
				</div>

				<p className="text-base text-muted-foreground tabular-nums sm:text-sm">
					{entries.length === total
						? `${total} variable${total === 1 ? "" : "s"}`
						: `${entries.length} of ${total}`}
				</p>

				<div className="ml-auto flex flex-wrap items-center gap-2">
					{anySecret && (
						<Button
							variant="outline"
							onClick={() =>
								setRevealed(
									allRevealed
										? new Set()
										: new Set(entries.map(([key]) => key)),
								)
							}
							className="h-10 py-2 pr-3 pl-2 text-base sm:h-9 sm:text-sm"
						>
							{allRevealed ? (
								<EyeOffIcon className="size-4" />
							) : (
								<EyeIcon className="size-4" />
							)}
							{allRevealed ? "Hide values" : "Reveal values"}
						</Button>
					)}

					{isEditing ? (
						<>
							<Button
								variant="outline"
								onClick={() => fileInputRef.current?.click()}
								className="h-10 py-2 pr-3 pl-2 text-base sm:h-9 sm:text-sm"
							>
								<UploadIcon className="size-4" />
								Upload .env
							</Button>
							<Button
								variant="outline"
								onClick={() => setShowAddNew(true)}
								className="h-10 py-2 pr-3 pl-2 text-base sm:h-9 sm:text-sm"
							>
								<PlusIcon className="size-4" />
								Add
							</Button>
							<Button
								variant="ghost"
								onClick={resetEditing}
								className="h-10 text-base sm:h-9 sm:text-sm"
							>
								Cancel
							</Button>
							<Button
								onClick={() => setShowConfirmDialog(true)}
								disabled={updateMutation.isPending}
								className="h-10 text-base sm:h-9 sm:text-sm"
							>
								{updateMutation.isPending && <Spinner className="size-4" />}
								Save and recreate
							</Button>
						</>
					) : (
						<Tooltip>
							<TooltipTrigger asChild>
								<span className="inline-block">
									<Button
										variant="outline"
										disabled={isReadOnly}
										onClick={() => {
											setDraft({ ...envVariables });
											setDeletedKeys(new Set());
											setTouchedKeys(new Set());
											setIsEditing(true);
										}}
										className="h-10 py-2 pr-3 pl-2 text-base sm:h-9 sm:text-sm"
									>
										<PencilIcon className="size-4" />
										Edit
									</Button>
								</span>
							</TooltipTrigger>
							{isReadOnly && (
								<TooltipContent>LogDeck is in read-only mode</TooltipContent>
							)}
						</Tooltip>
					)}
				</div>
			</div>

			{isEditing && (
				<p className="mt-3 text-base text-muted-foreground sm:text-sm">
					Saving recreates the container — it will get a new ID and restart.
				</p>
			)}

			{showAddNew && (
				<div className={`${rowClass} mt-3 border-b border-border/60 pb-3`}>
					<Input
						value={newKey}
						onChange={(e) => handleNewKeyChange(e.target.value)}
						onPaste={handleKeyPaste}
						placeholder="VARIABLE_NAME, KEY=value, or paste a .env"
						aria-label="New variable name"
						className="h-10 font-mono sm:h-9"
					/>
					<Input
						ref={newValueInputRef}
						value={newValue}
						onChange={(e) => setNewValue(e.target.value)}
						onKeyDown={(e) => {
							if (e.key === "Enter") handleAddNew();
						}}
						placeholder="value"
						aria-label="New variable value"
						className="h-10 font-mono sm:h-9"
					/>
					<div className="flex items-center gap-1">
						<Button
							variant="outline"
							onClick={handleAddNew}
							className="h-10 sm:h-9"
						>
							Add
						</Button>
						<Button
							variant="ghost"
							size="icon-sm"
							onClick={() => {
								setShowAddNew(false);
								setNewKey("");
								setNewValue("");
							}}
							aria-label="Cancel adding a variable"
							className="size-10 sm:size-9"
						>
							<XIcon className="size-4" />
						</Button>
					</div>
				</div>
			)}

			{entries.length === 0 ? (
				<div className="mt-4">
					<PanelNote>
						{total === 0
							? "This container has no environment variables."
							: "No variables match that filter."}
					</PanelNote>
				</div>
			) : (
				<div className="mt-3 max-h-96 divide-y divide-border/60 overflow-y-auto">
					{entries.map(([key, value]) => {
						const secret = isSecretKey(key);
						const hidden = secret && !revealed.has(key) && !isEditing;
						return (
							<div key={key} className={rowClass}>
								<div className="flex min-w-0 items-center gap-2">
									<span
										className="truncate font-mono text-base sm:text-sm"
										title={key}
									>
										{key}
									</span>
									{touchedKeys.has(key) && isEditing && (
										<span className="shrink-0 text-xs text-muted-foreground">
											edited
										</span>
									)}
								</div>

								{isEditing ? (
									<Input
										value={value}
										onChange={(e) => {
											const next = e.target.value;
											setDraft((prev) => ({ ...prev, [key]: next }));
											setTouchedKeys((prev) => new Set(prev).add(key));
										}}
										aria-label={`Value for ${key}`}
										className="h-10 font-mono sm:h-9"
									/>
								) : (
									<span
										className="truncate font-mono text-base text-muted-foreground sm:text-sm"
										title={hidden ? undefined : value}
									>
										{hidden ? MASKED_VALUE : value || "—"}
									</span>
								)}

								<div className="flex items-center justify-end gap-0.5">
									{isEditing ? (
										<Button
											variant="ghost"
											size="icon-sm"
											onClick={() =>
												setDeletedKeys((prev) => new Set(prev).add(key))
											}
											aria-label={`Remove ${key}`}
											className="text-muted-foreground hover:text-destructive"
										>
											<Trash2Icon className="size-4" />
										</Button>
									) : (
										<>
											{secret && (
												<Button
													variant="ghost"
													size="icon-sm"
													onClick={() =>
														setRevealed((prev) => {
															const next = new Set(prev);
															if (next.has(key)) {
																next.delete(key);
															} else {
																next.add(key);
															}
															return next;
														})
													}
													aria-label={`${revealed.has(key) ? "Hide" : "Reveal"} ${key}`}
												>
													{revealed.has(key) ? (
														<EyeOffIcon className="size-4 text-muted-foreground" />
													) : (
														<EyeIcon className="size-4 text-muted-foreground" />
													)}
												</Button>
											)}
											<CopyButton value={value} label={`Copy ${key}`} />
										</>
									)}
								</div>
							</div>
						);
					})}
				</div>
			)}

			<EnvUploadPreviewDialog
				open={showUploadPreview}
				onOpenChange={setShowUploadPreview}
				parsedEnv={parsedEnvFile}
				currentEnv={draft}
				onConfirm={() => {
					setDraft((prev) => ({ ...prev, ...parsedEnvFile }));
					setTouchedKeys(
						(prev) => new Set([...prev, ...Object.keys(parsedEnvFile)]),
					);
					setShowUploadPreview(false);
					setParsedEnvFile({});
					toast.success(
						`Imported ${Object.keys(parsedEnvFile).length} variables from .env`,
					);
				}}
				onCancel={() => {
					setShowUploadPreview(false);
					setParsedEnvFile({});
				}}
			/>

			<EnvUpdateConfirmDialog
				open={showConfirmDialog}
				onOpenChange={setShowConfirmDialog}
				isCoolifyManaged={isCoolifyManaged}
				onConfirm={() => {
					const finalEnv = { ...draft };
					deletedKeys.forEach((key) => {
						delete finalEnv[key];
					});
					updateMutation.mutate(finalEnv);
					setShowConfirmDialog(false);
				}}
			/>
		</div>
	);
}
