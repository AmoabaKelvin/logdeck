import { useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";

import { useTestDockerHost, useUpdateDockerHosts } from "../hooks/use-settings";
import type { DockerHostsConfig } from "../types";
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

interface DockerHostsSectionProps {
	config: DockerHostsConfig;
}

interface EditingHost {
	name: string;
	host: string;
}

const EMPTY_HOST: EditingHost = { name: "", host: "" };

export function DockerHostsSection({ config }: DockerHostsSectionProps) {
	const envHosts = config.hosts.filter((h) => h.source === "env");
	const originalFileHosts = config.hosts
		.filter((h) => h.source !== "env")
		.map(({ name, host }) => ({ name, host }));

	const [fileHosts, setFileHosts] = useState<EditingHost[]>(originalFileHosts);
	const [editingIndex, setEditingIndex] = useState<number | null>(null);
	const [editingHost, setEditingHost] = useState<EditingHost>(EMPTY_HOST);
	const [isAdding, setIsAdding] = useState(false);
	const [newHost, setNewHost] = useState<EditingHost>(EMPTY_HOST);
	const [testResults, setTestResults] = useState<
		Record<string, { success: boolean; message: string }>
	>({});
	const [testingKey, setTestingKey] = useState<string | null>(null);

	const updateMutation = useUpdateDockerHosts();
	const testMutation = useTestDockerHost();

	const hasChanges =
		fileHosts.length !== originalFileHosts.length ||
		fileHosts.some(
			(h, i) =>
				h.name !== originalFileHosts[i]?.name ||
				h.host !== originalFileHosts[i]?.host,
		);

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
		if (!editingHost.name.trim() || !editingHost.host.trim()) {
			toast.error("Name and host are required");
			return;
		}
		const trimmedName = editingHost.name.trim();
		if (envHosts.some((h) => h.name === trimmedName)) {
			toast.error(
				`Host name "${trimmedName}" is defined via environment variable`,
			);
			return;
		}
		if (
			fileHosts.some((h, i) => i !== editingIndex && h.name === trimmedName)
		) {
			toast.error(`Host name "${trimmedName}" already exists`);
			return;
		}
		const next = [...fileHosts];
		next[editingIndex] = { ...editingHost };
		setFileHosts(next);
		setEditingIndex(null);
		setEditingHost(EMPTY_HOST);
		setTestResults({});
	}

	function handleAddHost() {
		if (!newHost.name.trim() || !newHost.host.trim()) {
			toast.error("Name and host are required");
			return;
		}
		const trimmedName = newHost.name.trim();
		if (envHosts.some((h) => h.name === trimmedName)) {
			toast.error(
				`Host name "${trimmedName}" is already defined via environment variable`,
			);
			return;
		}
		if (fileHosts.some((h) => h.name === trimmedName)) {
			toast.error(`Host name "${trimmedName}" already exists`);
			return;
		}
		setFileHosts([
			...fileHosts,
			{ name: trimmedName, host: newHost.host.trim() },
		]);
		setNewHost(EMPTY_HOST);
		setIsAdding(false);
		setTestResults({});
	}

	function handleTest(key: string, name: string, hostUrl: string) {
		setTestingKey(key);
		setTestResults((prev) => {
			const next = { ...prev };
			delete next[key];
			return next;
		});
		testMutation.mutate(
			{ name, host: hostUrl },
			{
				onSuccess: (result) => {
					const engineLabel =
						result.engine && result.engineVersion
							? `${result.engine} ${result.engineVersion}`
							: `Docker ${result.dockerVersion ?? ""}`;
					setTestResults((prev) => ({
						...prev,
						[key]: {
							success: result.success,
							message: result.success
								? `Connected (${engineLabel})`
								: result.message,
						},
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
		h: { name: string; host: string },
		isEnvRow: boolean,
		fileIndex?: number,
	) {
		const testKey = `${isEnvRow ? "env" : "file"}-${h.name}`;
		const isEditing = !isEnvRow && editingIndex === fileIndex;

		if (isEditing && fileIndex !== undefined) {
			return (
				<tr key={testKey}>
					<Td>
						<Input
							aria-label="Host name"
							value={editingHost.name}
							onChange={(e) =>
								setEditingHost((prev) => ({ ...prev, name: e.target.value }))
							}
							className="h-8"
						/>
					</Td>
					<Td>
						<Input
							aria-label="Host address"
							value={editingHost.host}
							onChange={(e) =>
								setEditingHost((prev) => ({ ...prev, host: e.target.value }))
							}
							className="h-8"
							placeholder="unix:///var/run/docker.sock"
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
						<TruncatedValue value={h.name} className="max-w-40" />
						{isEnvRow && <EnvBadge />}
					</div>
				</Td>
				<Td className="font-mono text-muted-foreground">
					<TruncatedValue value={h.host} className="max-w-64" />
				</Td>
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
							onClick={() => handleTest(testKey, h.name, h.host)}
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
			source: "file" as const,
			isEnv: false as const,
			fileIndex: i,
		})),
	];

	return (
		<SettingsSection
			title="Docker hosts"
			description="The Docker or Podman sockets and remote engines LogDeck reads from."
		>
			<div className="space-y-4">
				{allHosts.length > 0 && (
					<SettingsTable>
						<THead>
							<Th>Name</Th>
							<Th>Host</Th>
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
					<Note>No Docker hosts configured.</Note>
				)}

				{isAdding && (
					<Well>
						<div className="grid gap-3 sm:grid-cols-[1fr_2fr]">
							<Field id="new-docker-name" label="Name">
								<Input
									id="new-docker-name"
									name="name"
									value={newHost.name}
									onChange={(e) =>
										setNewHost((prev) => ({ ...prev, name: e.target.value }))
									}
									placeholder="my-server"
									className="h-8"
								/>
							</Field>
							<Field id="new-docker-host" label="Host">
								<Input
									id="new-docker-host"
									name="host"
									value={newHost.host}
									onChange={(e) =>
										setNewHost((prev) => ({ ...prev, host: e.target.value }))
									}
									placeholder="ssh://root@10.0.0.1"
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

				<div className="flex items-center gap-2">
					{!isAdding && (
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
