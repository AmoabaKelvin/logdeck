import { useRef, useState } from "react";
import { toast } from "sonner";

import {
	AlertDialog,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	DownloadIcon,
	RotateCcwIcon,
	Trash2Icon,
	UploadIcon,
} from "@/components/ui/icons";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import { formatBytes } from "@/features/containers/components/container-utils";

import { backupDownloadUrl } from "../api/get-backups";
import type { RestoreSource } from "../api/restore-backup";
import {
	useBackups,
	useCancelRestore,
	useCheckRestore,
	useConfirmRestore,
	useCreateBackup,
	useDeleteBackup,
	useUpdateBackupSchedule,
} from "../hooks/use-backups";
import type {
	BackupContents,
	BackupEntry,
	BackupSchedule,
	BackupStatus,
	RestorePreview,
} from "../types";
import { describeContents, validateSchedule } from "./backup-utils";
import { showResultToast } from "./mutation-toast";
import { SaveButton } from "./save-button";
import {
	ErrorNote,
	Field,
	Note,
	SettingsSection,
	SettingsSubsection,
	SettingsTable,
	TBody,
	Td,
	Th,
	THead,
	Well,
} from "./settings-ui";

function formatDate(value: string) {
	return new Date(value).toLocaleString(undefined, {
		dateStyle: "medium",
		timeStyle: "short",
	});
}

function Toggle({
	id,
	label,
	checked,
	disabled,
	onCheckedChange,
}: {
	id: string;
	label: string;
	checked: boolean;
	disabled?: boolean;
	onCheckedChange: (checked: boolean) => void;
}) {
	return (
		<label
			htmlFor={id}
			className="flex items-center gap-2.5 text-base sm:text-sm"
		>
			<Switch
				id={id}
				checked={checked}
				disabled={disabled}
				onCheckedChange={onCheckedChange}
			/>
			{label}
		</label>
	);
}

function CreateBackupForm({ status }: { status: BackupStatus }) {
	const [includeLogs, setIncludeLogs] = useState(false);
	const [includeSecrets, setIncludeSecrets] = useState(false);
	const [passphrase, setPassphrase] = useState("");
	const createMutation = useCreateBackup();
	const busy = status.running || createMutation.isPending;

	return (
		<SettingsSubsection title="Back up now">
			<div className="space-y-4">
				<div className="flex flex-wrap gap-x-6 gap-y-3">
					<Toggle
						id="backup-include-logs"
						label="Include stored logs"
						checked={includeLogs && status.logsAvailable}
						disabled={!status.logsAvailable}
						onCheckedChange={setIncludeLogs}
					/>
					<Toggle
						id="backup-include-secrets"
						label="Include credentials"
						checked={includeSecrets}
						onCheckedChange={setIncludeSecrets}
					/>
				</div>
				<div className="max-w-sm">
					<Field
						id="backup-passphrase"
						label="Passphrase (optional)"
						hint="Encrypts the backup. It cannot be restored without it, and LogDeck does not keep it."
					>
						<Input
							id="backup-passphrase"
							type="password"
							autoComplete="new-password"
							value={passphrase}
							onChange={(e) => setPassphrase(e.target.value)}
						/>
					</Field>
				</div>
				<Note>
					Credentials are the sign-in settings, API tokens, platform API tokens
					and alert channel URLs. Without them, restoring keeps this instance's
					own. Stored logs and alert history can hold sensitive output; encrypt
					a backup that includes them before it leaves the host.
				</Note>
				<Button
					size="sm"
					disabled={busy}
					onClick={() =>
						createMutation.mutate(
							{
								includeLogs: includeLogs && status.logsAvailable,
								includeSecrets,
								passphrase,
							},
							{
								onSuccess: (message) => {
									toast.success(message);
									setPassphrase("");
								},
								onError: (error) => toast.error(error.message),
							},
						)
					}
				>
					{busy && <Spinner className="size-3" />}
					{status.running ? "Backing up…" : "Create backup"}
				</Button>
			</div>
		</SettingsSubsection>
	);
}

function ScheduleForm({ schedule }: { schedule: BackupSchedule }) {
	const [intervalHours, setIntervalHours] = useState(
		String(schedule.intervalHours),
	);
	const [keep, setKeep] = useState(String(schedule.keep));
	const [includeLogs, setIncludeLogs] = useState(schedule.includeLogs);
	const [includeSecrets, setIncludeSecrets] = useState(schedule.includeSecrets);
	const updateMutation = useUpdateBackupSchedule();
	const hasChanges =
		intervalHours !== String(schedule.intervalHours) ||
		keep !== String(schedule.keep) ||
		includeLogs !== schedule.includeLogs ||
		includeSecrets !== schedule.includeSecrets;

	function handleSave() {
		const error = validateSchedule(intervalHours, keep);
		if (error) {
			toast.error(error);
			return;
		}
		updateMutation.mutate(
			{
				intervalHours: Number(intervalHours),
				keep: Number(keep),
				includeLogs,
				includeSecrets,
			},
			showResultToast,
		);
	}

	return (
		<div className="space-y-4">
			<div className="grid max-w-md gap-4 sm:grid-cols-2">
				<Field id="backup-interval" label="Every (hours, 0 is off)">
					<Input
						id="backup-interval"
						type="number"
						min={0}
						value={intervalHours}
						onChange={(e) => setIntervalHours(e.target.value)}
					/>
				</Field>
				<Field id="backup-keep" label="Keep the latest">
					<Input
						id="backup-keep"
						type="number"
						min={1}
						value={keep}
						onChange={(e) => setKeep(e.target.value)}
					/>
				</Field>
			</div>
			<div className="flex flex-wrap gap-x-6 gap-y-3">
				<Toggle
					id="schedule-include-logs"
					label="Include stored logs"
					checked={includeLogs}
					onCheckedChange={setIncludeLogs}
				/>
				<Toggle
					id="schedule-include-secrets"
					label="Include credentials"
					checked={includeSecrets}
					onCheckedChange={setIncludeSecrets}
				/>
			</div>
			{hasChanges && (
				<SaveButton isPending={updateMutation.isPending} onClick={handleSave} />
			)}
		</div>
	);
}

function BackupRow({
	entry,
	readOnly,
	onRestore,
	onDelete,
}: {
	entry: BackupEntry;
	readOnly: boolean;
	onRestore: () => void;
	onDelete: () => void;
}) {
	return (
		<tr>
			<Td className="tabular-nums">
				<div className="flex items-center gap-2">
					{formatDate(entry.createdAt)}
					{entry.scheduled && (
						<Badge className="border-transparent bg-muted font-normal text-muted-foreground">
							Scheduled
						</Badge>
					)}
					{entry.encrypted && (
						<Badge className="border-transparent bg-muted font-normal text-muted-foreground">
							Encrypted
						</Badge>
					)}
				</div>
			</Td>
			<Td className="font-mono tabular-nums">{formatBytes(entry.sizeBytes)}</Td>
			<Td className="text-muted-foreground">
				{entry.manifest
					? `${describeContents(entry.manifest.contents)}${entry.manifest.includesSecrets ? ", with credentials" : ""}`
					: "Contents shown when restoring"}
			</Td>
			<Td className="text-right">
				<div className="flex justify-end gap-1">
					<Button variant="ghost" size="icon-sm" asChild>
						<a
							href={backupDownloadUrl(entry.name)}
							download={entry.name}
							aria-label={`Download ${entry.name}`}
						>
							<DownloadIcon className="size-4" />
						</a>
					</Button>
					{!readOnly && (
						<>
							<Button
								variant="ghost"
								size="icon-sm"
								onClick={onRestore}
								aria-label={`Restore ${entry.name}`}
							>
								<RotateCcwIcon className="size-4" />
							</Button>
							<Button
								variant="ghost"
								size="icon-sm"
								onClick={onDelete}
								className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
								aria-label={`Delete ${entry.name}`}
							>
								<Trash2Icon className="size-4" />
							</Button>
						</>
					)}
				</div>
			</Td>
		</tr>
	);
}

const CONTENT_ROWS: { key: keyof BackupContents; label: string }[] = [
	{ key: "dockerHosts", label: "Docker hosts" },
	{ key: "coolifyHosts", label: "Coolify connections" },
	{ key: "dokployHosts", label: "Dokploy connections" },
	{ key: "alertRules", label: "Alert rules" },
	{ key: "alertChannels", label: "Alert channels" },
	{ key: "alertHistory", label: "Past alerts" },
	{ key: "apiTokens", label: "API tokens" },
];

function PreviewTable({ preview }: { preview: RestorePreview }) {
	const { manifest, current } = preview;
	return (
		<SettingsTable>
			<THead>
				<Th>
					<span className="sr-only">Item</span>
				</Th>
				<Th>This instance</Th>
				<Th>After restore</Th>
			</THead>
			<TBody>
				{CONTENT_ROWS.map(({ key, label }) => {
					// Without credentials, the instance's own tokens are kept.
					const after =
						key === "apiTokens" && !manifest.includesSecrets
							? current.apiTokens
							: manifest.contents[key];
					return (
						<tr key={key}>
							<Td>{label}</Td>
							<Td className="tabular-nums">{current[key]}</Td>
							<Td className="tabular-nums">{after}</Td>
						</tr>
					);
				})}
				<tr>
					<Td>Stored logs</Td>
					<Td className="tabular-nums">{formatBytes(current.logsBytes)}</Td>
					<Td className="tabular-nums">
						{manifest.includesLogs
							? formatBytes(manifest.contents.logsBytes)
							: "Kept as is"}
					</Td>
				</tr>
			</TBody>
		</SettingsTable>
	);
}

function RestoreDialog({
	source,
	encrypted,
	onClose,
}: {
	source: RestoreSource;
	/** Known for stored backups; an uploaded file is found out on checking. */
	encrypted: boolean;
	onClose: () => void;
}) {
	const [passphrase, setPassphrase] = useState("");
	const checkMutation = useCheckRestore();
	const confirmMutation = useConfirmRestore();
	const preview = checkMutation.data;
	const label = "file" in source ? source.file.name : source.name;

	return (
		<AlertDialog
			open
			onOpenChange={(open) => {
				if (!open) onClose();
			}}
		>
			<AlertDialogContent className="sm:max-w-xl">
				<AlertDialogHeader>
					<AlertDialogTitle>Restore from backup?</AlertDialogTitle>
					<AlertDialogDescription>
						{preview
							? `Made ${formatDate(preview.manifest.createdAt)} by LogDeck ${preview.manifest.logdeckVersion}. It replaces the settings, alert rules, channels and history below.`
							: `LogDeck checks ${label} completely before anything changes.`}
					</AlertDialogDescription>
				</AlertDialogHeader>

				{!preview && (
					<div className="space-y-3">
						<Field
							id="restore-passphrase"
							label={encrypted ? "Passphrase" : "Passphrase (if encrypted)"}
						>
							<Input
								id="restore-passphrase"
								type="password"
								autoComplete="off"
								value={passphrase}
								onChange={(e) => setPassphrase(e.target.value)}
							/>
						</Field>
						{checkMutation.error && (
							<ErrorNote>{checkMutation.error.message}</ErrorNote>
						)}
					</div>
				)}

				{preview && (
					<div className="space-y-3">
						<PreviewTable preview={preview} />
						{preview.warnings.map((warning) => (
							<Note key={warning}>{warning}</Note>
						))}
						<Note>
							The restore is applied the next time LogDeck starts, so restart it
							once you confirm. The current data is moved to a pre-restore
							folder beside it first, and past alerts are not sent again.
						</Note>
					</div>
				)}

				<AlertDialogFooter>
					<AlertDialogCancel>Cancel</AlertDialogCancel>
					{preview ? (
						<Button
							disabled={confirmMutation.isPending}
							className="bg-destructive text-white hover:bg-destructive/90"
							onClick={() =>
								confirmMutation.mutate(undefined, {
									onSuccess: (message) => {
										toast.success(message);
										onClose();
									},
									onError: (error) => toast.error(error.message),
								})
							}
						>
							Restore on next start
						</Button>
					) : (
						<Button
							disabled={
								checkMutation.isPending || (encrypted && passphrase === "")
							}
							onClick={() => checkMutation.mutate({ source, passphrase })}
						>
							{checkMutation.isPending && <Spinner className="size-3" />}
							Check backup
						</Button>
					)}
				</AlertDialogFooter>
			</AlertDialogContent>
		</AlertDialog>
	);
}

export function BackupsSection({ readOnly }: { readOnly: boolean }) {
	const { data: status, isLoading, error } = useBackups();
	const deleteMutation = useDeleteBackup();
	const cancelMutation = useCancelRestore();
	const fileInputRef = useRef<HTMLInputElement>(null);
	const [restore, setRestore] = useState<{
		source: RestoreSource;
		encrypted: boolean;
	} | null>(null);
	const [deleteTarget, setDeleteTarget] = useState<string | null>(null);

	return (
		<SettingsSection
			title="Backups"
			description="A backup holds the saved settings, alert rules and channels, alert history and, if you choose, the stored logs. Settings from environment variables, the Docker socket, and keys or certificates mounted into the container are not included; keep those with your deployment files."
		>
			{isLoading && <Spinner className="size-4" />}
			{error && <ErrorNote>Failed to load backups: {error.message}</ErrorNote>}
			{status && (
				<div className="space-y-8">
					{status.pendingRestore && (
						<Well>
							<div className="flex flex-wrap items-center justify-between gap-3">
								<p className="max-w-prose text-base sm:text-sm">
									A restore of the backup made{" "}
									{formatDate(status.pendingRestore.manifest.createdAt)} is
									waiting. Restart LogDeck to apply it, for example with{" "}
									<code className="font-mono">docker restart logdeck</code>.
								</p>
								<Button
									variant="outline"
									size="sm"
									disabled={cancelMutation.isPending}
									onClick={() =>
										cancelMutation.mutate(undefined, showResultToast)
									}
								>
									Cancel restore
								</Button>
							</div>
						</Well>
					)}

					<CreateBackupForm status={status} />

					<SettingsSubsection
						title="Saved backups"
						action={
							!readOnly && (
								<>
									<input
										type="file"
										ref={fileInputRef}
										accept=".tar,.enc"
										className="hidden"
										onChange={(e) => {
											const file = e.target.files?.[0];
											e.target.value = "";
											if (file)
												setRestore({ source: { file }, encrypted: false });
										}}
									/>
									<Button
										variant="ghost"
										size="sm"
										onClick={() => fileInputRef.current?.click()}
									>
										<UploadIcon className="size-4" />
										Restore from file
									</Button>
								</>
							)
						}
					>
						<div className="space-y-4">
							<Note>
								Saved in <code className="font-mono">{status.dir}</code>. Set{" "}
								<code className="font-mono">BACKUP_DIR</code> to a mounted
								network share or synced folder to keep backups off this host.
							</Note>
							{status.last?.error && (
								<ErrorNote>
									The backup at {formatDate(status.last.at)} failed:{" "}
									{status.last.error}
								</ErrorNote>
							)}
							{status.listError && (
								<ErrorNote>
									The backup folder could not be read: {status.listError}
								</ErrorNote>
							)}
							{status.backups.length === 0 && !status.listError && (
								<Note>No backups yet.</Note>
							)}
							{status.backups.length > 0 && (
								<SettingsTable>
									<THead>
										<Th>Created</Th>
										<Th>Size</Th>
										<Th>Contents</Th>
										<Th className="text-right">
											<span className="sr-only">Actions</span>
										</Th>
									</THead>
									<TBody>
										{status.backups.map((entry) => (
											<BackupRow
												key={entry.name}
												entry={entry}
												readOnly={readOnly}
												onRestore={() =>
													setRestore({
														source: { name: entry.name },
														encrypted: entry.encrypted,
													})
												}
												onDelete={() => setDeleteTarget(entry.name)}
											/>
										))}
									</TBody>
								</SettingsTable>
							)}
						</div>
					</SettingsSubsection>

					<SettingsSubsection title="Schedule">
						<div className="space-y-4">
							<ScheduleForm
								key={JSON.stringify(status.schedule)}
								schedule={status.schedule}
							/>
							<Note>
								Only scheduled backups beyond the latest ones kept are deleted;
								backups made by hand stay until you delete them.{" "}
								{status.scheduleEncrypted
									? "Scheduled backups are encrypted with BACKUP_PASSPHRASE."
									: "Set BACKUP_PASSPHRASE to encrypt scheduled backups."}
								{status.nextRunAt &&
									` Next one around ${formatDate(status.nextRunAt)}.`}
							</Note>
						</div>
					</SettingsSubsection>
				</div>
			)}

			{restore && (
				<RestoreDialog
					source={restore.source}
					encrypted={restore.encrypted}
					onClose={() => setRestore(null)}
				/>
			)}

			<AlertDialog
				open={deleteTarget !== null}
				onOpenChange={(open) => {
					if (!open) setDeleteTarget(null);
				}}
			>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Delete this backup?</AlertDialogTitle>
						<AlertDialogDescription>
							{deleteTarget} is removed from the backup folder. Copies you
							downloaded are not affected.
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>Cancel</AlertDialogCancel>
						<Button
							disabled={deleteMutation.isPending}
							className="bg-destructive text-white hover:bg-destructive/90"
							onClick={() => {
								if (!deleteTarget) return;
								deleteMutation.mutate(deleteTarget, {
									...showResultToast,
									onSettled: () => setDeleteTarget(null),
								});
							}}
						>
							Delete backup
						</Button>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</SettingsSection>
	);
}
