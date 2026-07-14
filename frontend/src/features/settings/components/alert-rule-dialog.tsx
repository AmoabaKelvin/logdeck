import { useQuery } from "@tanstack/react-query";
import {
	ArrowLeftIcon,
	CheckIcon,
	HeartPulseIcon,
	type LucideIcon,
	MemoryStickIcon,
	OctagonXIcon,
	RegexIcon,
	RotateCcwIcon,
	SlidersHorizontalIcon,
	TrendingUpIcon,
	XIcon,
} from "lucide-react";
import { type ReactNode, useMemo, useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
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
import { getContainers } from "@/features/containers/api/get-containers";
import {
	formatContainerName,
	getComposeProject,
} from "@/features/containers/components/container-utils";
import { cn } from "@/lib/utils";

import type { AlertRulePayload } from "../api/create-alert-rule";
import type { AlertEventKind, AlertRule } from "../api/get-alert-rules";
import { useCreateAlertRule, useUpdateAlertRule } from "../hooks/use-alerts";
import { MultiCombobox } from "./multi-combobox";

const LOG_LEVELS = [
	"TRACE",
	"DEBUG",
	"INFO",
	"WARN",
	"ERROR",
	"FATAL",
	"PANIC",
];

interface FormState {
	name: string;
	type: "log" | "event";
	minLevel: string;
	pattern: string;
	die: boolean;
	oom: boolean;
	unhealthy: boolean;
	rateEnabled: boolean;
	threshold: string;
	windowSeconds: string;
	containers: string[];
	projects: string[];
	hosts: string[];
	cooldownValue: string;
	cooldownUnit: "minutes" | "seconds";
}

const EMPTY_FORM: FormState = {
	name: "",
	type: "log",
	minLevel: "any",
	pattern: "",
	die: false,
	oom: false,
	unhealthy: false,
	rateEnabled: false,
	threshold: "1",
	windowSeconds: "",
	containers: [],
	projects: [],
	hosts: [],
	cooldownValue: "",
	cooldownUnit: "minutes",
};

interface Preset {
	id: string;
	icon: LucideIcon;
	title: string;
	description: string;
	form: Partial<FormState>;
	focus: "pattern" | "name" | null;
}

const PRESETS: Preset[] = [
	{
		id: "crashed",
		icon: OctagonXIcon,
		title: "Container crashed",
		description: "Alert whenever a container dies.",
		form: { name: "Container crashed", type: "event", die: true },
		focus: null,
	},
	{
		id: "crash-loop",
		icon: RotateCcwIcon,
		title: "Crash looping",
		description: "A container dies 3 times within 2 minutes.",
		form: {
			name: "Crash looping",
			type: "event",
			die: true,
			rateEnabled: true,
			threshold: "3",
			windowSeconds: "120",
		},
		focus: null,
	},
	{
		id: "oom",
		icon: MemoryStickIcon,
		title: "Out of memory",
		description: "A container is killed for exceeding its memory limit.",
		form: { name: "Out of memory", type: "event", oom: true },
		focus: null,
	},
	{
		id: "unhealthy",
		icon: HeartPulseIcon,
		title: "Health check failing",
		description: "A container's health check transitions to unhealthy.",
		form: { name: "Health check failing", type: "event", unhealthy: true },
		focus: null,
	},
	{
		id: "error-spike",
		icon: TrendingUpIcon,
		title: "Error spike",
		description: "5 lines at ERROR or above within a minute.",
		form: {
			name: "Error spike",
			type: "log",
			minLevel: "ERROR",
			rateEnabled: true,
			threshold: "5",
			windowSeconds: "60",
		},
		focus: null,
	},
	{
		id: "pattern",
		icon: RegexIcon,
		title: "Log pattern",
		description: "A log line matches a regular expression.",
		form: { name: "Log pattern", type: "log" },
		focus: "pattern",
	},
	{
		id: "custom",
		icon: SlidersHorizontalIcon,
		title: "Custom",
		description: "Start from a blank rule.",
		form: {},
		focus: "name",
	},
];

function ruleToForm(rule: AlertRule): FormState {
	const cooldown = rule.cooldownSeconds ?? 0;
	return {
		name: rule.name,
		type: rule.type,
		minLevel: rule.minLevel ?? "any",
		pattern: rule.pattern ?? "",
		die: rule.events?.includes("die") ?? false,
		oom: rule.events?.includes("oom") ?? false,
		unhealthy: rule.events?.includes("unhealthy") ?? false,
		rateEnabled: rule.threshold > 1 || Boolean(rule.windowSeconds),
		threshold: String(rule.threshold),
		windowSeconds: rule.windowSeconds ? String(rule.windowSeconds) : "",
		containers: rule.containers ?? [],
		projects: rule.projects ?? [],
		hosts: rule.hosts ?? [],
		cooldownValue:
			cooldown > 0
				? String(cooldown % 60 === 0 ? cooldown / 60 : cooldown)
				: "",
		cooldownUnit: cooldown > 0 && cooldown % 60 !== 0 ? "seconds" : "minutes",
	};
}

function buildPayload(
	form: FormState,
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
		if (form.unhealthy) events.push("unhealthy");
		if (events.length === 0) {
			return { error: "Event rules need at least one event" };
		}
		payload.events = events;
	}

	if (form.rateEnabled) {
		const threshold = Number.parseInt(form.threshold, 10);
		if (!Number.isNaN(threshold) && threshold > 0) {
			payload.threshold = threshold;
		}
		const windowSeconds = Number.parseInt(form.windowSeconds, 10);
		if (!Number.isNaN(windowSeconds) && windowSeconds > 0) {
			payload.windowSeconds = windowSeconds;
		}
	}

	const cooldown = Number.parseInt(form.cooldownValue, 10);
	if (!Number.isNaN(cooldown) && cooldown > 0) {
		payload.cooldownSeconds =
			form.cooldownUnit === "minutes" ? cooldown * 60 : cooldown;
	}

	if (form.containers.length > 0) payload.containers = form.containers;
	if (form.projects.length > 0) payload.projects = form.projects;
	if (form.hosts.length > 0) payload.hosts = form.hosts;

	return { payload };
}

function formatList(values: string[]): string {
	if (values.length <= 2) return values.join(" and ");
	return `${values[0]}, ${values[1]} and ${values.length - 2} more`;
}

function summarize(form: FormState): string {
	const targets: string[] = [];
	if (form.containers.length > 0) targets.push(formatList(form.containers));
	if (form.projects.length > 0) {
		targets.push(
			`${form.projects.length === 1 ? "project" : "projects"} ${formatList(form.projects)}`,
		);
	}
	if (form.hosts.length > 0) targets.push(`on ${formatList(form.hosts)}`);
	const target = targets.length > 0 ? targets.join(", ") : "any container";

	let condition: string;
	if (form.type === "log") {
		const qualifiers = [
			form.minLevel !== "any" ? `at ${form.minLevel} or above` : "",
			form.pattern.trim() ? `matching /${form.pattern.trim()}/` : "",
		]
			.filter(Boolean)
			.join(" ");
		if (form.rateEnabled) {
			condition = `logs ${form.threshold || "?"} lines${
				qualifiers ? ` ${qualifiers}` : ""
			} within ${form.windowSeconds || "?"}s`;
		} else {
			condition = `logs a line${qualifiers ? ` ${qualifiers}` : ""}`;
		}
	} else {
		const events =
			[
				form.die ? "dies" : "",
				form.oom ? "runs out of memory" : "",
				form.unhealthy ? "becomes unhealthy" : "",
			]
				.filter(Boolean)
				.join(" or ") || "…";
		condition = form.rateEnabled
			? `${events} ${form.threshold || "?"} times within ${form.windowSeconds || "?"}s`
			: events;
	}

	let quiet: string;
	if (form.cooldownValue) {
		const unit =
			form.cooldownValue === "1"
				? form.cooldownUnit.slice(0, -1)
				: form.cooldownUnit;
		quiet = `${form.cooldownValue} ${unit}`;
	} else {
		quiet = "5 minutes";
	}

	return `When ${target} ${condition} → alert, then stay quiet for ${quiet}.`;
}

// Extends the click target above/below the visual bounds so 32-36px controls
// meet a 40px minimum hit area.
const HIT_AREA = "relative after:absolute after:inset-x-0 after:-inset-y-1";

function SectionLabel({ children }: { children: ReactNode }) {
	return (
		<h4 className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
			{children}
		</h4>
	);
}

function ToggleChip({
	label,
	pressed,
	onToggle,
}: {
	label: string;
	pressed: boolean;
	onToggle: () => void;
}) {
	return (
		<button
			type="button"
			aria-pressed={pressed}
			onClick={onToggle}
			className={cn(
				"inline-flex h-8 items-center gap-1.5 rounded-md border px-3 text-xs font-medium transition-colors",
				HIT_AREA,
				pressed
					? "border-primary/50 bg-primary/10 text-primary"
					: "border-input text-muted-foreground hover:bg-accent hover:text-accent-foreground",
			)}
		>
			{pressed && <CheckIcon className="size-3" />}
			{label}
		</button>
	);
}

function InlineNumberInput({
	id,
	value,
	onChange,
	min,
	max,
	placeholder,
	ariaLabel,
}: {
	id: string;
	value: string;
	onChange: (value: string) => void;
	min: number;
	max: number;
	placeholder?: string;
	ariaLabel: string;
}) {
	return (
		<Input
			id={id}
			type="number"
			min={min}
			max={max}
			value={value}
			onChange={(e) => onChange(e.target.value)}
			placeholder={placeholder}
			aria-label={ariaLabel}
			className="h-8 w-16 text-center tabular-nums"
		/>
	);
}

function PresetGrid({ onPick }: { onPick: (preset: Preset) => void }) {
	return (
		<div className="grid grid-cols-1 gap-2.5 sm:grid-cols-2">
			{PRESETS.map((preset) => (
				<button
					key={preset.id}
					type="button"
					onClick={() => onPick(preset)}
					className="flex items-start gap-3 rounded-xl border p-3 text-left transition-colors hover:border-primary/40 hover:bg-accent/50"
				>
					<span className="flex size-9 shrink-0 items-center justify-center rounded-xs border bg-muted/50">
						<preset.icon className="size-4 text-muted-foreground" />
					</span>
					<span className="min-w-0 space-y-0.5">
						<span className="block text-sm font-medium">{preset.title}</span>
						<span className="block text-xs text-muted-foreground">
							{preset.description}
						</span>
					</span>
				</button>
			))}
		</div>
	);
}

interface AlertRuleDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	rule: AlertRule | null;
}

export function AlertRuleDialog({
	open,
	onOpenChange,
	rule,
}: AlertRuleDialogProps) {
	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent
				className="max-h-[85vh] overflow-y-auto sm:max-w-xl"
				onEscapeKeyDown={(e) => {
					// Escape while a target combobox list is open should close the
					// list (handled by the combobox itself), not dismiss the dialog.
					const target = e.target as HTMLElement | null;
					if (target?.closest("[data-combobox-open]")) e.preventDefault();
				}}
			>
				{open && <RuleEditor rule={rule} onClose={() => onOpenChange(false)} />}
			</DialogContent>
		</Dialog>
	);
}

function RuleEditor({
	rule,
	onClose,
}: {
	rule: AlertRule | null;
	onClose: () => void;
}) {
	const isEdit = rule !== null;
	const [screen, setScreen] = useState<"presets" | "form">(
		isEdit ? "form" : "presets",
	);
	const [form, setForm] = useState<FormState>(() =>
		rule ? ruleToForm(rule) : EMPTY_FORM,
	);
	const [initialFocus, setInitialFocus] = useState<"pattern" | "name" | null>(
		null,
	);

	const createMutation = useCreateAlertRule();
	const updateMutation = useUpdateAlertRule();
	const isSaving = createMutation.isPending || updateMutation.isPending;

	const containersQuery = useQuery({
		queryKey: ["containers"],
		queryFn: getContainers,
		staleTime: 30_000,
	});

	const targetOptions = useMemo(() => {
		const containers = new Set<string>();
		const projects = new Set<string>();
		const hosts = new Set<string>();
		for (const container of containersQuery.data?.containers ?? []) {
			const name = formatContainerName(container.names);
			if (name !== "—") containers.add(name);
			const project = getComposeProject(container.labels);
			if (project) projects.add(project);
			if (container.host) hosts.add(container.host);
		}
		for (const host of containersQuery.data?.hosts ?? []) {
			hosts.add(host.name);
		}
		return {
			containers: [...containers],
			projects: [...projects],
			hosts: [...hosts],
		};
	}, [containersQuery.data]);

	function set<K extends keyof FormState>(key: K, value: FormState[K]) {
		setForm((prev) => ({ ...prev, [key]: value }));
	}

	function applyPreset(preset: Preset) {
		setForm({ ...EMPTY_FORM, ...preset.form });
		setInitialFocus(preset.focus);
		setScreen("form");
	}

	function enableRate() {
		setForm((prev) => ({
			...prev,
			rateEnabled: true,
			threshold: Number.parseInt(prev.threshold, 10) > 1 ? prev.threshold : "3",
			windowSeconds: prev.windowSeconds || "60",
		}));
	}

	function disableRate() {
		setForm((prev) => ({
			...prev,
			rateEnabled: false,
			threshold: "1",
			windowSeconds: "",
		}));
	}

	function handleSave() {
		const result = buildPayload(form);
		if ("error" in result) {
			toast.error(result.error);
			return;
		}
		if (rule) {
			updateMutation.mutate(
				{ id: rule.id, rule: { ...result.payload, enabled: rule.enabled } },
				{
					onSuccess: (updated) => {
						toast.success(`Rule "${updated.name}" updated`);
						onClose();
					},
					onError: (err) => toast.error(err.message),
				},
			);
		} else {
			createMutation.mutate(result.payload, {
				onSuccess: (created) => {
					toast.success(`Rule "${created.name}" created`);
					onClose();
				},
				onError: (err) => toast.error(err.message),
			});
		}
	}

	if (screen === "presets") {
		return (
			<div className="animate-in fade-in-0 slide-in-from-left-2 space-y-4 duration-200">
				<DialogHeader>
					<DialogTitle>New alert rule</DialogTitle>
					<DialogDescription>
						Start from a preset, or build a custom rule from scratch.
					</DialogDescription>
				</DialogHeader>
				<PresetGrid onPick={applyPreset} />
			</div>
		);
	}

	return (
		<div className="animate-in fade-in-0 slide-in-from-right-2 space-y-5 duration-200">
			<DialogHeader>
				<DialogTitle className="flex items-center gap-2">
					{!isEdit && (
						<button
							type="button"
							onClick={() => setScreen("presets")}
							aria-label="Back to presets"
							className={cn(
								"-ml-1 flex size-6 items-center justify-center rounded-sm text-muted-foreground transition-colors hover:bg-accent hover:text-foreground",
								HIT_AREA,
							)}
						>
							<ArrowLeftIcon className="size-4" />
						</button>
					)}
					{isEdit ? "Edit alert rule" : "New alert rule"}
				</DialogTitle>
			</DialogHeader>

			{/* When */}
			<div className="space-y-3">
				<SectionLabel>When</SectionLabel>
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

				{form.type === "log" ? (
					<div className="space-y-3">
						<div className="flex items-start gap-3">
							<div className="space-y-1.5">
								<Label>Minimum level</Label>
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
							<div className="min-w-0 flex-1 space-y-1.5">
								<Label htmlFor="alert-rule-pattern">
									Pattern{" "}
									<span className="font-normal text-muted-foreground">
										(optional)
									</span>
								</Label>
								<Input
									id="alert-rule-pattern"
									value={form.pattern}
									onChange={(e) => set("pattern", e.target.value)}
									placeholder="connection refused"
									className="h-8 font-mono"
									autoFocus={initialFocus === "pattern"}
								/>
								<p className="text-xs text-muted-foreground">
									Regular expression, matched against the message.
								</p>
							</div>
						</div>
					</div>
				) : (
					<div className="flex items-center gap-2">
						<ToggleChip
							label="Container died"
							pressed={form.die}
							onToggle={() => set("die", !form.die)}
						/>
						<ToggleChip
							label="Out of memory"
							pressed={form.oom}
							onToggle={() => set("oom", !form.oom)}
						/>
						<ToggleChip
							label="Unhealthy"
							pressed={form.unhealthy}
							onToggle={() => set("unhealthy", !form.unhealthy)}
						/>
					</div>
				)}

				{form.rateEnabled ? (
					<div className="flex flex-wrap items-center gap-x-1.5 gap-y-2 text-sm">
						Alert after
						<InlineNumberInput
							id="alert-rule-threshold"
							value={form.threshold}
							onChange={(v) => set("threshold", v)}
							min={1}
							max={1000}
							ariaLabel="Occurrence threshold"
						/>
						occurrences within
						<InlineNumberInput
							id="alert-rule-window"
							value={form.windowSeconds}
							onChange={(v) => set("windowSeconds", v)}
							min={5}
							max={3600}
							placeholder="60"
							ariaLabel="Window in seconds"
						/>
						seconds
						<Button
							type="button"
							variant="ghost"
							size="icon"
							onClick={disableRate}
							aria-label="Remove rate condition"
							className="ml-0.5 size-6 text-muted-foreground hover:text-foreground"
						>
							<XIcon className="size-3.5" />
						</Button>
					</div>
				) : (
					<p className="text-sm">
						Alert on the first occurrence.{" "}
						<Button
							type="button"
							variant="link"
							size="sm"
							onClick={enableRate}
							className="h-auto p-0 text-xs"
						>
							Add a rate condition
						</Button>
					</p>
				)}
			</div>

			{/* Where */}
			<div className="space-y-3">
				<div className="space-y-1.5">
					<SectionLabel>Where</SectionLabel>
					{form.containers.length === 0 &&
						form.projects.length === 0 &&
						form.hosts.length === 0 && (
							<p className="text-xs text-muted-foreground">
								Applies to all containers. Narrow it down below.
							</p>
						)}
				</div>
				<div className="space-y-1.5">
					<Label htmlFor="alert-rule-containers">Containers</Label>
					<MultiCombobox
						id="alert-rule-containers"
						placeholder="Add container..."
						options={targetOptions.containers}
						selected={form.containers}
						onChange={(values) => set("containers", values)}
					/>
				</div>
				<div className="space-y-1.5">
					<Label htmlFor="alert-rule-projects">Compose projects</Label>
					<MultiCombobox
						id="alert-rule-projects"
						placeholder="Add compose project..."
						options={targetOptions.projects}
						selected={form.projects}
						onChange={(values) => set("projects", values)}
					/>
				</div>
				<div className="space-y-1.5">
					<Label htmlFor="alert-rule-hosts">Hosts</Label>
					<MultiCombobox
						id="alert-rule-hosts"
						placeholder="Add host..."
						options={targetOptions.hosts}
						selected={form.hosts}
						onChange={(values) => set("hosts", values)}
					/>
				</div>
			</div>

			{/* Then */}
			<div className="space-y-2">
				<SectionLabel>Then</SectionLabel>
				<div className="flex flex-wrap items-center gap-x-1.5 gap-y-2 text-sm">
					After alerting, stay quiet for
					<InlineNumberInput
						id="alert-rule-cooldown"
						value={form.cooldownValue}
						onChange={(v) => set("cooldownValue", v)}
						min={0}
						max={form.cooldownUnit === "minutes" ? 1440 : 86400}
						placeholder="5"
						ariaLabel={`Cooldown in ${form.cooldownUnit}`}
					/>
					{form.cooldownUnit}
					{form.cooldownValue === "" && (
						<span className="text-muted-foreground">(default)</span>
					)}
				</div>
				<p className="text-xs text-muted-foreground">
					Repeat occurrences during the quiet period are counted and reported on
					the next alert.
				</p>
			</div>

			<Separator />

			<div className="space-y-4">
				<p className="text-xs text-muted-foreground">{summarize(form)}</p>
				<div className="space-y-1.5">
					<Label htmlFor="alert-rule-name">Rule name</Label>
					<Input
						id="alert-rule-name"
						value={form.name}
						onChange={(e) => set("name", e.target.value)}
						placeholder="api errors"
						maxLength={64}
						className="h-8"
						autoFocus={initialFocus === "name"}
					/>
				</div>
				<div className="flex justify-end gap-2">
					<Button type="button" variant="ghost" size="sm" onClick={onClose}>
						Cancel
					</Button>
					<Button
						type="button"
						size="sm"
						disabled={isSaving}
						onClick={handleSave}
					>
						{isSaving ? (
							<>
								<Spinner className="size-3" />
								Saving...
							</>
						) : isEdit ? (
							"Save changes"
						) : (
							"Create rule"
						)}
					</Button>
				</div>
			</div>
		</div>
	);
}
