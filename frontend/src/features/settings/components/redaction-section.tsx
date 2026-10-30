import { useState } from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { PlusIcon, Trash2Icon } from "@/components/ui/icons";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";

import type {
	RedactionConfig,
	RedactionResponse,
	RedactionRule,
} from "../api/get-redaction";
import {
	usePreviewRedaction,
	useRedaction,
	useUpdateRedaction,
} from "../hooks/use-settings";
import { useTargetOptions } from "../hooks/use-target-options";
import { MultiCombobox } from "./multi-combobox";
import { showResultToast } from "./mutation-toast";
import {
	cleanRedactionConfig,
	EMPTY_REDACTION_RULE,
	redactionDraft,
	validateRedactionDraft,
} from "./redaction-utils";
import { SaveButton } from "./save-button";
import {
	ErrorNote,
	Field,
	Note,
	SettingsSection,
	SettingsSubsection,
	Well,
} from "./settings-ui";

const TITLE = "Log redaction";
const DESCRIPTION =
	"Masks secrets in log lines before LogDeck stores them or sends them anywhere: log views, history, downloads, the API, the CLI and MCP, and alert notifications. The patterns catch common formats, not every secret.";

const SAMPLE_PLACEHOLDER = `POST /login user=jane password=hunter2
Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.c2lnbmF0dXJl
connecting to postgres://app:s3cret@db:5432/app`;

const textareaClass =
	"w-full rounded-md border border-input bg-transparent p-2 font-mono text-sm";

interface RedactionSectionProps {
	/** Opens the Log storage tab, where stored history can be deleted. */
	onOpenStorage: () => void;
}

export function RedactionSection({ onOpenStorage }: RedactionSectionProps) {
	const { data, isLoading, error } = useRedaction();

	if (!data) {
		return (
			<SettingsSection title={TITLE} description={DESCRIPTION}>
				{isLoading && <Spinner className="size-4" />}
				{error && (
					<ErrorNote>
						Failed to load redaction settings: {error.message}
					</ErrorNote>
				)}
			</SettingsSection>
		);
	}

	return (
		<RedactionForm
			key={JSON.stringify(data)}
			data={data}
			onOpenStorage={onOpenStorage}
		/>
	);
}

function RedactionForm({
	data,
	onOpenStorage,
}: {
	data: RedactionResponse;
	onOpenStorage: () => void;
}) {
	const [draft, setDraft] = useState(() => redactionDraft(data));
	const updateMutation = useUpdateRedaction();
	const hasChanges =
		JSON.stringify(cleanRedactionConfig(draft)) !==
		JSON.stringify(cleanRedactionConfig(redactionDraft(data)));

	function set<K extends keyof RedactionConfig>(
		key: K,
		value: RedactionConfig[K],
	) {
		setDraft((prev) => ({ ...prev, [key]: value }));
	}

	function setRule(index: number, rule: RedactionRule) {
		set(
			"rules",
			draft.rules.map((current, i) => (i === index ? rule : current)),
		);
	}

	function toggleBuiltin(id: string, enabled: boolean) {
		// Kept in catalog order; the server runs them in that order anyway.
		set(
			"builtins",
			data.catalog
				.map((builtin) => builtin.id)
				.filter((builtinId) =>
					builtinId === id ? enabled : draft.builtins.includes(builtinId),
				),
		);
	}

	function handleSave() {
		const problem = validateRedactionDraft(draft);
		if (problem) {
			toast.error(problem);
			return;
		}
		updateMutation.mutate(cleanRedactionConfig(draft), showResultToast);
	}

	return (
		<SettingsSection
			title={TITLE}
			description={DESCRIPTION}
			badge={
				<Badge className="border-transparent bg-muted font-normal text-muted-foreground">
					{data.enabled ? "On" : "Off"}
				</Badge>
			}
			action={
				<Switch
					id="redaction-enabled"
					aria-label="Enable log redaction"
					checked={draft.enabled}
					onCheckedChange={(checked) => set("enabled", checked)}
					className="relative after:absolute after:-inset-x-1 after:-inset-y-3"
				/>
			}
		>
			<div className="space-y-8">
				{!data.enabled && (
					<Note>
						Redaction is off: logs are stored and shown exactly as containers
						write them. Choose the patterns below, try them in the preview, then
						turn it on and save.
					</Note>
				)}

				<SettingsSubsection title="Built-in patterns">
					<ul className="divide-y divide-border/60">
						{data.catalog.map((builtin) => (
							<li key={builtin.id} className="flex items-start gap-3 py-3">
								<Switch
									id={`redaction-builtin-${builtin.id}`}
									checked={draft.builtins.includes(builtin.id)}
									onCheckedChange={(checked) =>
										toggleBuiltin(builtin.id, checked)
									}
									className="mt-0.5"
								/>
								<label
									htmlFor={`redaction-builtin-${builtin.id}`}
									className="min-w-0"
								>
									<span className="block text-base font-medium sm:text-sm">
										{builtin.name}
									</span>
									<span className="block text-base/6 text-muted-foreground sm:text-sm/6">
										{builtin.description}
									</span>
								</label>
							</li>
						))}
					</ul>
				</SettingsSubsection>

				<div className="max-w-xs">
					<Field
						id="redaction-replacement"
						label="Replacement text"
						hint="Stands in for every hidden value. Keys such as password= are kept."
					>
						<Input
							id="redaction-replacement"
							name="replacement"
							value={draft.replacement ?? ""}
							placeholder={data.defaultReplacement}
							onChange={(e) => set("replacement", e.target.value)}
						/>
					</Field>
				</div>

				<SettingsSubsection
					title="Custom patterns"
					action={
						<Button
							variant="ghost"
							size="sm"
							onClick={() =>
								set("rules", [...draft.rules, { ...EMPTY_REDACTION_RULE }])
							}
						>
							<PlusIcon className="size-4" />
							Add pattern
						</Button>
					}
				>
					<div className="space-y-3">
						{draft.rules.length === 0 && (
							<Note>
								No custom patterns. Add one for secrets the built-ins don't
								know, such as internal IDs or customer data.
							</Note>
						)}
						{draft.rules.map((rule, index) => (
							<RuleFields
								// Rules have no id; position is their identity until saved.
								key={index}
								index={index}
								rule={rule}
								onChange={(next) => setRule(index, next)}
								onRemove={() =>
									set(
										"rules",
										draft.rules.filter((_, i) => i !== index),
									)
								}
							/>
						))}
					</div>
				</SettingsSubsection>

				<PreviewBlock config={draft} />

				<div className="max-w-prose space-y-2">
					<Note>
						Built-in patterns run first, then custom patterns from top to
						bottom, each on the text the previous one left. Every line is
						redacted on its own before it is parsed, so its message, raw text
						and fields all show the redacted value. The timestamp Docker adds is
						never changed.
					</Note>
					<Note>
						Alert rules match the original line, so turning redaction on doesn't
						change which alerts fire. The line an alert keeps in its history and
						sends in notifications is redacted.
					</Note>
					<Note>
						Saved changes apply to every line read afterwards, including log
						views already open. Logs stored before, alert history, and copies
						already shown or sent keep their original text. You can{" "}
						<button
							type="button"
							onClick={onOpenStorage}
							className="font-medium text-foreground underline underline-offset-2"
						>
							delete stored logs under Log storage
						</button>
						.
					</Note>
				</div>

				{hasChanges && (
					<SaveButton
						isPending={updateMutation.isPending}
						onClick={handleSave}
					/>
				)}
			</div>
		</SettingsSection>
	);
}

function RuleFields({
	index,
	rule,
	onChange,
	onRemove,
}: {
	index: number;
	rule: RedactionRule;
	onChange: (rule: RedactionRule) => void;
	onRemove: () => void;
}) {
	const targetOptions = useTargetOptions();
	const id = `redaction-rule-${index}`;
	const set = <K extends keyof RedactionRule>(
		key: K,
		value: RedactionRule[K],
	) => onChange({ ...rule, [key]: value });
	const untargeted =
		!rule.containers?.length && !rule.projects?.length && !rule.hosts?.length;

	return (
		<Well>
			<div className="space-y-4">
				<div className="flex items-center justify-between gap-3">
					<div className="flex items-center gap-2.5">
						<Switch
							id={`${id}-enabled`}
							aria-label="Enable this pattern"
							checked={rule.enabled}
							onCheckedChange={(checked) => set("enabled", checked)}
						/>
						<span className="text-sm text-muted-foreground">
							{rule.enabled ? "Enabled" : "Disabled"}
						</span>
					</div>
					<Button
						variant="ghost"
						size="icon-sm"
						onClick={onRemove}
						className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
						aria-label={`Remove ${rule.name || `custom pattern ${index + 1}`}`}
					>
						<Trash2Icon className="size-4" />
					</Button>
				</div>
				<div className="grid gap-4 sm:grid-cols-2">
					<Field id={`${id}-name`} label="Name">
						<Input
							id={`${id}-name`}
							value={rule.name}
							placeholder="Customer IDs"
							onChange={(e) => set("name", e.target.value)}
							className="h-8"
						/>
					</Field>
					<Field
						id={`${id}-replacement`}
						label="Replacement"
						hint="Optional. $1 keeps the first capture group."
					>
						<Input
							id={`${id}-replacement`}
							value={rule.replacement ?? ""}
							placeholder="Replacement text"
							onChange={(e) => set("replacement", e.target.value)}
							className="h-8"
						/>
					</Field>
				</div>
				<Field
					id={`${id}-pattern`}
					label="Pattern"
					hint="A regular expression in RE2 syntax, as in Go. It is checked when you save or preview."
				>
					<Input
						id={`${id}-pattern`}
						value={rule.pattern}
						placeholder="(customer_id=)\d+"
						onChange={(e) => set("pattern", e.target.value)}
						className="h-8 font-mono"
						spellCheck={false}
						autoComplete="off"
					/>
				</Field>
				<div className="space-y-3">
					<Note>
						{untargeted
							? "Applies to every container. Narrow it down below."
							: "Applies only to the containers selected below."}
					</Note>
					<div className="grid gap-4 sm:grid-cols-3">
						<Field id={`${id}-containers`} label="Containers">
							<MultiCombobox
								id={`${id}-containers`}
								placeholder="Add container..."
								options={targetOptions.containers}
								selected={rule.containers ?? []}
								onChange={(values) => set("containers", values)}
							/>
						</Field>
						<Field id={`${id}-projects`} label="Compose projects">
							<MultiCombobox
								id={`${id}-projects`}
								placeholder="Add project..."
								options={targetOptions.projects}
								selected={rule.projects ?? []}
								onChange={(values) => set("projects", values)}
							/>
						</Field>
						<Field id={`${id}-hosts`} label="Hosts">
							<MultiCombobox
								id={`${id}-hosts`}
								placeholder="Add host..."
								options={targetOptions.hosts}
								selected={rule.hosts ?? []}
								onChange={(values) => set("hosts", values)}
							/>
						</Field>
					</div>
				</div>
			</div>
		</Well>
	);
}

function PreviewBlock({ config }: { config: RedactionConfig }) {
	const [sample, setSample] = useState("");
	const previewMutation = usePreviewRedaction();

	function handlePreview() {
		const problem = validateRedactionDraft(config);
		if (problem) {
			toast.error(problem);
			return;
		}
		previewMutation.mutate({ config: cleanRedactionConfig(config), sample });
	}

	return (
		<SettingsSubsection
			title="Preview"
			action={
				<Button
					variant="outline"
					size="sm"
					disabled={!sample || previewMutation.isPending}
					onClick={handlePreview}
				>
					{previewMutation.isPending && <Spinner className="size-3" />}
					Preview
				</Button>
			}
		>
			<div className="space-y-3">
				<textarea
					aria-label="Sample log lines"
					className={textareaClass}
					rows={4}
					spellCheck={false}
					value={sample}
					placeholder={SAMPLE_PLACEHOLDER}
					onChange={(e) => setSample(e.target.value)}
				/>
				<Note>
					Runs the patterns above, saved or not, as if redaction were on. Every
					enabled pattern applies, whatever its targeting. The sample is not
					saved or logged.
				</Note>
				{previewMutation.error && (
					<ErrorNote>{previewMutation.error.message}</ErrorNote>
				)}
				{previewMutation.data !== undefined && !previewMutation.error && (
					<pre className="overflow-x-auto rounded-md bg-muted/40 p-3 font-mono text-sm whitespace-pre-wrap">
						{previewMutation.data}
					</pre>
				)}
			</div>
		</SettingsSubsection>
	);
}
