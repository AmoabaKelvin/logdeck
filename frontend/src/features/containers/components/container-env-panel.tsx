import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useRef, useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { CopyButton } from "@/components/ui/copy-button";
import {
	EyeIcon,
	EyeOffIcon,
	PlusIcon,
	RotateCcwIcon,
	SearchIcon,
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
import { isSecretKey, parseEnvFile } from "./env-file";
import { EnvUpdateConfirmDialog } from "./env-update-confirm-dialog";
import { EnvUploadPreviewDialog } from "./env-upload-preview-dialog";

interface ContainerEnvPanelProps {
	containerId: string;
	containerHost: string;
	isReadOnly?: boolean;
	isCoolifyManaged?: boolean;
	onContainerIdChange?: (newContainerId: string) => void;
}

const ROW = "grid gap-x-2 sm:grid-cols-[minmax(9rem,18rem)_minmax(0,1fr)_auto]";

/**
 * A value reads as text until you touch it. Giving every row a boxed input
 * turns the list into a wall of rectangles; the border arrives on hover and
 * focus, where it means something.
 */
const ICON_BUTTON = "transition-transform active:scale-90";

const VALUE_INPUT =
	"h-9 border-transparent bg-transparent px-2 font-mono shadow-none hover:border-input focus-visible:border-ring disabled:opacity-100";

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
	// Only what the user changed, so the original stays the source of truth and
	// "is this edited?" needs no bookkeeping.
	const [edits, setEdits] = useState<Record<string, string>>({});
	const [removed, setRemoved] = useState<Set<string>>(new Set());
	const [draftEntry, setDraftEntry] = useState("");
	const [isAdding, setIsAdding] = useState(false);
	const [showConfirmDialog, setShowConfirmDialog] = useState(false);
	const [showUploadPreview, setShowUploadPreview] = useState(false);
	const [uploadedEnv, setUploadedEnv] = useState<Record<string, string>>({});
	// Rows that just arrived flash once so a change lands somewhere visible in a
	// list that may be scrolled well away from the add control.
	const [justAdded, setJustAdded] = useState<Set<string>>(new Set());
	const fileInputRef = useRef<HTMLInputElement>(null);
	const listRef = useRef<HTMLDivElement>(null);

	const {
		data: original,
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
			discard();

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

	function discard() {
		setEdits({});
		setRemoved(new Set());
		setJustAdded(new Set());
		setIsAdding(false);
		setDraftEntry("");
	}

	// A variable added while the list is scrolled elsewhere would land out of
	// sight, so the list goes to it and the row flashes on arrival.
	useEffect(() => {
		const [first] = justAdded;
		if (!first) return;
		// One frame late, so the row is measured where it finally sits rather
		// than where it landed before the list re-sorted around it.
		const frame = requestAnimationFrame(() => {
			listRef.current
				?.querySelector(`[data-env-key="${CSS.escape(first)}"]`)
				?.scrollIntoView({
					block: "center",
					behavior: window.matchMedia("(prefers-reduced-motion: reduce)")
						.matches
						? "auto"
						: "smooth",
				});
		});
		return () => cancelAnimationFrame(frame);
	}, [justAdded]);

	const base = original ?? {};
	const effective = useMemo(() => ({ ...base, ...edits }), [base, edits]);

	const entries = useMemo(() => {
		const query = filter.trim().toLowerCase();
		return Object.entries(effective)
			.filter(
				([key, value]) =>
					!query ||
					key.toLowerCase().includes(query) ||
					value.toLowerCase().includes(query),
			)
			.sort(([a], [b]) => a.localeCompare(b));
	}, [effective, filter]);

	const addedCount = Object.keys(edits).filter((key) => !(key in base)).length;
	const editedCount = Object.keys(edits).filter(
		(key) => key in base && base[key] !== edits[key],
	).length;
	const isDirty = addedCount + editedCount + removed.size > 0;
	const anySecret = entries.some(([key]) => isSecretKey(key));
	const allRevealed =
		anySecret &&
		entries
			.filter(([key]) => isSecretKey(key))
			.every(([key]) => revealed.has(key));

	const applyImport = (imported: Record<string, string>) => {
		const keys = Object.keys(imported);
		setEdits((prev) => ({ ...prev, ...imported }));
		setRemoved((prev) => {
			const next = new Set(prev);
			for (const key of keys) next.delete(key);
			return next;
		});
		setJustAdded(new Set(keys));
		// Long enough for the flash to finish; re-adding the same key restarts it
		// because the class is removed in between.
		setTimeout(() => setJustAdded(new Set()), 1300);
	};

	// One field, parsed on submit. Splitting name from value while the user is
	// still typing meant moving focus mid-keystroke, which raced anyone typing
	// faster than a frame and spilled the value into the name.
	const handleAdd = () => {
		const text = draftEntry.trim();
		if (!text) {
			toast.error("Enter a name, or NAME=value");
			return;
		}

		const equalIndex = text.indexOf("=");
		const key = (equalIndex === -1 ? text : text.slice(0, equalIndex)).trim();
		if (!key) {
			toast.error("Name cannot be empty");
			return;
		}
		if (key in effective && !removed.has(key)) {
			toast.error(`${key} is already set`);
			return;
		}

		let value = equalIndex === -1 ? "" : text.slice(equalIndex + 1).trim();
		if (
			(value.startsWith('"') && value.endsWith('"')) ||
			(value.startsWith("'") && value.endsWith("'"))
		) {
			value = value.slice(1, -1);
		}

		applyImport({ [key]: value });
		setDraftEntry("");
		setIsAdding(false);
	};

	// Pasting a whole .env imports every pair at once. The paste must be
	// intercepted: text inputs strip newlines before onChange.
	const handlePaste = (event: React.ClipboardEvent<HTMLInputElement>) => {
		const text = event.clipboardData.getData("text");
		if (!text.includes("\n")) return;

		event.preventDefault();
		const parsed = parseEnvFile(text);
		const count = Object.keys(parsed).length;
		if (count === 0) {
			toast.error("No KEY=value pairs found in pasted text");
			return;
		}

		applyImport(parsed);
		setDraftEntry("");
		setIsAdding(false);
		toast.success(`Imported ${count} variable${count === 1 ? "" : "s"}`);
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
			setUploadedEnv(parseEnvFile(content));
			setShowUploadPreview(true);
		};
		reader.readAsText(file);

		// Reset the input so the same file can be picked again.
		event.target.value = "";
	};

	if (isLoading && !original) {
		return <PanelLoading label="Reading environment…" />;
	}
	if (error) {
		return (
			<PanelError>Could not read this container's environment.</PanelError>
		);
	}

	const changeSummary = [
		editedCount && `${editedCount} changed`,
		addedCount && `${addedCount} added`,
		removed.size && `${removed.size} removed`,
	]
		.filter(Boolean)
		.join(" · ");

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
					{filter.trim()
						? `${entries.length} of ${Object.keys(effective).length}`
						: `${Object.keys(effective).length} variable${Object.keys(effective).length === 1 ? "" : "s"}`}
				</p>

				<div className="ml-auto flex flex-wrap items-center gap-2">
					{anySecret && (
						<Button
							variant="ghost"
							onClick={() =>
								setRevealed(
									allRevealed ? new Set() : new Set(entries.map(([k]) => k)),
								)
							}
							className="h-10 py-2 pr-3 pl-2 text-base text-muted-foreground hover:text-foreground sm:h-9 sm:text-sm"
						>
							{allRevealed ? (
								<EyeOffIcon className="size-4" />
							) : (
								<EyeIcon className="size-4" />
							)}
							{allRevealed ? "Hide secrets" : "Reveal secrets"}
						</Button>
					)}
					{!isReadOnly && (
						<Button
							variant="ghost"
							onClick={() => fileInputRef.current?.click()}
							className="h-10 py-2 pr-3 pl-2 text-base text-muted-foreground hover:text-foreground sm:h-9 sm:text-sm"
						>
							<UploadIcon className="size-4" />
							Import .env
						</Button>
					)}
				</div>
			</div>

			{entries.length === 0 && !isAdding ? (
				<div className="mt-4">
					<PanelNote>
						{Object.keys(effective).length === 0
							? "This container has no environment variables."
							: "No variables match that filter."}
					</PanelNote>
				</div>
			) : (
				<div
					ref={listRef}
					className="mt-3 max-h-96 divide-y divide-border/60 overflow-y-auto"
				>
					{entries.map(([key, value]) => {
						const isRemoved = removed.has(key);
						const isChanged = key in edits;
						const secret = isSecretKey(key);
						const hidden = secret && !revealed.has(key);

						return (
							<div
								key={key}
								data-env-key={key}
								className={`group ${ROW} items-center border-l-2 py-1 pl-2 ${
									isChanged && !isRemoved
										? "border-l-amber-500"
										: "border-l-transparent"
								} ${isRemoved ? "opacity-60" : ""} ${
									justAdded.has(key) ? "row-added" : ""
								}`}
							>
								<span
									className={`truncate font-mono text-base sm:text-sm ${
										isRemoved ? "line-through" : ""
									}`}
									title={key}
								>
									{key}
								</span>

								{isRemoved ? (
									<span className="truncate px-2 font-mono text-base text-muted-foreground line-through sm:text-sm">
										{hidden ? "••••••••" : value}
									</span>
								) : (
									<Input
										type={hidden ? "password" : "text"}
										autoComplete="off"
										spellCheck={false}
										name={`env-${key}`}
										aria-label={`Value for ${key}`}
										value={value}
										disabled={isReadOnly}
										onChange={(e) =>
											setEdits((prev) => ({ ...prev, [key]: e.target.value }))
										}
										className={VALUE_INPUT}
									/>
								)}

								<div className="flex items-center justify-end">
									{secret && !isRemoved && (
										<Button
											variant="ghost"
											size="icon-sm"
											onClick={() =>
												setRevealed((prev) => {
													const next = new Set(prev);
													if (next.has(key)) next.delete(key);
													else next.add(key);
													return next;
												})
											}
											aria-label={`${hidden ? "Reveal" : "Hide"} ${key}`}
										>
											{hidden ? (
												<EyeIcon className="size-4 text-muted-foreground" />
											) : (
												<EyeOffIcon className="size-4 text-muted-foreground" />
											)}
										</Button>
									)}
									<CopyButton value={value} label={`Copy ${key}`} />
									{!isReadOnly &&
										(isRemoved ? (
											<Button
												variant="ghost"
												size="icon-sm"
												onClick={() =>
													setRemoved((prev) => {
														const next = new Set(prev);
														next.delete(key);
														return next;
													})
												}
												aria-label={`Keep ${key}`}
											>
												<RotateCcwIcon className="size-4 text-muted-foreground" />
											</Button>
										) : (
											<Button
												variant="ghost"
												size="icon-sm"
												onClick={() => {
													setRemoved((prev) => new Set(prev).add(key));
													setEdits((prev) => {
														// A key added in this session just goes away.
														if (key in base) return prev;
														const next = { ...prev };
														delete next[key];
														return next;
													});
												}}
												aria-label={`Remove ${key}`}
												className="opacity-0 group-hover:opacity-100 focus-visible:opacity-100 hover:text-destructive"
											>
												<XIcon className="size-4 text-muted-foreground" />
											</Button>
										))}
								</div>
							</div>
						);
					})}
				</div>
			)}

			{!isReadOnly &&
				(isAdding ? (
					<div className="mt-3 flex animate-in items-center gap-2 fade-in pl-2 duration-200 slide-in-from-top-1">
						<Input
							autoFocus
							value={draftEntry}
							onChange={(e) => setDraftEntry(e.target.value)}
							onPaste={handlePaste}
							onKeyDown={(e) => {
								if (e.key === "Enter") handleAdd();
								if (e.key === "Escape") {
									setIsAdding(false);
									setDraftEntry("");
								}
							}}
							placeholder="NAME=value, or paste a .env"
							aria-label="New variable"
							className="h-10 max-w-lg font-mono sm:h-9"
						/>
						<Button onClick={handleAdd} className="h-10 sm:h-9">
							Add
						</Button>
						<Button
							variant="ghost"
							size="icon-sm"
							onClick={() => {
								setIsAdding(false);
								setDraftEntry("");
							}}
							aria-label="Cancel adding a variable"
							className={`${ICON_BUTTON} size-10 sm:size-9`}
						>
							<XIcon className="size-4" />
						</Button>
					</div>
				) : (
					<Tooltip>
						<TooltipTrigger asChild>
							<Button
								variant="ghost"
								size="icon-sm"
								onClick={() => setIsAdding(true)}
								aria-label="Add variable"
								className={`${ICON_BUTTON} mt-2 ml-0.5 text-muted-foreground hover:text-foreground`}
							>
								<PlusIcon className="size-4" />
							</Button>
						</TooltipTrigger>
						<TooltipContent>Add variable</TooltipContent>
					</Tooltip>
				))}

			{isDirty && (
				<div className="mt-4 flex flex-wrap items-center gap-3 border-t border-border/70 pt-4 animate-in fade-in slide-in-from-bottom-1 duration-200">
					<p className="text-base sm:text-sm">
						{changeSummary}
						<span className="ml-1.5 text-muted-foreground">
							— saving recreates the container under a new ID.
						</span>
					</p>
					<div className="ml-auto flex items-center gap-2">
						<Button
							variant="ghost"
							onClick={discard}
							disabled={updateMutation.isPending}
							className="h-10 text-base sm:h-9 sm:text-sm"
						>
							Discard
						</Button>
						<Button
							onClick={() => setShowConfirmDialog(true)}
							disabled={updateMutation.isPending}
							className="h-10 text-base sm:h-9 sm:text-sm"
						>
							{updateMutation.isPending && <Spinner className="size-4" />}
							Save and recreate
						</Button>
					</div>
				</div>
			)}

			<EnvUploadPreviewDialog
				open={showUploadPreview}
				onOpenChange={setShowUploadPreview}
				parsedEnv={uploadedEnv}
				currentEnv={effective}
				onConfirm={() => {
					applyImport(uploadedEnv);
					setShowUploadPreview(false);
					setUploadedEnv({});
					toast.success(
						`Imported ${Object.keys(uploadedEnv).length} variables from .env`,
					);
				}}
				onCancel={() => {
					setShowUploadPreview(false);
					setUploadedEnv({});
				}}
			/>

			<EnvUpdateConfirmDialog
				open={showConfirmDialog}
				onOpenChange={setShowConfirmDialog}
				isCoolifyManaged={isCoolifyManaged}
				onConfirm={() => {
					const finalEnv = { ...effective };
					removed.forEach((key) => {
						delete finalEnv[key];
					});
					updateMutation.mutate(finalEnv);
					setShowConfirmDialog(false);
				}}
			/>
		</div>
	);
}
