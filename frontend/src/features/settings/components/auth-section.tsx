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
	const [username, setUsername] = useState("");
	const [password, setPassword] = useState("");

	const mutation = useUpdateAuth();

	// Turning auth on is when the first admin is set up. Once it is on, users
	// are managed in the list below.
	const turningOn = enabled && !config.enabled;
	const hasChanges = enabled !== config.enabled;

	function handleSave() {
		if (turningOn && password && !username.trim()) {
			toast.error("Username is required");
			return;
		}

		const payload: UpdateAuthPayload = { enabled };
		if (turningOn && password) {
			payload.adminUsername = username.trim();
			payload.newPassword = password;
		}

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
			description="Require signing in to use the dashboard."
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
			{hasChanges && !isEnv && (
				<div className="space-y-4">
					{turningOn && (
						<div className="max-w-xs space-y-4">
							<Field
								id="admin-username"
								label="Admin username"
								hint="Leave both blank to keep the admins already in the list."
							>
								<Input
									id="admin-username"
									name="adminUsername"
									value={username}
									onChange={(e) => setUsername(e.target.value)}
									placeholder="admin"
								/>
							</Field>
							<Field id="admin-password" label="Password">
								<Input
									id="admin-password"
									name="newPassword"
									type="password"
									autoComplete="new-password"
									value={password}
									onChange={(e) => setPassword(e.target.value)}
								/>
							</Field>
						</div>
					)}

					<SaveButton isPending={mutation.isPending} onClick={handleSave} />
				</div>
			)}
		</SettingsSection>
	);
}
