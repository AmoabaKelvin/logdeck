import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { ExternalLinkIcon, EyeIcon, EyeOffIcon } from "@/components/ui/icons";
import { Label } from "@/components/ui/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";

import {
	type DokployEnvironment,
	type DokployEnvResponse,
	type DokployResource,
	deployDokployEnv,
	readDokployEnv,
	saveDokployEnv,
} from "../api/dokploy-env";
import { getContainerEnvVariables } from "../api/get-container-env-variables";
import { PanelError, PanelLoading, PanelNote } from "./container-panel-ui";
import { readDokployMapping, writeDokployMapping } from "./dokploy-mapping";
import { type EnvConfirmCopy, EnvConfirmDialog } from "./env-confirm-dialog";

interface DokployEnvPanelProps {
	containerId: string;
	containerHost: string;
	configuration: DokployEnvResponse;
	isReadOnly: boolean;
	onPlainCompose?: () => void;
}

const LINK =
	"inline-flex shrink-0 items-center gap-1.5 rounded-sm text-base text-muted-foreground hover:text-foreground sm:text-sm";

const resourceKey = (resource: DokployResource) =>
	`${resource.type}/${resource.id}`;

function DokployLink({ href }: { href: string }) {
	return (
		<a href={href} target="_blank" rel="noreferrer" className={LINK}>
			Open Dokploy
			<ExternalLinkIcon className="size-4 shrink-0" />
		</a>
	);
}

export function DokployEnvPanel(props: DokployEnvPanelProps) {
	const { containerId, containerHost, configuration, onPlainCompose } = props;

	// Refetched only on demand: the list is re-read when the user backs out of
	// a deployment, so a mapping that changed meanwhile is confirmed afresh.
	const inventory = useQuery({
		queryKey: [
			"container-env",
			containerId,
			containerHost,
			"dokploy-inventory",
		],
		queryFn: async () => {
			const result = await getContainerEnvVariables(containerId, containerHost);
			if (result.source !== "dokploy") {
				throw new Error(
					"The Dokploy connection changed. Check Settings and reopen this panel.",
				);
			}
			return result;
		},
		initialData: configuration,
		enabled: false,
	});

	const resources = inventory.data.resources ?? [];
	const [choice, setChoice] = useState(() =>
		configuration.suggested_resources?.length === 1
			? configuration.suggested_resources[0]
			: "",
	);
	// A deployment confirmed on an earlier visit opens straight away.
	const [owner, setOwner] = useState(() => {
		const remembered = readDokployMapping(containerHost, containerId);
		return configuration.resources?.find(
			(resource) => resourceKey(resource) === remembered,
		);
	});
	const [confirmPlain, setConfirmPlain] = useState(false);
	const chosen = resources.find((resource) => resourceKey(resource) === choice);
	const plainAllowed =
		Boolean(inventory.data.plain_compose_allowed) && Boolean(onPlainCompose);

	if (owner) {
		return (
			<DokployEditor
				key={resourceKey(owner)}
				{...props}
				owner={owner}
				onUnmap={() => {
					writeDokployMapping(containerHost, containerId, undefined);
					setOwner(undefined);
					void inventory.refetch();
				}}
			/>
		);
	}

	if (inventory.isFetching) {
		return <PanelLoading label="Reading Dokploy deployments…" />;
	}
	if (inventory.isError) {
		return (
			<div className="space-y-3">
				<PanelError>{inventory.error.message}</PanelError>
				<Button variant="outline" onClick={() => void inventory.refetch()}>
					Reload deployments
				</Button>
			</div>
		);
	}

	return (
		<div className="space-y-4">
			{resources.length > 0 ? (
				<>
					<PanelNote>
						Choose the Dokploy application or Compose deployment that owns this
						container. Edits are saved to that deployment and reach all of its
						containers.
					</PanelNote>
					<div className="grid gap-1.5">
						<Label htmlFor="dokploy-owner" className="text-base sm:text-sm">
							Dokploy deployment
						</Label>
						<Select value={choice} onValueChange={setChoice}>
							<SelectTrigger id="dokploy-owner" className="w-full sm:max-w-lg">
								<SelectValue placeholder="Choose a deployment" />
							</SelectTrigger>
							<SelectContent>
								{resources.map((resource) => (
									<SelectItem
										key={resourceKey(resource)}
										value={resourceKey(resource)}
									>
										{resource.project} / {resource.environment} /{" "}
										{resource.name} · {resource.type}
									</SelectItem>
								))}
							</SelectContent>
						</Select>
					</div>
				</>
			) : inventory.data.inventory_error ? (
				<PanelError>{inventory.data.inventory_error}</PanelError>
			) : plainAllowed ? (
				<PanelNote>
					No application or Compose deployment was found on this Dokploy server.
					If Dokploy manages this container, check the server ID and API token
					permissions in Settings.
				</PanelNote>
			) : (
				<PanelError>
					This container is managed by Dokploy, but no application or Compose
					deployment was found on its server. Check the server ID and API token
					permissions in Settings.
				</PanelError>
			)}

			<div className="flex flex-wrap items-center gap-x-2 gap-y-3">
				{resources.length > 0 && (
					<Button
						disabled={!chosen}
						onClick={() => {
							if (!chosen) return;
							writeDokployMapping(
								containerHost,
								containerId,
								resourceKey(chosen),
							);
							setOwner(chosen);
						}}
					>
						Use this deployment
					</Button>
				)}
				{inventory.data.inventory_error && (
					<Button variant="outline" onClick={() => void inventory.refetch()}>
						Reload deployments
					</Button>
				)}
				{plainAllowed && (
					<Button variant="ghost" onClick={() => setConfirmPlain(true)}>
						Not managed by Dokploy
					</Button>
				)}
				<div className="ml-auto flex items-center gap-4">
					<Link to="/settings" className={LINK}>
						Settings
					</Link>
					<DokployLink href={configuration.instance_url} />
				</div>
			</div>

			<EnvConfirmDialog
				copy={
					confirmPlain
						? {
								title: "Edit as plain Compose?",
								description:
									"Use this only when the container is managed outside Dokploy. Saving recreates this container, and Compose can overwrite the change, so persist it in the Compose file too.",
								confirmLabel: "Use runtime editor",
							}
						: null
				}
				onOpenChange={setConfirmPlain}
				onConfirm={() => onPlainCompose?.()}
			/>
		</div>
	);
}

function DokployEditor({
	containerId,
	containerHost,
	configuration,
	isReadOnly,
	owner,
	onUnmap,
}: DokployEnvPanelProps & { owner: DokployResource; onUnmap: () => void }) {
	const queryClient = useQueryClient();
	const query = useQuery({
		queryKey: [
			"container-env",
			containerId,
			containerHost,
			"dokploy",
			owner.type,
			owner.id,
		],
		queryFn: () => readDokployEnv(containerId, containerHost, owner),
		refetchOnWindowFocus: false,
	});
	const [draft, setDraft] = useState<{ text: string; revision: string }>();
	const [saved, setSaved] = useState<DokployEnvironment>();
	const [revealed, setRevealed] = useState(false);
	// A save or deploy whose outcome is unknown. Dokploy may or may not hold
	// the change, so nothing more is sent until the saved state is re-read.
	const [failure, setFailure] = useState<string>();
	const [confirming, setConfirming] = useState<
		"deploy" | "reload" | "unmap" | null
	>(null);

	const original = saved ?? query.data?.configuration;
	const text = draft?.text ?? original?.text ?? "";
	const dirty = draft !== undefined && draft.text !== (original?.text ?? "");
	const scope =
		owner.type === "compose"
			? "the whole Compose deployment"
			: "the application";

	const save = useMutation({
		mutationFn: () =>
			saveDokployEnv(
				containerId,
				containerHost,
				owner,
				text,
				draft?.revision ?? original?.revision ?? "",
			),
		onSuccess: (result) => {
			setSaved(result.configuration);
			setDraft(undefined);
			toast.success("Environment saved in Dokploy", {
				description:
					"Deploy to apply it. The running deployment has not changed.",
			});
		},
		onError: (error: Error) => {
			setFailure(error.message);
			toast.error("Dokploy save failed", { description: error.message });
		},
	});
	const deploy = useMutation({
		mutationFn: () => deployDokployEnv(containerId, containerHost, owner),
		onSuccess: () => {
			toast.success("Dokploy deployment requested", {
				description:
					"Check Dokploy for deployment progress. The container ID may change.",
			});
			queryClient.invalidateQueries({ queryKey: ["containers"] });
		},
		onError: (error: Error) => {
			setFailure(error.message);
			toast.error("Deployment request failed", { description: error.message });
		},
	});
	const busy = save.isPending || deploy.isPending || query.isFetching;

	async function reload() {
		const result = await query.refetch();
		if (result.isSuccess) {
			setDraft(undefined);
			setSaved(undefined);
			setFailure(undefined);
		}
	}

	const confirmations = {
		deploy: {
			copy: {
				title: `Deploy ${owner.name}?`,
				description: `This deploys the saved configuration through Dokploy. It can restart ${scope} and cause downtime.`,
				confirmLabel: "Deploy",
			},
			run: () => deploy.mutate(),
		},
		reload: {
			copy: {
				title: "Discard unsaved edits?",
				description:
					"Reloading replaces your edits with the configuration saved in Dokploy.",
				confirmLabel: "Discard and reload",
			},
			run: () => void reload(),
		},
		unmap: {
			copy: {
				title: "Discard unsaved edits?",
				description: `Choosing another deployment discards your edits to ${owner.name}.`,
				confirmLabel: "Discard",
			},
			run: onUnmap,
		},
	} satisfies Record<string, { copy: EnvConfirmCopy; run: () => void }>;

	if (query.isLoading) {
		return <PanelLoading label="Reading Dokploy configuration…" />;
	}
	if (!original) {
		return (
			<div className="space-y-3">
				<PanelError>
					{query.error?.message || "Dokploy configuration is unavailable."}
				</PanelError>
				<Button variant="outline" onClick={onUnmap}>
					Choose another deployment
				</Button>
			</div>
		);
	}

	return (
		<div className="space-y-4">
			<div className="flex flex-wrap items-center justify-between gap-3">
				<div className="min-w-0 text-base sm:text-sm">
					<p className="break-words font-medium">
						{owner.name} · Dokploy {owner.type}
					</p>
					<p className="break-words text-muted-foreground">
						{owner.project} / {owner.environment} · {owner.appName}
					</p>
				</div>
				<Button
					variant="ghost"
					disabled={busy}
					onClick={() => (dirty ? setConfirming("unmap") : onUnmap())}
				>
					Change deployment
				</Button>
			</div>

			<PanelNote>
				{isReadOnly ? "Showing" : "Editing"}{" "}
				{owner.type === "compose"
					? "the deployment's .env configuration. Variables reach containers only when the Compose file references them or includes env_file."
					: "Dokploy's saved environment configuration. Build arguments and build secrets are kept as they are."}{" "}
				{!isReadOnly &&
					"Comments, multiline values, and shared or vault references stay as written. "}
				Environment-file creation is{" "}
				{original.createEnvFile ? "enabled" : "disabled"}.
			</PanelNote>

			{query.error && <PanelError>{query.error.message}</PanelError>}

			<div>
				<div className="flex min-h-8 items-center justify-between gap-3">
					<Label htmlFor="dokploy-env-text" className="text-base sm:text-sm">
						Saved environment
					</Label>
					<Button
						variant="ghost"
						onClick={() => setRevealed((prev) => !prev)}
						className="py-2 pr-3 pl-2 text-muted-foreground hover:text-foreground"
					>
						{revealed ? (
							<EyeOffIcon className="size-4" />
						) : (
							<EyeIcon className="size-4" />
						)}
						{revealed
							? "Hide values"
							: isReadOnly
								? "Reveal values"
								: "Reveal and edit"}
					</Button>
				</div>
				{revealed ? (
					<textarea
						id="dokploy-env-text"
						name="dokployEnvironment"
						value={text}
						spellCheck={false}
						autoComplete="off"
						readOnly={isReadOnly}
						disabled={busy}
						rows={14}
						className="mt-2 min-h-64 w-full rounded-md border border-input bg-transparent p-3 font-mono text-base shadow-xs outline-none transition-[color,box-shadow] focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 sm:text-sm dark:bg-input/30"
						onChange={(e) =>
							setDraft({
								text: e.target.value,
								revision: draft?.revision ?? original.revision,
							})
						}
					/>
				) : (
					<p className="mt-2 rounded-md bg-muted/40 p-4 text-base text-muted-foreground sm:text-sm">
						Values are hidden.
					</p>
				)}
			</div>

			{failure && (
				<p role="alert" className="text-base text-destructive sm:text-sm">
					{failure} Dokploy may or may not hold this change. Reload the saved
					configuration before retrying; your edits stay here until you do.
				</p>
			)}

			<div className="flex flex-wrap items-center gap-2">
				{!isReadOnly && (
					<>
						<Button
							disabled={busy || !dirty || Boolean(failure)}
							onClick={() => save.mutate()}
						>
							{save.isPending ? "Saving…" : "Save in Dokploy"}
						</Button>
						<Button
							variant="outline"
							disabled={busy || dirty || Boolean(failure)}
							onClick={() => setConfirming("deploy")}
						>
							{deploy.isPending
								? "Requesting deployment…"
								: "Deploy saved configuration"}
						</Button>
					</>
				)}
				<Button
					variant="ghost"
					disabled={busy}
					onClick={() => (dirty ? setConfirming("reload") : void reload())}
				>
					Reload configuration
				</Button>
				<div className="ml-auto">
					<DokployLink href={configuration.instance_url} />
				</div>
			</div>

			<EnvConfirmDialog
				copy={confirming ? confirmations[confirming].copy : null}
				onOpenChange={(open) => {
					if (!open) setConfirming(null);
				}}
				onConfirm={() => {
					if (confirming) confirmations[confirming].run();
				}}
			/>
		</div>
	);
}
