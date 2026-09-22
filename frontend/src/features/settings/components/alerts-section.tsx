import { useState } from "react";
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
import { PencilIcon } from "@/components/ui/icons";
import { Input } from "@/components/ui/input";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";

import type { AlertChannel } from "../api/get-alert-channels";
import type { AlertHistoryEntry } from "../api/get-alert-history";
import type { AlertRule } from "../api/get-alert-rules";
import {
	useAlertChannels,
	useAlertHistory,
	useAlertRules,
	useClearAlertHistory,
	useCreateAlertChannel,
	useDeleteAlertChannel,
	useDeleteAlertRule,
	useTestAlertChannel,
	useUpdateAlertChannel,
	useUpdateAlertRule,
} from "../hooks/use-alerts";
import { AlertRuleDialog } from "./alert-rule-dialog";
import {
	buildChannelPayload,
	CHANNEL_TYPES,
	type ChannelDraft,
	channelDestination,
	channelTypeLabel,
	EMPTY_CHANNEL_DRAFT,
} from "./channel-utils";
import { showResultToast } from "./mutation-toast";
import {
	ErrorNote,
	Field,
	Note,
	Outcome,
	SettingsSection,
	SettingsSubsection,
	SettingsTable,
	TBody,
	Td,
	Th,
	THead,
	Well,
} from "./settings-ui";

const HISTORY_LIMIT = 50;

export function AlertsSection() {
	return (
		<SettingsSection
			title="Alerts"
			description="Get told when a container dies, runs out of memory, turns unhealthy or logs errors. Every enabled channel receives each alert, and the history keeps a record."
		>
			<div className="space-y-8">
				<ChannelsBlock />
				<RulesBlock />
				<HistoryBlock />
			</div>
		</SettingsSection>
	);
}

function ChannelDraftFields({
	draft,
	set,
}: {
	draft: ChannelDraft;
	set: <K extends keyof ChannelDraft>(key: K, value: ChannelDraft[K]) => void;
}) {
	const needsURL = draft.type !== "telegram";
	const needsToken = draft.type === "gotify" || draft.type === "telegram";
	const needsTarget = draft.type === "telegram";

	const urlLabel = draft.type === "gotify" ? "Server URL" : "URL";
	const urlPlaceholder =
		draft.type === "ntfy"
			? "https://ntfy.sh/mytopic"
			: draft.type === "gotify"
				? "https://gotify.example.com"
				: "https://example.com/webhook";
	const tokenLabel = draft.type === "gotify" ? "App token" : "Bot token";

	return (
		<>
			{needsURL && (
				<Field id="channel-url" label={urlLabel}>
					<Input
						id="channel-url"
						name="url"
						value={draft.url}
						onChange={(e) => set("url", e.target.value)}
						placeholder={urlPlaceholder}
						className="h-8"
					/>
				</Field>
			)}
			{needsToken && (
				<Field id="channel-token" label={tokenLabel}>
					<Input
						id="channel-token"
						name="token"
						type="password"
						autoComplete="new-password"
						value={draft.token}
						onChange={(e) => set("token", e.target.value)}
						placeholder={tokenLabel}
						className="h-8"
					/>
				</Field>
			)}
			{needsTarget && (
				<Field id="channel-target" label="Chat id">
					<Input
						id="channel-target"
						name="target"
						value={draft.target}
						onChange={(e) => set("target", e.target.value)}
						placeholder="-1001234567890"
						className="h-8"
					/>
				</Field>
			)}
		</>
	);
}

function AddChannelForm({ onDone }: { onDone: () => void }) {
	const createMutation = useCreateAlertChannel();
	const [draft, setDraft] = useState<ChannelDraft>(EMPTY_CHANNEL_DRAFT);

	function set<K extends keyof ChannelDraft>(key: K, value: ChannelDraft[K]) {
		setDraft((prev) => ({ ...prev, [key]: value }));
	}

	function handleAdd() {
		const result = buildChannelPayload(draft);
		if ("error" in result) {
			toast.error(result.error);
			return;
		}
		createMutation.mutate(result.payload, {
			onSuccess: (created) => {
				toast.success(`${channelTypeLabel(created.type)} channel added`);
				onDone();
			},
			onError: (err) => toast.error(err.message),
		});
	}

	return (
		<Well>
			<div className="grid gap-3 sm:grid-cols-[auto_1fr]">
				<Field id="channel-type" label="Type">
					<Select
						value={draft.type}
						onValueChange={(value) => {
							const type = CHANNEL_TYPES.find((t) => t === value);
							if (type)
								setDraft({ ...EMPTY_CHANNEL_DRAFT, type, name: draft.name });
						}}
					>
						<SelectTrigger id="channel-type" className="h-8 w-full sm:w-32">
							<SelectValue />
						</SelectTrigger>
						<SelectContent>
							{CHANNEL_TYPES.map((type) => (
								<SelectItem key={type} value={type}>
									{channelTypeLabel(type)}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				</Field>
				<Field
					id="channel-name"
					label={
						<>
							Name{" "}
							<span className="font-normal text-muted-foreground">
								(optional)
							</span>
						</>
					}
				>
					<Input
						id="channel-name"
						name="name"
						value={draft.name}
						onChange={(e) => set("name", e.target.value)}
						placeholder="Team Slack"
						className="h-8"
						maxLength={64}
					/>
				</Field>
			</div>
			<div className="mt-3 grid gap-3">
				<ChannelDraftFields draft={draft} set={set} />
			</div>
			<div className="mt-3 flex gap-1">
				<Button
					size="sm"
					variant="outline"
					disabled={createMutation.isPending}
					onClick={handleAdd}
				>
					{createMutation.isPending ? (
						<>
							<Spinner className="size-3" />
							Adding...
						</>
					) : (
						"Add"
					)}
				</Button>
				<Button size="sm" variant="ghost" onClick={onDone}>
					Cancel
				</Button>
			</div>
		</Well>
	);
}

function ChannelsBlock() {
	const { data, isLoading, error } = useAlertChannels();
	const updateMutation = useUpdateAlertChannel();
	const deleteMutation = useDeleteAlertChannel();
	const testMutation = useTestAlertChannel();

	const [isAdding, setIsAdding] = useState(false);
	const [channelToDelete, setChannelToDelete] = useState<AlertChannel | null>(
		null,
	);

	const channels = data?.channels ?? [];

	function handleToggle(channel: AlertChannel, enabled: boolean) {
		const { id, ...rest } = channel;
		updateMutation.mutate(
			{ id, channel: { ...rest, enabled } },
			{
				onSuccess: (updated) => {
					toast.success(
						`${channelTypeLabel(updated.type)} channel ${
							updated.enabled ? "enabled" : "disabled"
						}`,
					);
				},
				onError: (err) => toast.error(err.message),
			},
		);
	}

	function handleTest(channel: AlertChannel) {
		testMutation.mutate(channel.id, {
			onSuccess: (result) => {
				if (result.status === "ok") {
					toast.success("Test alert delivered");
				} else {
					const status = result.httpStatus
						? ` (HTTP ${result.httpStatus})`
						: "";
					const detail = result.error ? `: ${result.error}` : "";
					toast.error(`Test alert failed${status}${detail}`);
				}
			},
			onError: (err) => toast.error(err.message),
		});
	}

	function handleDelete() {
		if (!channelToDelete) return;
		deleteMutation.mutate(channelToDelete.id, showResultToast);
		setChannelToDelete(null);
	}

	return (
		<SettingsSubsection
			title="Channels"
			action={
				!isAdding && (
					<Button variant="outline" size="sm" onClick={() => setIsAdding(true)}>
						Add channel
					</Button>
				)
			}
		>
			<div className="space-y-3">
				{isLoading && <Spinner className="size-4" />}
				{error && (
					<ErrorNote>Failed to load channels: {error.message}</ErrorNote>
				)}

				{!isLoading && !error && channels.length === 0 && !isAdding && (
					<Note>
						No channels yet, so alerts are only recorded in the history.
					</Note>
				)}

				{channels.length > 0 && (
					<SettingsTable>
						<THead>
							<Th>Channel</Th>
							<Th>Destination</Th>
							<Th>Enabled</Th>
							<Th className="text-right">
								<span className="sr-only">Actions</span>
							</Th>
						</THead>
						<TBody>
							{channels.map((channel) => {
								const testing =
									testMutation.isPending &&
									testMutation.variables === channel.id;
								return (
									<tr key={channel.id}>
										<Td className="font-medium">
											<div className="flex items-baseline gap-1.5">
												{channel.name || channelTypeLabel(channel.type)}
												{channel.name && (
													<span className="font-normal text-muted-foreground">
														{channelTypeLabel(channel.type)}
													</span>
												)}
											</div>
										</Td>
										<Td className="font-mono text-muted-foreground">
											{channelDestination(channel)}
										</Td>
										<Td>
											<Switch
												checked={channel.enabled}
												onCheckedChange={(checked) =>
													handleToggle(channel, checked)
												}
												disabled={updateMutation.isPending}
												aria-label={`Toggle ${channelTypeLabel(channel.type)} channel`}
											/>
										</Td>
										<Td className="text-right">
											<div className="flex items-center justify-end gap-1">
												<Button
													variant="ghost"
													size="sm"
													disabled={testing}
													onClick={() => handleTest(channel)}
												>
													{testing ? (
														<>
															<Spinner className="size-3" />
															Sending...
														</>
													) : (
														"Test"
													)}
												</Button>
												<Button
													variant="ghost"
													size="sm"
													disabled={deleteMutation.isPending}
													onClick={() => setChannelToDelete(channel)}
													className="text-destructive hover:text-destructive"
												>
													Delete
												</Button>
											</div>
										</Td>
									</tr>
								);
							})}
						</TBody>
					</SettingsTable>
				)}

				{isAdding && <AddChannelForm onDone={() => setIsAdding(false)} />}

				<AlertDialog
					open={channelToDelete !== null}
					onOpenChange={(open) => {
						if (!open) setChannelToDelete(null);
					}}
				>
					<AlertDialogContent>
						<AlertDialogHeader>
							<AlertDialogTitle>Delete channel?</AlertDialogTitle>
							<AlertDialogDescription>
								Alerts will no longer be delivered to this{" "}
								{channelToDelete ? channelTypeLabel(channelToDelete.type) : ""}{" "}
								channel. This cannot be undone.
							</AlertDialogDescription>
						</AlertDialogHeader>
						<AlertDialogFooter>
							<AlertDialogCancel>Cancel</AlertDialogCancel>
							<AlertDialogAction
								onClick={handleDelete}
								className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
							>
								Delete
							</AlertDialogAction>
						</AlertDialogFooter>
					</AlertDialogContent>
				</AlertDialog>
			</div>
		</SettingsSubsection>
	);
}

function renderTarget(rule: AlertRule): string {
	const parts: string[] = [];
	if (rule.hosts?.length) parts.push(`hosts: ${rule.hosts.join(", ")}`);
	if (rule.containers?.length) {
		parts.push(`containers: ${rule.containers.join(", ")}`);
	}
	if (rule.projects?.length) {
		parts.push(`projects: ${rule.projects.join(", ")}`);
	}
	return parts.length > 0 ? parts.join(" · ") : "all containers";
}

function renderTrigger(rule: AlertRule): string {
	const parts: string[] = [];
	if (rule.type === "log") {
		if (rule.minLevel) parts.push(`level >= ${rule.minLevel}`);
		if (rule.pattern) parts.push(`pattern /${rule.pattern}/`);
	} else {
		parts.push((rule.events ?? []).join(", "));
	}
	if (rule.threshold > 1) {
		parts.push(`${rule.threshold} in ${rule.windowSeconds ?? 0}s`);
	}
	if (rule.cooldownSeconds) {
		parts.push(`cooldown ${rule.cooldownSeconds}s`);
	} else {
		parts.push("cooldown 300s (default)");
	}
	return parts.filter(Boolean).join(" · ");
}

function RulesBlock() {
	const { data, isLoading, error } = useAlertRules();
	const updateMutation = useUpdateAlertRule();
	const deleteMutation = useDeleteAlertRule();

	const [isDialogOpen, setIsDialogOpen] = useState(false);
	const [editingRule, setEditingRule] = useState<AlertRule | null>(null);
	const [ruleToDelete, setRuleToDelete] = useState<AlertRule | null>(null);

	const rules = data?.rules ?? [];

	function openCreate() {
		setEditingRule(null);
		setIsDialogOpen(true);
	}

	function openEdit(rule: AlertRule) {
		setEditingRule(rule);
		setIsDialogOpen(true);
	}

	function handleToggle(rule: AlertRule, enabled: boolean) {
		const { id: _id, createdAt: _createdAt, ...payload } = rule;
		updateMutation.mutate(
			{ id: rule.id, rule: { ...payload, enabled } },
			{
				onSuccess: (updated) => {
					toast.success(
						`Rule "${updated.name}" ${updated.enabled ? "enabled" : "disabled"}`,
					);
				},
				onError: (err) => toast.error(err.message),
			},
		);
	}

	function handleDelete() {
		if (!ruleToDelete) return;
		deleteMutation.mutate(ruleToDelete.id, showResultToast);
		setRuleToDelete(null);
	}

	return (
		<SettingsSubsection
			title="Rules"
			action={
				<Button variant="outline" size="sm" onClick={openCreate}>
					Create rule
				</Button>
			}
		>
			<div className="space-y-3">
				{isLoading && <Spinner className="size-4" />}
				{error && (
					<ErrorNote>Failed to load alert rules: {error.message}</ErrorNote>
				)}

				{!isLoading && !error && rules.length === 0 && (
					<Note>No alert rules yet.</Note>
				)}

				{rules.length > 0 && (
					<SettingsTable>
						<THead>
							<Th>Name</Th>
							<Th>Type</Th>
							<Th>Target</Th>
							<Th>Trigger</Th>
							<Th>Enabled</Th>
							<Th className="text-right">
								<span className="sr-only">Actions</span>
							</Th>
						</THead>
						<TBody>
							{rules.map((rule) => (
								<tr key={rule.id}>
									<Td className="font-medium">{rule.name}</Td>
									<Td className="text-muted-foreground">{rule.type}</Td>
									<Td className="text-muted-foreground">
										{renderTarget(rule)}
									</Td>
									<Td className="font-mono text-muted-foreground">
										{renderTrigger(rule)}
									</Td>
									<Td>
										<Switch
											checked={rule.enabled}
											onCheckedChange={(checked) => handleToggle(rule, checked)}
											disabled={updateMutation.isPending}
											aria-label={`Toggle rule ${rule.name}`}
										/>
									</Td>
									<Td className="text-right">
										<div className="flex items-center justify-end gap-1">
											<Button
												variant="ghost"
												size="sm"
												onClick={() => openEdit(rule)}
												aria-label={`Edit rule ${rule.name}`}
											>
												<PencilIcon className="size-4" />
											</Button>
											<Button
												variant="ghost"
												size="sm"
												disabled={deleteMutation.isPending}
												onClick={() => setRuleToDelete(rule)}
												className="text-destructive hover:text-destructive"
											>
												Delete
											</Button>
										</div>
									</Td>
								</tr>
							))}
						</TBody>
					</SettingsTable>
				)}

				<AlertRuleDialog
					open={isDialogOpen}
					onOpenChange={setIsDialogOpen}
					rule={editingRule}
				/>

				<AlertDialog
					open={ruleToDelete !== null}
					onOpenChange={(open) => {
						if (!open) setRuleToDelete(null);
					}}
				>
					<AlertDialogContent>
						<AlertDialogHeader>
							<AlertDialogTitle>Delete alert rule?</AlertDialogTitle>
							<AlertDialogDescription>
								The rule "{ruleToDelete?.name}" will stop firing alerts
								immediately. This cannot be undone.
							</AlertDialogDescription>
						</AlertDialogHeader>
						<AlertDialogFooter>
							<AlertDialogCancel>Cancel</AlertDialogCancel>
							<AlertDialogAction
								onClick={handleDelete}
								className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
							>
								Delete
							</AlertDialogAction>
						</AlertDialogFooter>
					</AlertDialogContent>
				</AlertDialog>
			</div>
		</SettingsSubsection>
	);
}

function DeliveryStatus({ entry }: { entry: AlertHistoryEntry }) {
	if (!entry.delivery) {
		return <span className="text-muted-foreground">—</span>;
	}
	const ok = entry.delivery.status === "ok";
	const detail = ok
		? undefined
		: [
				entry.delivery.httpStatus ? `HTTP ${entry.delivery.httpStatus}` : null,
				entry.delivery.error,
			]
				.filter(Boolean)
				.join(": ");
	return (
		<Outcome ok={ok} title={detail}>
			{entry.delivery.status}
		</Outcome>
	);
}

function HistoryBlock() {
	const { data, isLoading, error } = useAlertHistory(HISTORY_LIMIT);
	const clearMutation = useClearAlertHistory();
	const [isClearOpen, setIsClearOpen] = useState(false);

	const alerts = data?.alerts ?? [];

	function handleClear() {
		clearMutation.mutate(undefined, showResultToast);
		setIsClearOpen(false);
	}

	return (
		<SettingsSubsection
			title="History"
			action={
				alerts.length > 0 && (
					<Button
						variant="ghost"
						size="sm"
						disabled={clearMutation.isPending}
						onClick={() => setIsClearOpen(true)}
					>
						Clear
					</Button>
				)
			}
		>
			<div className="space-y-3">
				{isLoading && <Spinner className="size-4" />}
				{error && (
					<ErrorNote>Failed to load alert history: {error.message}</ErrorNote>
				)}

				{!isLoading && !error && alerts.length === 0 && (
					<Note>No alerts fired yet.</Note>
				)}

				{alerts.length > 0 && (
					<div className="max-h-96 overflow-y-auto">
						<SettingsTable>
							<THead>
								<Th>Time</Th>
								<Th>Rule</Th>
								<Th>Container</Th>
								<Th>Reason</Th>
								<Th>Delivery</Th>
							</THead>
							<TBody>
								{alerts.map((entry) => (
									<tr key={entry.id}>
										<Td className="text-muted-foreground tabular-nums">
											{new Date(entry.firedAt).toLocaleString()}
										</Td>
										<Td className="font-medium">{entry.ruleName}</Td>
										<Td className="font-mono text-muted-foreground">
											{entry.containerName}@{entry.host}
										</Td>
										<Td className="text-muted-foreground">
											{entry.reason}
											{entry.suppressed > 0 && (
												<span className="text-muted-foreground/70">
													{` (+${entry.suppressed} suppressed)`}
												</span>
											)}
										</Td>
										<Td>
											<DeliveryStatus entry={entry} />
										</Td>
									</tr>
								))}
							</TBody>
						</SettingsTable>
					</div>
				)}

				<AlertDialog open={isClearOpen} onOpenChange={setIsClearOpen}>
					<AlertDialogContent>
						<AlertDialogHeader>
							<AlertDialogTitle>Clear alert history?</AlertDialogTitle>
							<AlertDialogDescription>
								All recorded alerts will be removed. This cannot be undone.
							</AlertDialogDescription>
						</AlertDialogHeader>
						<AlertDialogFooter>
							<AlertDialogCancel>Cancel</AlertDialogCancel>
							<AlertDialogAction
								onClick={handleClear}
								className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
							>
								Clear
							</AlertDialogAction>
						</AlertDialogFooter>
					</AlertDialogContent>
				</AlertDialog>
			</div>
		</SettingsSubsection>
	);
}
