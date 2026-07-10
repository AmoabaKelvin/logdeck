import { Check, Copy } from "lucide-react";
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
import { Spinner } from "@/components/ui/spinner";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";

import {
	useApiTokens,
	useCreateApiToken,
	useDeleteApiToken,
} from "../hooks/use-settings";
import type { APIToken, CreatedAPIToken } from "../types";
import { showResultToast } from "./mutation-toast";

export function ApiTokensSection() {
	const { data, isLoading, error } = useApiTokens();
	const createMutation = useCreateApiToken();
	const deleteMutation = useDeleteApiToken();

	const [isCreating, setIsCreating] = useState(false);
	const [newName, setNewName] = useState("");
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
		createMutation.mutate(name, {
			onSuccess: (token) => {
				toast.success(`Token "${token.name}" created`);
				setCreatedToken(token);
				setCopied(false);
				setNewName("");
				setIsCreating(false);
			},
			onError: (err) => toast.error(err.message),
		});
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
		<Card>
			<CardHeader>
				<CardTitle>API Access</CardTitle>
				<CardDescription>
					Generate long-lived tokens for the LogDeck CLI and other external
					tools. Send a token as{" "}
					<code className="font-mono text-xs">
						Authorization: Bearer &lt;token&gt;
					</code>{" "}
					to authenticate API requests.
				</CardDescription>
			</CardHeader>
			<CardContent className="space-y-4">
				{createdToken && (
					<div className="space-y-2 border rounded-md p-3 bg-muted/50">
						<p className="text-sm font-medium">
							Token "{createdToken.name}" created
						</p>
						<div className="flex items-center gap-2">
							<code className="flex-1 font-mono text-xs break-all rounded bg-background border px-2 py-1.5">
								{createdToken.token}
							</code>
							<Button variant="outline" size="sm" onClick={handleCopy}>
								{copied ? (
									<Check className="size-3.5" />
								) : (
									<Copy className="size-3.5" />
								)}
								{copied ? "Copied" : "Copy"}
							</Button>
						</div>
						<p className="text-xs text-amber-600 dark:text-amber-400">
							Copy this token now. It will not be shown again.
						</p>
						<Button
							variant="ghost"
							size="sm"
							onClick={() => setCreatedToken(null)}
						>
							Done
						</Button>
					</div>
				)}

				{isLoading && <Spinner className="size-4" />}
				{error && (
					<p className="text-sm text-destructive">
						Failed to load API tokens: {error.message}
					</p>
				)}

				{!isLoading && !error && tokens.length === 0 && (
					<p className="text-sm text-muted-foreground">
						No API tokens created yet.
					</p>
				)}

				{tokens.length > 0 && (
					<Table>
						<TableHeader>
							<TableRow>
								<TableHead>Name</TableHead>
								<TableHead>Token</TableHead>
								<TableHead>Created</TableHead>
								<TableHead className="text-right">Actions</TableHead>
							</TableRow>
						</TableHeader>
						<TableBody>
							{tokens.map((t) => (
								<TableRow key={t.prefix}>
									<TableCell className="font-medium">{t.name}</TableCell>
									<TableCell className="font-mono text-xs text-muted-foreground">
										{t.prefix}…
									</TableCell>
									<TableCell className="text-xs text-muted-foreground">
										{new Date(t.createdAt).toLocaleDateString()}
									</TableCell>
									<TableCell className="text-right">
										<Button
											variant="ghost"
											size="sm"
											disabled={deleteMutation.isPending}
											onClick={() => setTokenToRevoke(t)}
											className="text-destructive hover:text-destructive"
										>
											Revoke
										</Button>
									</TableCell>
								</TableRow>
							))}
						</TableBody>
					</Table>
				)}

				{isCreating ? (
					<div className="flex items-end gap-3 border rounded-md p-3">
						<div className="space-y-1.5 flex-1">
							<Label htmlFor="new-token-name">Name</Label>
							<Input
								id="new-token-name"
								value={newName}
								onChange={(e) => setNewName(e.target.value)}
								onKeyDown={(e) => {
									if (e.key === "Enter") handleCreate();
								}}
								placeholder="my-cli"
								className="h-8"
								maxLength={64}
							/>
						</div>
						<div className="flex gap-1">
							<Button
								size="sm"
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
								}}
							>
								Cancel
							</Button>
						</div>
					</div>
				) : (
					<Button
						variant="outline"
						size="sm"
						onClick={() => setIsCreating(true)}
					>
						Create token
					</Button>
				)}
			</CardContent>

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
		</Card>
	);
}
