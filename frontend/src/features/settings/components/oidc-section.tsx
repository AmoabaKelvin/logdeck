import { useState } from "react";
import { toast } from "sonner";

import { Input } from "@/components/ui/input";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { API_BASE_URL } from "@/types/api";

import type { UpdateOIDCPayload } from "../api/update-oidc";
import { useUpdateOIDC } from "../hooks/use-settings";
import type { OIDCConfig } from "../types";
import { SaveButton } from "./save-button";
import {
	EnvBadge,
	Field,
	SettingsSection,
	SettingsSubsection,
} from "./settings-ui";

const splitGroups = (value: string) =>
	value
		.split(",")
		.map((g) => g.trim())
		.filter(Boolean);

// The callback LogDeck serves; the provider must have it registered.
const defaultRedirectUrl = () =>
	`${API_BASE_URL || window.location.origin}/api/v1/auth/oidc/callback`;

export function OidcSection({
	config,
	authEnabled,
}: {
	config: OIDCConfig;
	authEnabled: boolean;
}) {
	const isEnv = config.source === "env";
	const initial: UpdateOIDCPayload = {
		enabled: config.enabled,
		issuer: config.issuer ?? "",
		clientId: config.clientId ?? "",
		clientSecret: config.clientSecret ?? "",
		redirectUrl: config.redirectUrl || defaultRedirectUrl(),
		groupsClaim: config.groupsClaim ?? "",
		adminGroups: config.adminGroups ?? [],
		readGroups: config.readGroups ?? [],
		defaultRole: config.defaultRole ?? "",
	};
	const [form, setForm] = useState(initial);
	const [adminGroups, setAdminGroups] = useState(
		initial.adminGroups?.join(", ") ?? "",
	);
	const [readGroups, setReadGroups] = useState(
		initial.readGroups?.join(", ") ?? "",
	);

	const mutation = useUpdateOIDC();

	const payload: UpdateOIDCPayload = {
		...form,
		adminGroups: splitGroups(adminGroups),
		readGroups: splitGroups(readGroups),
	};
	const hasChanges = JSON.stringify(payload) !== JSON.stringify(initial);
	const set = (patch: Partial<UpdateOIDCPayload>) =>
		setForm((f) => ({ ...f, ...patch }));

	function handleSave() {
		mutation.mutate(payload, {
			onSuccess: (msg) => toast.success(msg),
			onError: (err) => toast.error(err.message),
		});
	}

	return (
		<SettingsSection
			title="Single sign-on (OIDC)"
			description={
				<>
					Let people sign in through an OpenID Connect provider such as
					Authelia, Authentik, Keycloak or Google. Their role comes from the
					groups the provider puts in the ID token.
					{!authEnabled && " It takes effect once authentication is on."}
				</>
			}
			badge={isEnv && <EnvBadge />}
			action={
				<Switch
					id="oidc-enabled"
					aria-label="Enable OIDC sign-in"
					checked={form.enabled}
					onCheckedChange={(checked) => set({ enabled: checked })}
					disabled={isEnv}
					className="relative after:absolute after:-inset-x-1 after:-inset-y-3"
				/>
			}
		>
			{(form.enabled || hasChanges) && (
				<div className="space-y-4">
					{form.enabled && (
						<div className="max-w-2xl space-y-6">
							<SettingsSubsection title="Provider">
								<div className="grid gap-4 sm:grid-cols-2">
									<div className="sm:col-span-2">
										<Field id="oidc-issuer" label="Issuer URL">
											<Input
												id="oidc-issuer"
												value={form.issuer}
												onChange={(e) => set({ issuer: e.target.value })}
												placeholder="https://auth.example.com"
												disabled={isEnv}
											/>
										</Field>
									</div>
									<div className="sm:col-span-2">
										<Field
											id="oidc-redirect"
											label="Redirect URL"
											hint="Register this URL with the provider."
										>
											<Input
												id="oidc-redirect"
												value={form.redirectUrl}
												onChange={(e) => set({ redirectUrl: e.target.value })}
												disabled={isEnv}
											/>
										</Field>
									</div>
									<Field id="oidc-client-id" label="Client ID">
										<Input
											id="oidc-client-id"
											value={form.clientId}
											onChange={(e) => set({ clientId: e.target.value })}
											disabled={isEnv}
										/>
									</Field>
									<Field
										id="oidc-client-secret"
										label="Client secret"
										hint="Leave blank for a public client."
									>
										<Input
											id="oidc-client-secret"
											type="password"
											autoComplete="off"
											value={form.clientSecret}
											onChange={(e) => set({ clientSecret: e.target.value })}
											disabled={isEnv}
										/>
									</Field>
								</div>
							</SettingsSubsection>

							<div className="border-t border-border/60 pt-6">
								<SettingsSubsection title="Roles">
									<div className="grid gap-4 sm:grid-cols-2">
										<Field
											id="oidc-admin-groups"
											label="Admin groups"
											hint="Comma-separated. Full access."
										>
											<Input
												id="oidc-admin-groups"
												value={adminGroups}
												onChange={(e) => setAdminGroups(e.target.value)}
												placeholder="logdeck-admins"
												disabled={isEnv}
											/>
										</Field>
										<Field
											id="oidc-read-groups"
											label="Read-only groups"
											hint="Comma-separated. Can look, not change."
										>
											<Input
												id="oidc-read-groups"
												value={readGroups}
												onChange={(e) => setReadGroups(e.target.value)}
												placeholder="developers"
												disabled={isEnv}
											/>
										</Field>
										<Field
											id="oidc-default-role"
											label="Everyone else"
											hint="Users in none of these groups."
										>
											<Select
												value={form.defaultRole || "none"}
												onValueChange={(v) =>
													set({
														defaultRole: v === "admin" || v === "read" ? v : "",
													})
												}
												disabled={isEnv}
											>
												<SelectTrigger
													id="oidc-default-role"
													className="w-full"
												>
													<SelectValue />
												</SelectTrigger>
												<SelectContent>
													<SelectItem value="none">Can't sign in</SelectItem>
													<SelectItem value="read">Read-only</SelectItem>
													<SelectItem value="admin">Admin</SelectItem>
												</SelectContent>
											</Select>
										</Field>
										<Field
											id="oidc-groups-claim"
											label="Groups claim"
											hint="ID token claim listing the groups."
										>
											<Input
												id="oidc-groups-claim"
												value={form.groupsClaim}
												onChange={(e) => set({ groupsClaim: e.target.value })}
												placeholder="groups"
												disabled={isEnv}
											/>
										</Field>
									</div>
								</SettingsSubsection>
							</div>
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
