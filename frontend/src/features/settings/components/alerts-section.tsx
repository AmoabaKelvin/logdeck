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

import type { AlertHistoryEntry } from "../api/get-alert-history";
import type { AlertRule } from "../api/get-alert-rules";
import {
	useAlertHistory,
	useAlertRules,
	useAlertWebhook,
	useClearAlertHistory,
	useDeleteAlertRule,
	useTestAlertWebhook,
	useUpdateAlertRule,
	useUpdateAlertWebhook,
} from "../hooks/use-alerts";
import { AlertRuleDialog } from "./alert-rule-dialog";
import { showResultToast } from "./mutation-toast";

const HISTORY_LIMIT = 50;

export function AlertsSection() {
	return (
		<Card>
			<CardHeader>
				<CardTitle>Alerts</CardTitle>
				<CardDescription>
					Get notified when containers die, run out of memory, or log errors.
					Alerts are delivered to a webhook and recorded in the history below.
				</CardDescription>
			</CardHeader>
			<CardContent className="space-y-6">
				<WebhookBlock />
				<Separator />
				<RulesBlock />
				<Separator />
				<HistoryBlock />
			</CardContent>
		</Card>
	);
}

function WebhookBlock() {
	const { data, isLoading, error } = useAlertWebhook();
	const updateMutation = useUpdateAlertWebhook();
	const testMutation = useTestAlertWebhook();

	const [draft, setDraft] = useState<string | null>(null);

	const savedUrl = data?.url ?? "";
	const url = draft ?? savedUrl;

	function handleSave() {
		updateMutation.mutate(url.trim(), {
			onSuccess: (message) => {
				toast.success(message);
				setDraft(null);
			},
			onError: (err) => toast.error(err.message),
		});
	}

	function handleTest() {
		testMutation.mutate(undefined, {
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

	return (
		<div className="space-y-3">
			<h3 className="text-sm font-medium">Webhook</h3>
			{isLoading && <Spinner className="size-4" />}
			{error && (
				<p className="text-sm text-destructive">
					Failed to load webhook: {error.message}
				</p>
			)}
			{!isLoading && !error && (
				<>
					<div className="flex items-end gap-2">
						<div className="space-y-1.5 flex-1">
							<Label htmlFor="alert-webhook-url">URL</Label>
							<Input
								id="alert-webhook-url"
								value={url}
								onChange={(e) => setDraft(e.target.value)}
								placeholder="https://example.com/webhook"
								className="h-8"
							/>
						</div>
						<Button
							size="sm"
							disabled={updateMutation.isPending || draft === null}
							onClick={handleSave}
						>
							{updateMutation.isPending ? (
								<>
									<Spinner className="size-3" />
									Saving...
								</>
							) : (
								"Save"
							)}
						</Button>
						<Button
							variant="outline"
							size="sm"
							disabled={testMutation.isPending}
							onClick={handleTest}
						>
							{testMutation.isPending ? (
								<>
									<Spinner className="size-3" />
									Sending...
								</>
							) : (
								"Send test"
							)}
						</Button>
					</div>
					{savedUrl === "" && (
						<p className="text-xs text-muted-foreground">
							(no webhook configured — alerts are recorded in history only)
						</p>
					)}
				</>
			)}
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
