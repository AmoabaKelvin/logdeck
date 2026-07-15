import { PencilIcon } from "lucide-react";
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
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Spinner } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";

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

const HISTORY_LIMIT = 50;

export function AlertsSection() {
	return (
		<Card>
			<CardHeader>
				<CardTitle>Alerts</CardTitle>
				<CardDescription>
					Get notified when containers die, run out of memory, become unhealthy,
					or log errors. Alerts are delivered to every enabled channel and
					recorded in the history below.
				</CardDescription>
			</CardHeader>
			<CardContent className="space-y-6">
				<ChannelsBlock />
				<Separator />
				<RulesBlock />
				<Separator />
				<HistoryBlock />
			</CardContent>
		</Card>
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
				<div className="space-y-1.5">
					<Label htmlFor="channel-url">{urlLabel}</Label>
					<Input
						id="channel-url"
						value={draft.url}
						onChange={(e) => set("url", e.target.value)}
						placeholder={urlPlaceholder}
						className="h-8"
					/>
				</div>
			)}
			{needsToken && (
				<div className="space-y-1.5">
					<Label htmlFor="channel-token">{tokenLabel}</Label>
					<Input
						id="channel-token"
						type="password"
						autoComplete="new-password"
						value={draft.token}
						onChange={(e) => set("token", e.target.value)}
						placeholder={tokenLabel}
						className="h-8"
					/>
				</div>
			)}
			{needsTarget && (
				<div className="space-y-1.5">
					<Label htmlFor="channel-target">Chat id</Label>
					<Input
						id="channel-target"
						value={draft.target}
						onChange={(e) => set("target", e.target.value)}
						placeholder="-1001234567890"
						className="h-8"
					/>
				</div>
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
		<div className="space-y-3 border rounded-md p-3">
			<div className="flex flex-wrap items-end gap-3">
				<div className="space-y-1.5">
					<Label htmlFor="channel-type">Type</Label>
					<Select
						value={draft.type}
						onValueChange={(value) =>
							setDraft({
								...EMPTY_CHANNEL_DRAFT,
								type: value as ChannelDraft["type"],
								name: draft.name,
							})
						}
					>
						<SelectTrigger id="channel-type" className="h-8 w-32">
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
				</div>
				<div className="space-y-1.5 flex-1 min-w-40">
					<Label htmlFor="channel-name">
						Name{" "}
						<span className="font-normal text-muted-foreground">
							(optional)
						</span>
					</Label>
					<Input
						id="channel-name"
						value={draft.name}
						onChange={(e) => set("name", e.target.value)}
						placeholder="Team Slack"
						className="h-8"
						maxLength={64}
					/>
				</div>
			</div>
			<ChannelDraftFields draft={draft} set={set} />
			<div className="flex gap-1">
				<Button
					size="sm"
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
		</div>
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
		<div className="space-y-3">
			<h3 className="text-sm font-medium">Channels</h3>

			{isLoading && <Spinner className="size-4" />}
			{error && (
				<p className="text-sm text-destructive">
					Failed to load channels: {error.message}
				</p>
			)}

			{!isLoading && !error && channels.length === 0 && !isAdding && (
				<p className="text-sm text-muted-foreground">
					No channels configured — alerts are recorded in history only.
				</p>
			)}

			{channels.length > 0 && (
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead>Type</TableHead>
							<TableHead>Destination</TableHead>
							<TableHead>Enabled</TableHead>
							<TableHead className="text-right">Actions</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{channels.map((channel) => {
							const testing =
								testMutation.isPending && testMutation.variables === channel.id;
							return (
								<TableRow key={channel.id}>
									<TableCell className="font-medium">
										{channel.name || channelTypeLabel(channel.type)}
										{channel.name && (
											<span className="ml-1.5 text-xs text-muted-foreground">
												{channelTypeLabel(channel.type)}
											</span>
										)}
									</TableCell>
									<TableCell className="text-xs text-muted-foreground font-mono break-all">
										{channelDestination(channel)}
									</TableCell>
									<TableCell>
										<Switch
											checked={channel.enabled}
											onCheckedChange={(checked) =>
												handleToggle(channel, checked)
											}
											disabled={updateMutation.isPending}
											aria-label={`Toggle ${channelTypeLabel(channel.type)} channel`}
										/>
									</TableCell>
									<TableCell className="text-right">
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
									</TableCell>
								</TableRow>
							);
						})}
					</TableBody>
				</Table>
			)}

			{isAdding ? (
				<AddChannelForm onDone={() => setIsAdding(false)} />
			) : (
				<Button variant="outline" size="sm" onClick={() => setIsAdding(true)}>
					Add channel
				</Button>
			)}

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
		<div className="space-y-3">
			<h3 className="text-sm font-medium">Rules</h3>

			{isLoading && <Spinner className="size-4" />}
			{error && (
				<p className="text-sm text-destructive">
					Failed to load alert rules: {error.message}
				</p>
			)}

			{!isLoading && !error && rules.length === 0 && (
				<p className="text-sm text-muted-foreground">
					No alert rules created yet.
				</p>
			)}

			{rules.length > 0 && (
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead>Name</TableHead>
							<TableHead>Type</TableHead>
							<TableHead>Target</TableHead>
							<TableHead>Trigger</TableHead>
							<TableHead>Enabled</TableHead>
							<TableHead className="text-right">Actions</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{rules.map((rule) => (
							<TableRow key={rule.id}>
								<TableCell className="font-medium">{rule.name}</TableCell>
								<TableCell className="text-xs text-muted-foreground">
									{rule.type}
								</TableCell>
								<TableCell className="text-xs text-muted-foreground">
									{renderTarget(rule)}
								</TableCell>
								<TableCell className="text-xs text-muted-foreground font-mono">
									{renderTrigger(rule)}
								</TableCell>
								<TableCell>
									<Switch
										checked={rule.enabled}
										onCheckedChange={(checked) => handleToggle(rule, checked)}
										disabled={updateMutation.isPending}
										aria-label={`Toggle rule ${rule.name}`}
									/>
								</TableCell>
								<TableCell className="text-right">
									<div className="flex items-center justify-end gap-1">
										<Button
											variant="ghost"
											size="sm"
											onClick={() => openEdit(rule)}
											aria-label={`Edit rule ${rule.name}`}
										>
											<PencilIcon className="size-3.5" />
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
								</TableCell>
							</TableRow>
						))}
					</TableBody>
				</Table>
			)}

			<Button variant="outline" size="sm" onClick={openCreate}>
				Create rule
			</Button>

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
	);
}

function DeliveryStatus({ entry }: { entry: AlertHistoryEntry }) {
	if (!entry.delivery) {
		return <span className="text-xs text-muted-foreground">—</span>;
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
		<span
			title={detail}
			className={`inline-flex items-center gap-1.5 text-xs ${
				ok
					? "text-green-600 dark:text-green-400"
					: "text-red-600 dark:text-red-400"
			}`}
		>
			<span
				className={`size-1.5 rounded-full ${ok ? "bg-green-500" : "bg-red-500"}`}
			/>
			{entry.delivery.status}
		</span>
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
		<div className="space-y-3">
			<div className="flex items-center justify-between">
				<h3 className="text-sm font-medium">History</h3>
				{alerts.length > 0 && (
					<Button
						variant="ghost"
						size="sm"
						disabled={clearMutation.isPending}
						onClick={() => setIsClearOpen(true)}
					>
						Clear
					</Button>
				)}
			</div>

			{isLoading && <Spinner className="size-4" />}
			{error && (
				<p className="text-sm text-destructive">
					Failed to load alert history: {error.message}
				</p>
			)}

			{!isLoading && !error && alerts.length === 0 && (
				<p className="text-sm text-muted-foreground">No alerts fired yet.</p>
			)}

			{alerts.length > 0 && (
				<div className="max-h-80 overflow-y-auto rounded-md border">
					<Table>
						<TableHeader>
							<TableRow>
								<TableHead>Time</TableHead>
								<TableHead>Rule</TableHead>
								<TableHead>Container</TableHead>
								<TableHead>Reason</TableHead>
								<TableHead>Delivery</TableHead>
							</TableRow>
						</TableHeader>
						<TableBody>
							{alerts.map((entry) => (
								<TableRow key={entry.id}>
									<TableCell className="text-xs text-muted-foreground whitespace-nowrap">
										{new Date(entry.firedAt).toLocaleString()}
									</TableCell>
									<TableCell className="font-medium">
										{entry.ruleName}
									</TableCell>
									<TableCell className="text-xs text-muted-foreground font-mono">
										{entry.containerName}@{entry.host}
									</TableCell>
									<TableCell className="text-xs text-muted-foreground">
										{entry.reason}
										{entry.suppressed > 0 && (
											<span className="ml-1.5 text-muted-foreground/70">
												(+{entry.suppressed} suppressed)
											</span>
										)}
									</TableCell>
									<TableCell>
										<DeliveryStatus entry={entry} />
									</TableCell>
								</TableRow>
							))}
						</TableBody>
					</Table>
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
	);
}
