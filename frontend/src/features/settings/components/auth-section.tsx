import { useState } from "react";
import { toast } from "sonner";

import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";

import type { UpdateAuthPayload } from "../api/update-auth";
import { useUpdateAuth } from "../hooks/use-settings";
import type { AuthConfig } from "../types";
import { SaveButton } from "./save-button";
import { EnvBadge, Field, SettingsSection } from "./settings-ui";

interface AuthSectionProps {
	config: AuthConfig;
}

export function AuthSection({ config }: AuthSectionProps) {
	const isEnv = config.source === "env";
	const [enabled, setEnabled] = useState(config.enabled);
	const [username, setUsername] = useState(config.adminUsername ?? "");
	const [password, setPassword] = useState("");

	const mutation = useUpdateAuth();

	const hasChanges =
		enabled !== config.enabled ||
		username !== (config.adminUsername ?? "") ||
		password.length > 0;

	function handleSave() {
		if (enabled && !username.trim()) {
			toast.error("Username is required when enabling auth");
			return;
		}
		if (enabled && !config.enabled && !password) {
			toast.error("Password is required when first enabling auth");
			return;
		}

		const payload: UpdateAuthPayload = { enabled, adminUsername: username };
		if (password) payload.newPassword = password;

		mutation.mutate(payload, {
			onSuccess: (msg) => {
				toast.success(msg);
				setPassword("");
			},
			onError: (err) => toast.error(err.message),
		});
	}

	return (
		<SettingsSection
			title="Authentication"
			description="Protect the dashboard with a username and password."
			badge={isEnv && <EnvBadge />}
			action={
				<Switch
					id="auth-enabled"
					aria-label="Enable authentication"
					checked={enabled}
					onCheckedChange={(checked) => {
						setEnabled(checked);
						if (!checked) setPassword("");
					}}
					disabled={isEnv}
					className="relative after:absolute after:-inset-x-1 after:-inset-y-3"
				/>
			}
		>
			{(enabled || hasChanges) && (
				<div className="space-y-4">
					{enabled && (
						<div className="max-w-xs space-y-4">
							<Field id="admin-username" label="Admin username">
								<Input
									id="admin-username"
									name="adminUsername"
									value={username}
									onChange={(e) => setUsername(e.target.value)}
									disabled={isEnv}
									placeholder="admin"
								/>
							</Field>
							<Field
								id="admin-password"
								label={config.enabled ? "New password" : "Password"}
								hint={
									config.enabled ? "Leave blank to keep the current one." : null
								}
							>
								<Input
									id="admin-password"
									name="newPassword"
									type="password"
									autoComplete="new-password"
									value={password}
									onChange={(e) => setPassword(e.target.value)}
									disabled={isEnv}
								/>
							</Field>
						</div>
					)}

					{hasChanges && !isEnv && (
						<SaveButton isPending={mutation.isPending} onClick={handleSave} />
					)}
				</div>
			)}
		</SettingsSection>
	);
}
