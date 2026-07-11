import { CheckIcon } from "lucide-react";
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

import type { AlertRulePayload } from "../api/create-alert-rule";
import type { AlertHistoryEntry } from "../api/get-alert-history";
import type { AlertEventKind, AlertRule } from "../api/get-alert-rules";
import {
	useAlertHistory,
	useAlertRules,
	useAlertWebhook,
	useClearAlertHistory,
	useCreateAlertRule,
	useDeleteAlertRule,
	useTestAlertWebhook,
	useUpdateAlertRule,
	useUpdateAlertWebhook,
} from "../hooks/use-alerts";
import { showResultToast } from "./mutation-toast";

const LOG_LEVELS = [
	"TRACE",
	"DEBUG",
	"INFO",
	"WARN",
	"ERROR",
	"FATAL",
	"PANIC",
];

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

interface RuleFormState {
	name: string;
	type: "log" | "event";
	minLevel: string;
	pattern: string;
	die: boolean;
	oom: boolean;
	threshold: string;
	windowSeconds: string;
	containers: string;
	projects: string;
	hosts: string;
	cooldownSeconds: string;
}

const EMPTY_FORM: RuleFormState = {
	name: "",
	type: "log",
	minLevel: "any",
	pattern: "",
	die: false,
	oom: false,
	threshold: "1",
	windowSeconds: "",
	containers: "",
	projects: "",
	hosts: "",
	cooldownSeconds: "",
};

function splitList(value: string): string[] {
	return value
		.split(",")
		.map((s) => s.trim())
		.filter(Boolean);
}

function ruleToForm(rule: AlertRule): RuleFormState {
	return {
		name: rule.name,
		type: rule.type,
		minLevel: rule.minLevel ?? "any",
		pattern: rule.pattern ?? "",
		die: rule.events?.includes("die") ?? false,
		oom: rule.events?.includes("oom") ?? false,
		threshold: String(rule.threshold),
		windowSeconds: rule.windowSeconds ? String(rule.windowSeconds) : "",
		containers: rule.containers?.join(", ") ?? "",
		projects: rule.projects?.join(", ") ?? "",
		hosts: rule.hosts?.join(", ") ?? "",
		cooldownSeconds: rule.cooldownSeconds ? String(rule.cooldownSeconds) : "",
	};
}

function buildPayload(
	form: RuleFormState,
): { error: string } | { payload: AlertRulePayload } {
	const name = form.name.trim();
	if (!name) return { error: "Rule name is required" };

	const payload: AlertRulePayload = {
		name,
		enabled: true,
		type: form.type,
		threshold: 1,
	};

	if (form.type === "log") {
		const pattern = form.pattern.trim();
		if (form.minLevel === "any" && !pattern) {
			return { error: "Log rules need a minimum level or a pattern" };
		}
		if (form.minLevel !== "any") payload.minLevel = form.minLevel;
		if (pattern) payload.pattern = pattern;
	} else {
		const events: AlertEventKind[] = [];
		if (form.die) events.push("die");
		if (form.oom) events.push("oom");
		if (events.length === 0) {
			return { error: "Event rules need at least one event" };
		}
		payload.events = events;
	}

	const threshold = Number.parseInt(form.threshold, 10);
	if (!Number.isNaN(threshold) && threshold > 0) payload.threshold = threshold;

	const windowSeconds = Number.parseInt(form.windowSeconds, 10);
	if (!Number.isNaN(windowSeconds) && windowSeconds > 0) {
		payload.windowSeconds = windowSeconds;
	}

	const cooldownSeconds = Number.parseInt(form.cooldownSeconds, 10);
	if (!Number.isNaN(cooldownSeconds) && cooldownSeconds > 0) {
		payload.cooldownSeconds = cooldownSeconds;
	}

	const containers = splitList(form.containers);
	if (containers.length > 0) payload.containers = containers;
	const projects = splitList(form.projects);
	if (projects.length > 0) payload.projects = projects;
	const hosts = splitList(form.hosts);
	if (hosts.length > 0) payload.hosts = hosts;

	return { payload };
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

function EventCheckbox({
	label,
	checked,
	onToggle,
}: {
	label: string;
	checked: boolean;
	onToggle: () => void;
}) {
	return (
		<label className="flex items-center gap-2 cursor-pointer text-sm">
			<button
				type="button"
				onClick={onToggle}
				aria-pressed={checked}
				className={`size-4 rounded border flex items-center justify-center ${
					checked ? "bg-primary border-primary" : "border-input"
				}`}
			>
				{checked && <CheckIcon className="size-3 text-primary-foreground" />}
			</button>
			{label}
		</label>
	);
}

function RulesBlock() {
	const { data, isLoading, error } = useAlertRules();
	const createMutation = useCreateAlertRule();
	const updateMutation = useUpdateAlertRule();
	const deleteMutation = useDeleteAlertRule();

	const [isFormOpen, setIsFormOpen] = useState(false);
	const [editingRule, setEditingRule] = useState<AlertRule | null>(null);
	const [form, setForm] = useState<RuleFormState>(EMPTY_FORM);
	const [ruleToDelete, setRuleToDelete] = useState<AlertRule | null>(null);

	const rules = data?.rules ?? [];

	function set<K extends keyof RuleFormState>(key: K, value: RuleFormState[K]) {
		setForm((prev) => ({ ...prev, [key]: value }));
	}

	function openCreate() {
		setEditingRule(null);
		setForm(EMPTY_FORM);
		setIsFormOpen(true);
	}

	function openEdit(rule: AlertRule) {
		setEditingRule(rule);
		setForm(ruleToForm(rule));
		setIsFormOpen(true);
	}

	function closeForm() {
		setIsFormOpen(false);
		setEditingRule(null);
		setForm(EMPTY_FORM);
	}

	function handleSave() {
		const result = buildPayload(form);
		if ("error" in result) {
			toast.error(result.error);
			return;
		}
		if (editingRule) {
			const payload = { ...result.payload, enabled: editingRule.enabled };
			updateMutation.mutate(
				{ id: editingRule.id, rule: payload },
				{
					onSuccess: (rule) => {
						toast.success(`Rule "${rule.name}" updated`);
						closeForm();
					},
					onError: (err) => toast.error(err.message),
				},
			);
		} else {
			createMutation.mutate(result.payload, {
				onSuccess: (rule) => {
					toast.success(`Rule "${rule.name}" created`);
					closeForm();
				},
				onError: (err) => toast.error(err.message),
			});
		}
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

	const isSaving = createMutation.isPending || updateMutation.isPending;

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
										>
											Edit
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

			{isFormOpen ? (
				<div className="border rounded-md p-3 space-y-3">
					<div className="flex items-end gap-3">
						<div className="space-y-1.5 flex-1">
							<Label htmlFor="alert-rule-name">Name</Label>
							<Input
								id="alert-rule-name"
								value={form.name}
								onChange={(e) => set("name", e.target.value)}
								placeholder="api errors"
								className="h-8"
								maxLength={64}
							/>
						</div>
						<div className="flex gap-1">
							<Button
								type="button"
								size="sm"
								variant={form.type === "log" ? "default" : "outline"}
								onClick={() => set("type", "log")}
							>
								Log
							</Button>
							<Button
								type="button"
								size="sm"
								variant={form.type === "event" ? "default" : "outline"}
								onClick={() => set("type", "event")}
							>
								Event
							</Button>
						</div>
					</div>

					{form.type === "log" ? (
						<div className="flex items-end gap-3 flex-wrap">
							<div className="space-y-1.5">
								<Label>Min level</Label>
								<Select
									value={form.minLevel}
									onValueChange={(v) => set("minLevel", v)}
								>
									<SelectTrigger size="sm" className="w-32">
										<SelectValue />
									</SelectTrigger>
									<SelectContent>
										<SelectItem value="any">any</SelectItem>
										{LOG_LEVELS.map((level) => (
											<SelectItem key={level} value={level}>
												{level}
											</SelectItem>
										))}
									</SelectContent>
								</Select>
							</div>
							<div className="space-y-1.5 flex-1 min-w-40">
								<Label htmlFor="alert-rule-pattern">Pattern (regex)</Label>
								<Input
									id="alert-rule-pattern"
									value={form.pattern}
									onChange={(e) => set("pattern", e.target.value)}
									placeholder="connection refused"
									className="h-8 font-mono"
								/>
							</div>
							<div className="space-y-1.5 w-24">
								<Label htmlFor="alert-rule-threshold">Threshold</Label>
								<Input
									id="alert-rule-threshold"
									type="number"
									min={1}
									max={1000}
									value={form.threshold}
									onChange={(e) => set("threshold", e.target.value)}
									className="h-8"
								/>
							</div>
							<div className="space-y-1.5 w-28">
								<Label htmlFor="alert-rule-window">Window (s)</Label>
								<Input
									id="alert-rule-window"
									type="number"
									min={5}
									max={3600}
									value={form.windowSeconds}
									onChange={(e) => set("windowSeconds", e.target.value)}
									placeholder="60"
									className="h-8"
								/>
							</div>
						</div>
					) : (
						<div className="space-y-2">
							<div className="flex items-end gap-3 flex-wrap">
								<div className="flex items-center gap-4 h-8">
									<EventCheckbox
										label="die"
										checked={form.die}
										onToggle={() => set("die", !form.die)}
									/>
									<EventCheckbox
										label="oom"
										checked={form.oom}
										onToggle={() => set("oom", !form.oom)}
									/>
								</div>
								<div className="space-y-1.5 w-24">
									<Label htmlFor="alert-rule-threshold">Threshold</Label>
									<Input
										id="alert-rule-threshold"
										type="number"
										min={1}
										max={1000}
										value={form.threshold}
										onChange={(e) => set("threshold", e.target.value)}
										className="h-8"
									/>
								</div>
								<div className="space-y-1.5 w-28">
									<Label htmlFor="alert-rule-window">Window (s)</Label>
									<Input
										id="alert-rule-window"
										type="number"
										min={5}
										max={3600}
										value={form.windowSeconds}
										onChange={(e) => set("windowSeconds", e.target.value)}
										placeholder="120"
										className="h-8"
									/>
								</div>
							</div>
							<p className="text-xs text-muted-foreground">
								Tip: threshold 3 in a 120s window catches crash-loops.
							</p>
						</div>
					)}

					<div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
						<div className="space-y-1.5">
							<Label htmlFor="alert-rule-containers">Containers</Label>
							<Input
								id="alert-rule-containers"
								value={form.containers}
								onChange={(e) => set("containers", e.target.value)}
								placeholder="api, worker"
								className="h-8"
							/>
						</div>
						<div className="space-y-1.5">
							<Label htmlFor="alert-rule-projects">Projects</Label>
							<Input
								id="alert-rule-projects"
								value={form.projects}
								onChange={(e) => set("projects", e.target.value)}
								placeholder="myapp"
								className="h-8"
							/>
						</div>
						<div className="space-y-1.5">
							<Label htmlFor="alert-rule-hosts">Hosts</Label>
							<Input
								id="alert-rule-hosts"
								value={form.hosts}
								onChange={(e) => set("hosts", e.target.value)}
								placeholder="local"
								className="h-8"
							/>
						</div>
						<div className="space-y-1.5">
							<Label htmlFor="alert-rule-cooldown">Cooldown (s)</Label>
							<Input
								id="alert-rule-cooldown"
								type="number"
								min={0}
								max={86400}
								value={form.cooldownSeconds}
								onChange={(e) => set("cooldownSeconds", e.target.value)}
								placeholder="300 (default)"
								className="h-8"
							/>
						</div>
					</div>
					<p className="text-xs text-muted-foreground">
						Containers, projects, and hosts are comma-separated. Leave empty to
						match all containers.
					</p>

					<div className="flex gap-1">
						<Button size="sm" disabled={isSaving} onClick={handleSave}>
							{isSaving ? (
								<>
									<Spinner className="size-3" />
									Saving...
								</>
							) : editingRule ? (
								"Save"
							) : (
								"Create"
							)}
						</Button>
						<Button size="sm" variant="ghost" onClick={closeForm}>
							Cancel
						</Button>
					</div>
				</div>
			) : (
				<Button variant="outline" size="sm" onClick={openCreate}>
					Create rule
				</Button>
			)}

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
