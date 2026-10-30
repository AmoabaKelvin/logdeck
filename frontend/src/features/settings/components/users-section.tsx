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
	useCreateUser,
	useDeleteUser,
	useUpdateUser,
	useUsers,
} from "../hooks/use-settings";
import type { ManagedUser, UserRole } from "../types";
import { showResultToast } from "./mutation-toast";
import {
	EnvBadge,
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

function RoleSelect({
	id,
	value,
	onChange,
	disabled,
}: {
	id: string;
	value: UserRole;
	onChange: (role: UserRole) => void;
	disabled?: boolean;
}) {
	return (
		<Select
			value={value}
			onValueChange={(v) => {
				if (v === "admin" || v === "read") onChange(v);
			}}
			disabled={disabled}
		>
			<SelectTrigger id={id} className="h-8 w-full sm:w-32">
				<SelectValue />
			</SelectTrigger>
			<SelectContent>
				<SelectItem value="admin">Admin</SelectItem>
				<SelectItem value="read">Read-only</SelectItem>
			</SelectContent>
		</Select>
	);
}

export function UsersSection() {
	const { data, isLoading, error } = useUsers();
	const createMutation = useCreateUser();
	const updateMutation = useUpdateUser();
	const deleteMutation = useDeleteUser();

	const [isCreating, setIsCreating] = useState(false);
	const [newUsername, setNewUsername] = useState("");
	const [newPassword, setNewPassword] = useState("");
	const [newRole, setNewRole] = useState<UserRole>("read");
	const [resetting, setResetting] = useState<ManagedUser | null>(null);
	const [resetPassword, setResetPassword] = useState("");
	const [userToDelete, setUserToDelete] = useState<ManagedUser | null>(null);

	const users = data?.users ?? [];

	function closeCreate() {
		setIsCreating(false);
		setNewUsername("");
		setNewPassword("");
		setNewRole("read");
	}

	function handleCreate() {
		const username = newUsername.trim();
		if (!username || !newPassword) {
			toast.error("Username and password are required");
			return;
		}
		createMutation.mutate(
			{ username, password: newPassword, role: newRole },
			{
				onSuccess: (msg) => {
					toast.success(msg);
					closeCreate();
				},
				onError: (err) => toast.error(err.message),
			},
		);
	}

	function handleReset() {
		if (!resetting || !resetPassword) return;
		updateMutation.mutate(
			{ username: resetting.username, password: resetPassword },
			{
				onSuccess: (msg) => {
					toast.success(msg);
					setResetting(null);
					setResetPassword("");
				},
				onError: (err) => toast.error(err.message),
			},
		);
	}

	function handleDelete() {
		if (!userToDelete) return;
		deleteMutation.mutate(userToDelete.username, showResultToast);
		setUserToDelete(null);
	}

	return (
		<SettingsSection
			title="Users"
			description="People who can sign in. Read-only users can view containers and logs but cannot change anything, open a shell, see environment variables, or open Settings."
		>
			<div className="space-y-4">
				{isLoading && <Spinner className="size-4" />}
				{error && <ErrorNote>Failed to load users: {error.message}</ErrorNote>}

				{!isLoading && !error && users.length === 0 && (
					<Note>
						No users yet. Turning on authentication creates the first admin.
					</Note>
				)}

				{users.length > 0 && (
					<SettingsTable>
						<THead>
							<Th>Username</Th>
							<Th>Role</Th>
							<Th>Created</Th>
							<Th className="text-right">
								<span className="sr-only">Actions</span>
							</Th>
						</THead>
						<TBody>
							{users.map((u) => {
								const isEnv = u.source === "env";
								return (
									<tr key={u.username}>
										<Td className="font-medium">
											<span className="mr-2">{u.username}</span>
											{isEnv && <EnvBadge />}
										</Td>
										<Td>
											<RoleSelect
												id={`role-${u.username}`}
												value={u.role}
												disabled={isEnv || updateMutation.isPending}
												onChange={(role) =>
													updateMutation.mutate(
														{ username: u.username, role },
														showResultToast,
													)
												}
											/>
										</Td>
										<Td className="text-muted-foreground">
											{u.createdAt
												? new Date(u.createdAt).toLocaleDateString()
												: "—"}
										</Td>
										<Td className="text-right">
											{!isEnv && (
												<>
													<Button
														variant="ghost"
														size="sm"
														onClick={() => {
															setResetting(u);
															setResetPassword("");
														}}
													>
														Reset password
													</Button>
													<Button
														variant="ghost"
														size="sm"
														disabled={deleteMutation.isPending}
														onClick={() => setUserToDelete(u)}
														className="text-destructive hover:text-destructive"
													>
														Delete
													</Button>
												</>
											)}
										</Td>
									</tr>
								);
							})}
						</TBody>
					</SettingsTable>
				)}

				{resetting && (
					<Well>
						<div className="max-w-xs">
							<Field
								id="reset-password"
								label={`New password for ${resetting.username}`}
								hint="Their current sessions end."
							>
								<Input
									id="reset-password"
									type="password"
									autoComplete="new-password"
									value={resetPassword}
									onChange={(e) => setResetPassword(e.target.value)}
									onKeyDown={(e) => {
										if (e.key === "Enter") handleReset();
									}}
									className="h-8"
								/>
							</Field>
						</div>
						<div className="mt-3 flex gap-1">
							<Button
								size="sm"
								variant="outline"
								disabled={updateMutation.isPending || !resetPassword}
								onClick={handleReset}
							>
								Set password
							</Button>
							<Button
								size="sm"
								variant="ghost"
								onClick={() => setResetting(null)}
							>
								Cancel
							</Button>
						</div>
					</Well>
				)}

				{isCreating ? (
					<Well>
						<div className="grid gap-3 sm:grid-cols-[1fr_1fr_auto]">
							<Field id="new-user-name" label="Username">
								<Input
									id="new-user-name"
									name="username"
									autoComplete="off"
									value={newUsername}
									onChange={(e) => setNewUsername(e.target.value)}
									className="h-8"
									maxLength={64}
								/>
							</Field>
							<Field id="new-user-password" label="Password">
								<Input
									id="new-user-password"
									name="password"
									type="password"
									autoComplete="new-password"
									value={newPassword}
									onChange={(e) => setNewPassword(e.target.value)}
									onKeyDown={(e) => {
										if (e.key === "Enter") handleCreate();
									}}
									className="h-8"
								/>
							</Field>
							<Field id="new-user-role" label="Role">
								<RoleSelect
									id="new-user-role"
									value={newRole}
									onChange={setNewRole}
								/>
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
										Adding...
									</>
								) : (
									"Add user"
								)}
							</Button>
							<Button size="sm" variant="ghost" onClick={closeCreate}>
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
						Add user
					</Button>
				)}
			</div>

			<AlertDialog
				open={userToDelete !== null}
				onOpenChange={(open) => {
					if (!open) setUserToDelete(null);
				}}
			>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Delete user?</AlertDialogTitle>
						<AlertDialogDescription>
							"{userToDelete?.username}" will be signed out on their next
							request and can't sign in again. This cannot be undone.
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
		</SettingsSection>
	);
}
