import { useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";

import type { DokployHostInput } from "../api/update-dokploy-hosts";
import {
	useTestDokployHost,
	useUpdateDokployHosts,
} from "../hooks/use-settings";
import type { DokployHostsConfig } from "../types";
import { showResultToast } from "./mutation-toast";
import { SaveButton } from "./save-button";
import {
	EnvBadge,
	Field,
	Note,
	Outcome,
	SettingsSection,
	SettingsTable,
	TBody,
	Td,
	Th,
	THead,
	Well,
} from "./settings-ui";
import { TruncatedValue } from "./truncated-value";

interface DokployHostsSectionProps {
	config: DokployHostsConfig;
	/** Names of the configured Docker hosts a connection can attach to. */
	dockerHosts: string[];
}

const EMPTY_HOST: DokployHostInput = {
	hostName: "",
	apiURL: "",
	apiToken: "",
	serverId: "",
};

// Must match the server's secretMask constant. GET /settings returns tokens
// masked with this value, and the server only resolves it back to the stored
// token when the host name and API URL match an existing entry.
const SECRET_MASK = "••••••••";

const LOCAL_SERVER = "Instance's own server";

function normalize(host: DokployHostInput): DokployHostInput {
	return {
		hostName: host.hostName,
		apiURL: host.apiURL.trim().replace(/\/+$/, ""),
		apiToken: host.apiToken.trim(),
		serverId: host.serverId.trim(),
	};
}

function HostSelect({
	id,
	label,
	value,
	options,
	onChange,
}: {
	id?: string;
	label?: string;
	value: string;
	options: string[];
	onChange: (value: string) => void;
}) {
	return (
		<Select value={value} onValueChange={onChange}>
			<SelectTrigger id={id} aria-label={label} className="h-8 w-full">
				<SelectValue placeholder="Choose a host" />
			</SelectTrigger>
			<SelectContent>
				{options.map((name) => (
					<SelectItem key={name} value={name}>
						{name}
					</SelectItem>
				))}
			</SelectContent>
		</Select>
	);
}

export function DokployHostsSection({
	config,
	dockerHosts,
}: DokployHostsSectionProps) {
	const envHosts = config.hosts.filter((h) => h.source === "env");
	const originalFileHosts = config.hosts
		.filter((h) => h.source !== "env")
		.map(({ hostName, apiURL, apiToken, serverId }) => ({
			hostName,
			apiURL,
			apiToken,
			serverId,
		}));

	const [fileHosts, setFileHosts] =
		useState<DokployHostInput[]>(originalFileHosts);
	const [editingIndex, setEditingIndex] = useState<number | null>(null);
	const [editingHost, setEditingHost] = useState<DokployHostInput>(EMPTY_HOST);
	const [isAdding, setIsAdding] = useState(false);
	const [newHost, setNewHost] = useState<DokployHostInput>(EMPTY_HOST);
	const [testResults, setTestResults] = useState<
		Record<string, { success: boolean; message: string }>
	>({});
	const [testingKey, setTestingKey] = useState<string | null>(null);

	const updateMutation = useUpdateDokployHosts();
	const testMutation = useTestDokployHost();

	const hasChanges =
		fileHosts.length !== originalFileHosts.length ||
		fileHosts.some(
			(h, i) =>
				h.hostName !== originalFileHosts[i]?.hostName ||
				h.apiURL !== originalFileHosts[i]?.apiURL ||
				h.apiToken !== originalFileHosts[i]?.apiToken ||
				h.serverId !== originalFileHosts[i]?.serverId,
		);

	// A Docker host takes one connection, so the picker only offers the ones
	// still free, plus the one the edited row already holds.
	function availableHosts(exceptIndex?: number) {
		return dockerHosts.filter(
			(name) =>
				!envHosts.some((h) => h.hostName === name) &&
				!fileHosts.some((h, i) => i !== exceptIndex && h.hostName === name),
		);
	}

	function validate(host: DokployHostInput): string | null {
		if (!host.hostName || !host.apiURL || !host.apiToken) {
			return "Docker host, API URL, and API token are all required";
		}
		if (
			host.apiToken === SECRET_MASK &&
			!originalFileHosts.some(
				(h) => h.hostName === host.hostName && h.apiURL === host.apiURL,
			)
		) {
			return `Enter the API token for "${host.hostName}" — a masked token can only be kept for the same host and API URL`;
		}
		return null;
	}

	function handleSave() {
		updateMutation.mutate(
			{ hosts: fileHosts, revision: config.revision },
			showResultToast,
		);
	}

	function handleRemove(index: number) {
		setFileHosts((prev) => prev.filter((_, i) => i !== index));
		setTestResults({});
	}

	function handleStartEdit(index: number) {
		setEditingIndex(index);
		setEditingHost({ ...fileHosts[index] });
	}

	function handleCancelEdit() {
		setEditingIndex(null);
		setEditingHost(EMPTY_HOST);
	}

	function handleSaveEdit() {
		if (editingIndex === null) return;
		const host = normalize(editingHost);
		const problem = validate(host);
		if (problem) {
			toast.error(problem);
			return;
		}
		const next = [...fileHosts];
		next[editingIndex] = host;
		setFileHosts(next);
		setEditingIndex(null);
		setEditingHost(EMPTY_HOST);
		setTestResults({});
	}

	function handleAddHost() {
		const host = normalize(newHost);
		const problem = validate(host);
		if (problem) {
			toast.error(problem);
			return;
		}
		setFileHosts([...fileHosts, host]);
		setNewHost(EMPTY_HOST);
		setIsAdding(false);
		setTestResults({});
	}

	function handleTest(key: string, h: DokployHostInput) {
		setTestingKey(key);
		testMutation.mutate(
			{
				hostName: h.hostName,
				apiURL: h.apiURL,
				apiToken: h.apiToken,
				serverId: h.serverId,
			},
			{
				onSuccess: (result) => {
					setTestResults((prev) => ({
						...prev,
						[key]: { success: result.success, message: result.message },
					}));
					setTestingKey(null);
				},
				onError: (err) => {
					setTestResults((prev) => ({
						...prev,
						[key]: { success: false, message: err.message },
					}));
					setTestingKey(null);
				},
			},
		);
	}

	function renderRow(
		h: DokployHostInput,
		isEnvRow: boolean,
		fileIndex?: number,
	) {
		const testKey = `${isEnvRow ? "env" : "file"}-${h.hostName}`;
		const isEditing = !isEnvRow && editingIndex === fileIndex;

		if (isEditing && fileIndex !== undefined) {
			return (
				<tr key={testKey}>
					<Td>
						<HostSelect
							label="Docker host"
							value={editingHost.hostName}
							options={availableHosts(fileIndex)}
							onChange={(hostName) =>
								setEditingHost((prev) => ({ ...prev, hostName }))
							}
						/>
					</Td>
					<Td>
						<Input
							aria-label="API URL"
							value={editingHost.apiURL}
							onChange={(e) =>
								setEditingHost((prev) => ({ ...prev, apiURL: e.target.value }))
							}
							className="h-8"
							placeholder="https://dokploy.example.com"
						/>
					</Td>
					<Td>
						<Input
							aria-label="Server ID"
							value={editingHost.serverId}
							onChange={(e) =>
								setEditingHost((prev) => ({
									...prev,
									serverId: e.target.value,
								}))
							}
							className="h-8"
							placeholder={LOCAL_SERVER}
						/>
					</Td>
					<Td>
						<Input
							aria-label="API token"
							value={editingHost.apiToken}
							onChange={(e) =>
								setEditingHost((prev) => ({
									...prev,
									apiToken: e.target.value,
								}))
							}
							className="h-8"
							placeholder="Leave masked to keep existing"
							type="password"
						/>
					</Td>
					<Td />
					<Td className="text-right">
						<div className="flex items-center justify-end gap-1">
							<Button variant="ghost" size="sm" onClick={handleSaveEdit}>
								Done
							</Button>
							<Button variant="ghost" size="sm" onClick={handleCancelEdit}>
								Cancel
							</Button>
						</div>
					</Td>
				</tr>
			);
		}

		return (
			<tr key={testKey}>
				<Td className="font-medium">
					<div className="flex flex-wrap items-center gap-2">
						<TruncatedValue value={h.hostName} className="max-w-36" />
						{isEnvRow && <EnvBadge />}
					</div>
				</Td>
				<Td className="font-mono text-muted-foreground">
					<TruncatedValue value={h.apiURL} className="max-w-48" />
				</Td>
				<Td className="text-muted-foreground">
					{h.serverId ? (
						<TruncatedValue value={h.serverId} className="max-w-36 font-mono" />
					) : (
						LOCAL_SERVER
					)}
				</Td>
				<Td className="text-muted-foreground">{h.apiToken}</Td>
				<Td>
					{testResults[testKey] && (
						<Outcome ok={testResults[testKey].success}>
							{testResults[testKey].message}
						</Outcome>
					)}
				</Td>
				<Td className="text-right">
					<div className="flex items-center justify-end gap-1">
						<Button
							variant="ghost"
							size="sm"
							disabled={testingKey === testKey}
							onClick={() => handleTest(testKey, h)}
						>
							{testingKey === testKey ? <Spinner className="size-3" /> : "Test"}
						</Button>
						{!isEnvRow && fileIndex !== undefined && (
							<>
								<Button
									variant="ghost"
									size="sm"
									disabled={editingIndex !== null}
									onClick={() => handleStartEdit(fileIndex)}
								>
									Edit
								</Button>
								<Button
									variant="ghost"
									size="sm"
									disabled={editingIndex !== null}
									onClick={() => handleRemove(fileIndex)}
									className="text-destructive hover:text-destructive"
								>
									Remove
								</Button>
							</>
						)}
					</div>
				</Td>
			</tr>
		);
	}

	const allHosts = [
		...envHosts.map((h) => ({ ...h, isEnv: true as const })),
		...fileHosts.map((h, i) => ({
			...h,
			isEnv: false as const,
			fileIndex: i,
		})),
	];
	const canAdd = availableHosts().length > 0;

	return (
		<SettingsSection
			title="Dokploy hosts"
			description="Environment variable edits are written back to Dokploy so they survive a redeploy. Each Docker host connects to the Dokploy instance and server that deploys to it."
		>
			<div className="space-y-4">
				{allHosts.length > 0 && (
					<SettingsTable>
						<THead>
							<Th>Docker host</Th>
							<Th>API URL</Th>
							<Th>Server</Th>
							<Th>Token</Th>
							<Th>Status</Th>
							<Th className="text-right">
								<span className="sr-only">Actions</span>
							</Th>
						</THead>
						<TBody>
							{allHosts.map((h) =>
								renderRow(h, h.isEnv, h.isEnv ? undefined : h.fileIndex),
							)}
						</TBody>
					</SettingsTable>
				)}

				{allHosts.length === 0 && !isAdding && (
					<Note>No Dokploy hosts configured.</Note>
				)}

				{isAdding && (
					<Well>
						<div className="grid gap-3 sm:grid-cols-2">
							<Field id="new-dokploy-host" label="Docker host">
								<HostSelect
									id="new-dokploy-host"
									value={newHost.hostName}
									options={availableHosts()}
									onChange={(hostName) =>
										setNewHost((prev) => ({ ...prev, hostName }))
									}
								/>
							</Field>
							<Field
								id="new-dokploy-url"
								label="API URL"
								hint={
									newHost.apiURL.trim().startsWith("http://") &&
									"Over http:// the token is sent unencrypted. Use it only on a trusted network."
								}
							>
								<Input
									id="new-dokploy-url"
									name="apiURL"
									value={newHost.apiURL}
									onChange={(e) =>
										setNewHost((prev) => ({ ...prev, apiURL: e.target.value }))
									}
									placeholder="https://dokploy.example.com"
									className="h-8"
								/>
							</Field>
							<Field id="new-dokploy-token" label="API token">
								<Input
									id="new-dokploy-token"
									name="apiToken"
									value={newHost.apiToken}
									onChange={(e) =>
										setNewHost((prev) => ({
											...prev,
											apiToken: e.target.value,
										}))
									}
									placeholder="Token"
									type="password"
									autoComplete="off"
									className="h-8"
								/>
							</Field>
							<Field
								id="new-dokploy-server"
								label="Server ID"
								hint="Leave empty when this host runs the Dokploy instance itself."
							>
								<Input
									id="new-dokploy-server"
									name="serverId"
									value={newHost.serverId}
									onChange={(e) =>
										setNewHost((prev) => ({
											...prev,
											serverId: e.target.value,
										}))
									}
									placeholder={LOCAL_SERVER}
									className="h-8"
								/>
							</Field>
						</div>
						<div className="mt-3 flex gap-1">
							<Button size="sm" variant="outline" onClick={handleAddHost}>
								Add
							</Button>
							<Button
								size="sm"
								variant="ghost"
								onClick={() => {
									setIsAdding(false);
									setNewHost(EMPTY_HOST);
								}}
							>
								Cancel
							</Button>
						</div>
					</Well>
				)}

				{!isAdding && !canAdd && allHosts.length > 0 && (
					<Note>Every Docker host already has a Dokploy connection.</Note>
				)}

				<div className="flex items-center gap-2">
					{!isAdding && canAdd && (
						<Button
							variant="outline"
							size="sm"
							onClick={() => setIsAdding(true)}
						>
							Add host
						</Button>
					)}
					{hasChanges && (
						<SaveButton
							isPending={updateMutation.isPending}
							onClick={handleSave}
						/>
					)}
				</div>
			</div>
		</SettingsSection>
	);
}
