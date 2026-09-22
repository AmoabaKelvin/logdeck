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
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { CheckIcon, CopyIcon } from "@/components/ui/icons";
import { Input } from "@/components/ui/input";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";

import {
	useApiTokens,
	useCreateApiToken,
	useDeleteApiToken,
} from "../hooks/use-settings";
import type { APIToken, APITokenScope, CreatedAPIToken } from "../types";
import { showResultToast } from "./mutation-toast";
import {
	ErrorNote,
	Field,
	Note,
	SettingsSection,
	SettingsTable,
	TBody,
	Td,
	Th,
	THead,
	Well,
} from "./settings-ui";

export function ApiTokensSection() {
	const { data, isLoading, error } = useApiTokens();
	const createMutation = useCreateApiToken();
	const deleteMutation = useDeleteApiToken();

	const [isCreating, setIsCreating] = useState(false);
	const [newName, setNewName] = useState("");
	const [newScope, setNewScope] = useState<APITokenScope>("admin");
	const [createdToken, setCreatedToken] = useState<CreatedAPIToken | null>(
		null,
	);
	const [copied, setCopied] = useState(false);
	const [tokenToRevoke, setTokenToRevoke] = useState<APIToken | null>(null);

	const tokens = data?.tokens ?? [];

	function handleCreate() {
		const name = newName.trim();
		if (!name) {
			toast.error("Token name is required");
			return;
		}
		createMutation.mutate(
			{ name, scope: newScope },
			{
				onSuccess: (token) => {
					toast.success(`Token "${token.name}" created`);
					setCreatedToken(token);
					setCopied(false);
					setNewName("");
					setNewScope("admin");
					setIsCreating(false);
				},
				onError: (err) => toast.error(err.message),
			},
		);
	}

	function handleCopy() {
		if (!createdToken) return;
		navigator.clipboard
			.writeText(createdToken.token)
			.then(() => {
				setCopied(true);
				toast.success("Token copied to clipboard");
			})
			.catch(() => toast.error("Failed to copy token"));
	}

	function handleRevoke() {
		if (!tokenToRevoke) return;
		deleteMutation.mutate(tokenToRevoke.prefix, showResultToast);
		setTokenToRevoke(null);
	}

	return (
		<SettingsSection
			title="API access"
			description={
				<>
					Long-lived tokens for the LogDeck CLI and other tools. Send one as{" "}
					<code className="font-mono">Authorization: Bearer &lt;token&gt;</code>
					.
				</>
			}
		>
			<div className="space-y-4">
				{createdToken && (
					<Well>
						<p className="text-base font-medium sm:text-sm">
							Token "{createdToken.name}" created
						</p>
						<p className="mt-1 text-base/6 text-amber-700 sm:text-sm/6 dark:text-amber-400">
							Copy it now. It will not be shown again.
						</p>
						<div className="mt-3 flex items-center gap-2">
							<code className="min-w-0 flex-1 break-all rounded-md bg-background px-2.5 py-1.5 font-mono text-sm">
								{createdToken.token}
							</code>
							<Button
								variant="outline"
								size="sm"
								onClick={handleCopy}
								className="shrink-0"
							>
								{copied ? (
									<CheckIcon className="size-4" />
								) : (
									<CopyIcon className="size-4" />
								)}
								{copied ? "Copied" : "Copy"}
							</Button>
						</div>
						<Button
							variant="ghost"
							size="sm"
							onClick={() => setCreatedToken(null)}
							className="mt-3"
						>
							Done
						</Button>
					</Well>
				)}

				{isLoading && <Spinner className="size-4" />}
				{error && (
					<ErrorNote>Failed to load API tokens: {error.message}</ErrorNote>
				)}

				{!isLoading && !error && tokens.length === 0 && (
					<Note>No API tokens created yet.</Note>
				)}

				{tokens.length > 0 && (
					<SettingsTable>
						<THead>
							<Th>Name</Th>
							<Th>Token</Th>
							<Th>Scope</Th>
							<Th>Created</Th>
							<Th className="text-right">
								<span className="sr-only">Actions</span>
							</Th>
						</THead>
						<TBody>
							{tokens.map((t) => (
								<tr key={t.prefix}>
									<Td className="font-medium">{t.name}</Td>
									<Td className="font-mono text-muted-foreground">
										{t.prefix}…
									</Td>
									<Td>
										<Badge className="border-transparent bg-muted font-normal text-muted-foreground">
											{t.scope === "read" ? "Read-only" : "Admin"}
										</Badge>
									</Td>
									<Td className="text-muted-foreground">
										{new Date(t.createdAt).toLocaleDateString()}
									</Td>
									<Td className="text-right">
										<Button
											variant="ghost"
											size="sm"
											disabled={deleteMutation.isPending}
											onClick={() => setTokenToRevoke(t)}
											className="text-destructive hover:text-destructive"
										>
											Revoke
										</Button>
									</Td>
								</tr>
							))}
						</TBody>
					</SettingsTable>
				)}

				{isCreating ? (
					<Well>
						<div className="grid gap-3 sm:grid-cols-[1fr_auto]">
							<Field id="new-token-name" label="Name">
								<Input
									id="new-token-name"
									name="name"
									value={newName}
									onChange={(e) => setNewName(e.target.value)}
									onKeyDown={(e) => {
										if (e.key === "Enter") handleCreate();
									}}
									placeholder="my-cli"
									className="h-8"
									maxLength={64}
								/>
							</Field>
							<Field id="new-token-scope" label="Scope">
								<Select
									value={newScope}
									onValueChange={(value) => {
										if (value === "admin" || value === "read")
											setNewScope(value);
									}}
								>
									<SelectTrigger
										id="new-token-scope"
										className="h-8 w-full sm:w-32"
									>
										<SelectValue />
									</SelectTrigger>
									<SelectContent>
										<SelectItem value="admin">Admin</SelectItem>
										<SelectItem value="read">Read-only</SelectItem>
									</SelectContent>
								</Select>
							</Field>
						</div>
						<div className="mt-3 flex gap-1">
							<Button
								size="sm"
								variant="outline"
								disabled={createMutation.isPending}
								onClick={handleCreate}
							>
								{createMutation.isPending ? (
									<>
										<Spinner className="size-3" />
										Creating...
									</>
								) : (
									"Create"
								)}
							</Button>
							<Button
								size="sm"
								variant="ghost"
								onClick={() => {
									setIsCreating(false);
									setNewName("");
									setNewScope("admin");
								}}
							>
								Cancel
							</Button>
						</div>
					</Well>
				) : (
					<Button
						variant="outline"
						size="sm"
						onClick={() => setIsCreating(true)}
					>
						Create token
					</Button>
				)}
			</div>

			<AlertDialog
				open={tokenToRevoke !== null}
				onOpenChange={(open) => {
					if (!open) setTokenToRevoke(null);
				}}
			>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Revoke API token?</AlertDialogTitle>
						<AlertDialogDescription>
							The token "{tokenToRevoke?.name}" will stop working immediately.
							Any CLI or tool using it will lose access. This cannot be undone.
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>Cancel</AlertDialogCancel>
						<AlertDialogAction
							onClick={handleRevoke}
							className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
						>
							Revoke
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</SettingsSection>
	);
}
