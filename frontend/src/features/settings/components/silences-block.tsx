import { useState } from "react";
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
import { Spinner } from "@/components/ui/spinner";

import { useAlertRules } from "../hooks/use-alerts";
import {
	useAlertSilences,
	useCreateAlertSilence,
	useDeleteAlertSilence,
} from "../hooks/use-alert-silences";
import { useTargetOptions } from "../hooks/use-target-options";
import { ToggleChip } from "./alert-rule-dialog";
import { showResultToast } from "./mutation-toast";
import {
	ErrorNote,
	Note,
	SettingsSubsection,
	SettingsTable,
	TBody,
	Td,
	Th,
	THead,
} from "./settings-ui";
import {
	buildSilencePayload,
	describeSilenceTarget,
	describeSilenceWhen,
	EMPTY_SILENCE_DRAFT,
	SILENCE_DAYS,
	type SilenceDraft,
} from "./silence-utils";

// Radix Select can't hold an empty value, so "any" stands in for "".
const ANY = "__any";

const MODES: { value: SilenceDraft["mode"]; label: string }[] = [
	{ value: "for", label: "For" },
	{ value: "indefinite", label: "Until unmuted" },
	{ value: "between", label: "Between" },
	{ value: "weekly", label: "Every week" },
];

export function SilencesBlock() {
	const { data, isLoading, error } = useAlertSilences();
	const { data: rulesData } = useAlertRules();
	const deleteMutation = useDeleteAlertSilence();
	const [isDialogOpen, setIsDialogOpen] = useState(false);

	const silences = data?.silences ?? [];
	const rules = rulesData?.rules ?? [];

	return (
		<SettingsSubsection
			title="Silences"
			action={
				<Button
					variant="outline"
					size="sm"
					onClick={() => setIsDialogOpen(true)}
				>
					Mute alerts
				</Button>
			}
		>
			<div className="space-y-3">
				{isLoading && <Spinner className="size-4" />}
				{error && (
					<ErrorNote>Failed to load silences: {error.message}</ErrorNote>
				)}

				{!isLoading && !error && silences.length === 0 && (
					<Note>
						Nothing is muted. Muted alerts still land in history, marked muted,
						and go to no channel.
					</Note>
				)}

				{silences.length > 0 && (
					<SettingsTable>
						<THead>
							<Th>Mutes</Th>
							<Th>When</Th>
							<Th>Comment</Th>
							<Th className="text-right">
								<span className="sr-only">Actions</span>
							</Th>
						</THead>
						<TBody>
							{silences.map((silence) => (
								<tr key={silence.id}>
									<Td className="font-medium">
										{describeSilenceTarget(silence, rules)}
									</Td>
									<Td
										className={
											silence.active
												? "text-foreground"
												: "text-muted-foreground"
										}
									>
										{describeSilenceWhen(silence)}
									</Td>
									<Td className="text-muted-foreground">
										{silence.comment ?? ""}
									</Td>
									<Td className="text-right">
										<Button
											variant="ghost"
											size="sm"
											disabled={deleteMutation.isPending}
											onClick={() =>
												deleteMutation.mutate(silence.id, showResultToast)
											}
										>
											{silence.active ? "Unmute" : "Delete"}
										</Button>
									</Td>
								</tr>
							))}
						</TBody>
					</SettingsTable>
				)}

				<SilenceDialog open={isDialogOpen} onOpenChange={setIsDialogOpen} />
			</div>
		</SettingsSubsection>
	);
}

interface SilenceDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	/** Prefills the form, e.g. the rule a row's Mute button belongs to. */
	initial?: Partial<SilenceDraft>;
}

export function SilenceDialog({
	open,
	onOpenChange,
	initial,
}: SilenceDialogProps) {
	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-lg">
				{open && (
					<SilenceEditor
						initial={initial}
						onClose={() => onOpenChange(false)}
					/>
				)}
			</DialogContent>
		</Dialog>
	);
}

function MatcherSelect({
	label,
	value,
	options,
	onChange,
}: {
	label: string;
	value: string;
	options: { value: string; label: string }[];
	onChange: (value: string) => void;
}) {
	return (
		<div className="space-y-1.5">
			<Label>{label}</Label>
			<Select
				value={value || ANY}
				onValueChange={(v) => onChange(v === ANY ? "" : v)}
			>
				<SelectTrigger size="sm" className="w-full">
					<SelectValue />
				</SelectTrigger>
				<SelectContent>
					<SelectItem value={ANY}>Any</SelectItem>
					{options.map((option) => (
						<SelectItem key={option.value} value={option.value}>
							{option.label}
						</SelectItem>
					))}
				</SelectContent>
			</Select>
		</div>
	);
}

function SilenceEditor({
	initial,
	onClose,
}: {
	initial?: Partial<SilenceDraft>;
	onClose: () => void;
}) {
	const [draft, setDraft] = useState<SilenceDraft>(() => ({
		...EMPTY_SILENCE_DRAFT,
		timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
		...initial,
	}));
	const { data: rulesData } = useAlertRules();
	const targetOptions = useTargetOptions();
	const createMutation = useCreateAlertSilence();

	function set<K extends keyof SilenceDraft>(key: K, value: SilenceDraft[K]) {
		setDraft((prev) => ({ ...prev, [key]: value }));
	}

	const asOptions = (values: string[]) =>
		values.map((value) => ({ value, label: value }));

	function handleSave() {
		const result = buildSilencePayload(draft, new Date());
		if ("error" in result) {
			toast.error(result.error);
			return;
		}
		createMutation.mutate(result.payload, {
			onSuccess: () => {
				toast.success("Silence added");
				onClose();
			},
			onError: (err) => toast.error(err.message),
		});
	}

	return (
		<div className="space-y-5">
			<DialogHeader>
				<DialogTitle>Mute alerts</DialogTitle>
				<DialogDescription>
					Muted alerts are kept in history, marked muted, and sent to no
					channel. Leave every field on Any to mute everything.
				</DialogDescription>
			</DialogHeader>

			<div className="grid grid-cols-2 gap-3">
				<MatcherSelect
					label="Rule"
					value={draft.ruleId}
					options={(rulesData?.rules ?? []).map((rule) => ({
						value: rule.id,
						label: rule.name,
					}))}
					onChange={(v) => set("ruleId", v)}
				/>
				<MatcherSelect
					label="Host"
					value={draft.host}
					options={asOptions(targetOptions.hosts)}
					onChange={(v) => set("host", v)}
				/>
				<MatcherSelect
					label="Container"
					value={draft.container}
					options={asOptions(targetOptions.containers)}
					onChange={(v) => set("container", v)}
				/>
				<MatcherSelect
					label="Compose project"
					value={draft.project}
					options={asOptions(targetOptions.projects)}
					onChange={(v) => set("project", v)}
				/>
			</div>

			<div className="space-y-3">
				<div className="flex flex-wrap gap-1">
					{MODES.map((mode) => (
						<Button
							key={mode.value}
							type="button"
							size="sm"
							variant={draft.mode === mode.value ? "default" : "outline"}
							onClick={() => set("mode", mode.value)}
						>
							{mode.label}
						</Button>
					))}
				</div>

				{draft.mode === "for" && (
					<div className="flex items-center gap-1.5 text-sm">
						Mute for
						<Input
							type="number"
							min={1}
							value={draft.forValue}
							onChange={(e) => set("forValue", e.target.value)}
							aria-label="Mute for"
							className="h-8 w-20 text-center tabular-nums"
						/>
						<Select
							value={draft.forUnit}
							onValueChange={(v) =>
								set("forUnit", v === "minutes" || v === "days" ? v : "hours")
							}
						>
							<SelectTrigger size="sm" className="w-28" aria-label="Unit">
								<SelectValue />
							</SelectTrigger>
							<SelectContent>
								<SelectItem value="minutes">minutes</SelectItem>
								<SelectItem value="hours">hours</SelectItem>
								<SelectItem value="days">days</SelectItem>
							</SelectContent>
						</Select>
					</div>
				)}

				{draft.mode === "indefinite" && (
					<p className="text-sm text-muted-foreground">
						Muted until you unmute it here or with{" "}
						<code>logdeck alerts silences delete</code>.
					</p>
				)}

				{draft.mode === "between" && (
					<div className="grid grid-cols-2 gap-3">
						<div className="space-y-1.5">
							<Label htmlFor="silence-from">From</Label>
							<Input
								id="silence-from"
								type="datetime-local"
								value={draft.from}
								onChange={(e) => set("from", e.target.value)}
								className="h-8"
							/>
						</div>
						<div className="space-y-1.5">
							<Label htmlFor="silence-until">Until</Label>
							<Input
								id="silence-until"
								type="datetime-local"
								value={draft.until}
								onChange={(e) => set("until", e.target.value)}
								className="h-8"
							/>
						</div>
					</div>
				)}

				{draft.mode === "weekly" && (
					<div className="space-y-3">
						<div className="flex flex-wrap gap-1.5">
							{SILENCE_DAYS.map((day) => (
								<ToggleChip
									key={day.value}
									label={day.label}
									pressed={draft.days.includes(day.value)}
									onToggle={() =>
										set(
											"days",
											draft.days.includes(day.value)
												? draft.days.filter((d) => d !== day.value)
												: [...draft.days, day.value],
										)
									}
								/>
							))}
						</div>
						<div className="flex flex-wrap items-center gap-1.5 text-sm">
							At
							<Input
								type="time"
								value={draft.at}
								onChange={(e) => set("at", e.target.value)}
								aria-label="Window start"
								className="h-8 w-36"
							/>
							for
							<Input
								type="number"
								min={1}
								max={1440}
								value={draft.durationMinutes}
								onChange={(e) => set("durationMinutes", e.target.value)}
								aria-label="Window length in minutes"
								className="h-8 w-20 text-center tabular-nums"
							/>
							minutes, in
							<Input
								value={draft.timezone}
								onChange={(e) => set("timezone", e.target.value)}
								aria-label="Time zone"
								placeholder="UTC"
								className="h-8 w-40 font-mono"
							/>
						</div>
						<p className="text-xs text-muted-foreground">
							No day picked means every day. A window lasts at most a day.
						</p>
					</div>
				)}
			</div>

			<div className="space-y-1.5">
				<Label htmlFor="silence-comment">
					Comment{" "}
					<span className="font-normal text-muted-foreground">(optional)</span>
				</Label>
				<Input
					id="silence-comment"
					value={draft.comment}
					onChange={(e) => set("comment", e.target.value)}
					placeholder="Deploying v2.3"
					maxLength={200}
					className="h-8"
				/>
			</div>

			<div className="flex justify-end gap-2">
				<Button type="button" variant="ghost" size="sm" onClick={onClose}>
					Cancel
				</Button>
				<Button
					type="button"
					size="sm"
					disabled={createMutation.isPending}
					onClick={handleSave}
				>
					{createMutation.isPending ? (
						<>
							<Spinner className="size-3" />
							Saving...
						</>
					) : (
						"Mute"
					)}
				</Button>
			</div>
		</div>
	);
}
