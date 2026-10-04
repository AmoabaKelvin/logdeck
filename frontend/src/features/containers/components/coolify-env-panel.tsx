import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
	deployCoolifyEnv,
	saveCoolifyEnv,
	type CoolifyEnvChange,
} from "../api/coolify-env";
import type { EnvVariablesResponse } from "../api/get-container-env-variables";
import { isSecretKey } from "./env-file";
import { PanelNote } from "./container-panel-ui";

export function CoolifyEnvPanel({
	containerId,
	containerHost,
	configuration,
	isReadOnly,
}: {
	containerId: string;
	containerHost: string;
	configuration: EnvVariablesResponse;
	isReadOnly: boolean;
}) {
	const queryClient = useQueryClient();
	const [changes, setChanges] = useState<Record<string, CoolifyEnvChange>>({});
	const [newKey, setNewKey] = useState("");
	const [newValue, setNewValue] = useState("");
	const [saved, setSaved] = useState(false);
	const [revealed, setRevealed] = useState<ReadonlySet<string>>(new Set());
	const [needsReload, setNeedsReload] = useState(false);
	const variables = configuration.variables ?? [];
	const refresh = () =>
		queryClient.invalidateQueries({
			queryKey: ["container-env", containerId, containerHost],
		});
	const save = useMutation({
		mutationFn: () =>
			saveCoolifyEnv(containerId, containerHost, Object.values(changes)),
		onSuccess: async () => {
			setChanges({});
			setSaved(
				(previous) =>
					previous ||
					Object.values(changes).some((change) => !change.is_preview),
			);
			await refresh();
			toast.success("Environment saved in Coolify", {
				description: Object.values(changes).some((change) => !change.is_preview)
					? "Deploy through Coolify to apply production changes."
					: "Deploy the preview in Coolify to apply preview changes.",
			});
		},
		onError: async (error: Error) => {
			setSaved(false);
			setNeedsReload(true);
			await refresh();
			toast.error("Coolify save failed", { description: error.message });
		},
	});
	const deploy = useMutation({
		mutationFn: () => deployCoolifyEnv(containerId, containerHost),
		onSuccess: () => {
			setSaved(false);
			toast.success("Coolify deployment requested", {
				description:
					"Check Coolify for deployment progress. The container ID may change.",
			});
			queryClient.invalidateQueries({ queryKey: ["containers"] });
		},
		onError: (error: Error) =>
			toast.error("Deployment request failed", { description: error.message }),
	});
	const reload = useMutation({
		mutationFn: () =>
			queryClient.refetchQueries(
				{ queryKey: ["container-env", containerId, containerHost] },
				{ throwOnError: true },
			),
		onSuccess: () => {
			setChanges({});
			setNeedsReload(false);
		},
		onError: (error: Error) =>
			toast.error("Could not refresh Coolify configuration", {
				description: error.message,
			}),
	});
	const busy = save.isPending || deploy.isPending || reload.isPending;
	const locked = isReadOnly || busy || needsReload;
	const update = (identity: string, change: CoolifyEnvChange | undefined) => {
		setChanges((previous) => {
			const next = { ...previous };
			if (change) next[identity] = change;
			else delete next[identity];
			return next;
		});
	};
	return (
		<div className="space-y-4">
			<PanelNote>
				Editing Coolify's saved configuration. Saving keeps variable settings
				and does not restart the container. Service changes apply to the whole
				service. Hidden values stay unknown until explicitly replaced.
			</PanelNote>
			{variables.map((variable) => {
				const change = changes[variable.uuid];
				const removed = change?.remove;
				const masked =
					isSecretKey(variable.key) &&
					variable.value != null &&
					!revealed.has(variable.uuid);
				const settings = [
					variable.is_preview ? "Preview" : "Production",
					variable.is_buildtime && "Build",
					variable.is_runtime && "Runtime",
					variable.is_shared && "Shared reference",
					variable.is_literal && "Literal",
					variable.is_multiline && "Multiline",
					variable.is_shown_once && "Hidden in Coolify",
				]
					.filter(Boolean)
					.join(" · ");
				return (
					<div
						key={variable.uuid}
						className="space-y-2 border-b border-border pb-3"
					>
						<div className="flex items-center justify-between gap-2">
							<div className="min-w-0">
								<p className="break-all font-mono">{variable.key}</p>
								<p className="text-sm text-muted-foreground">{settings}</p>
							</div>
							{isSecretKey(variable.key) && variable.value != null && (
								<Button
									variant="ghost"
									onClick={() =>
										setRevealed((previous) => {
											const next = new Set(previous);
											if (next.has(variable.uuid)) next.delete(variable.uuid);
											else next.add(variable.uuid);
											return next;
										})
									}
								>
									{revealed.has(variable.uuid) ? "Hide value" : "Reveal value"}
								</Button>
							)}
							{!isReadOnly && (
								<Button
									variant="ghost"
									disabled={locked}
									onClick={() =>
										update(
											variable.uuid,
											removed
												? undefined
												: {
														uuid: variable.uuid,
														expected_value: change
															? change.expected_value
															: (variable.value ?? null),
														key: variable.key,
														is_preview: variable.is_preview,
														remove: true,
													},
										)
									}
								>
									{removed ? "Undo removal" : "Remove"}
								</Button>
							)}
						</div>
						{removed ? (
							<p className="text-sm text-muted-foreground">
								Will be removed when saved.
							</p>
						) : (
							<textarea
								aria-label={`${variable.key} ${variable.is_preview ? "preview" : "production"} value`}
								className="w-full rounded-md border border-input bg-transparent p-2 font-mono text-sm"
								rows={variable.is_multiline ? 4 : 1}
								readOnly={locked || masked}
								placeholder={
									variable.value == null
										? "Unknown value. Enter a replacement to change it."
										: undefined
								}
								value={
									masked ? "••••••••" : (change?.value ?? variable.value ?? "")
								}
								onChange={(event) =>
									update(
										variable.uuid,
										event.target.value === variable.value
											? undefined
											: {
													uuid: variable.uuid,
													expected_value: change
														? change.expected_value
														: (variable.value ?? null),
													key: variable.key,
													is_preview: variable.is_preview,
													value: event.target.value,
												},
									)
								}
							/>
						)}
					</div>
				);
			})}
			{Object.entries(changes)
				.filter(([, change]) => !change.uuid)
				.map(([identity, change]) => (
					<div key={identity} className="flex items-center gap-2">
						<span className="font-mono">{change.key}</span>
						<span className="text-sm text-muted-foreground">
							New production variable
						</span>
						<textarea
							aria-label={`${change.key} new value`}
							className="w-full rounded-md border border-input bg-transparent p-2 font-mono text-sm"
							value={change.value ?? ""}
							disabled={locked}
							onChange={(event) =>
								update(identity, { ...change, value: event.target.value })
							}
						/>
						<Button
							variant="ghost"
							disabled={locked}
							onClick={() => update(identity, undefined)}
						>
							Remove
						</Button>
					</div>
				))}
			{!isReadOnly && (
				<>
					<div className="flex flex-wrap gap-2">
						<Input
							aria-label="New Coolify variable name"
							placeholder="NAME"
							value={newKey}
							disabled={locked}
							onChange={(event) => setNewKey(event.target.value)}
						/>
						<textarea
							aria-label="New Coolify variable value"
							placeholder="Value"
							className="w-full rounded-md border border-input bg-transparent p-2 font-mono text-sm"
							value={newValue}
							disabled={locked}
							onChange={(event) => setNewValue(event.target.value)}
						/>
						<Button
							disabled={locked || !newKey.trim()}
							onClick={() => {
								const key = newKey.trim();
								if (
									variables.some(
										(variable) => variable.key === key && !variable.is_preview,
									) ||
									Object.values(changes).some(
										(change) => change.key === key && !change.is_preview,
									)
								) {
									toast.error(`${key} already exists`);
									return;
								}
								update(`new:${key}`, {
									key,
									value: newValue,
									is_preview: false,
								});
								setNewKey("");
								setNewValue("");
							}}
						>
							Add variable
						</Button>
					</div>
					<PanelNote>
						Coolify may also create a preview copy of new application variables.
						Shared references use their saved reference text. Preview changes
						apply when you deploy the preview in Coolify.
					</PanelNote>
					{needsReload && (
						<p role="alert" className="text-sm text-destructive">
							{save.error?.message} Review the refreshed configuration before
							retrying.{" "}
							<Button
								variant="ghost"
								disabled={busy}
								onClick={() => reload.mutate()}
							>
								Discard draft and review saved values
							</Button>
						</p>
					)}
					<div className="flex flex-wrap gap-2">
						<Button
							disabled={locked || Object.keys(changes).length === 0}
							onClick={() => save.mutate()}
						>
							{save.isPending ? "Saving…" : "Save in Coolify"}
						</Button>
						<Button
							variant="ghost"
							disabled={locked || Object.keys(changes).length === 0}
							onClick={() => {
								setChanges({});
								setNeedsReload(false);
							}}
						>
							Discard
						</Button>
						{saved && (
							<Button
								disabled={busy || Object.keys(changes).length > 0}
								onClick={() => {
									if (
										window.confirm(
											"Deploy this saved configuration through Coolify? This can restart the application or the whole service and cause downtime.",
										)
									)
										deploy.mutate();
								}}
							>
								{deploy.isPending
									? "Requesting deployment…"
									: configuration.resource_type === "application"
										? "Deploy production through Coolify"
										: "Restart service through Coolify"}
							</Button>
						)}
					</div>
				</>
			)}
		</div>
	);
}
