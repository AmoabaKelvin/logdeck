import { parseAsStringLiteral, useQueryState } from "nuqs";

import { Spinner } from "@/components/ui/spinner";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

import { useSettings } from "../hooks/use-settings";
import { AlertsSection } from "./alerts-section";
import { ApiTokensSection } from "./api-tokens-section";
import { AuthSection } from "./auth-section";
import { CoolifyHostsSection } from "./coolify-hosts-section";
import { DockerHostsSection } from "./docker-hosts-section";
import { LogStorageSection } from "./log-storage-section";
import { ReadOnlySection } from "./read-only-section";

const SETTINGS_TABS = ["connections", "access", "alerts", "storage"] as const;

const parseAsSettingsTab = parseAsStringLiteral(SETTINGS_TABS)
	.withDefault("connections")
	.withOptions({ history: "replace" });

export function SettingsPage() {
	const [tab, setTab] = useQueryState("tab", parseAsSettingsTab);
	const { data, isLoading, error } = useSettings();

	if (isLoading) {
		return (
			<div className="flex items-center justify-center py-20">
				<Spinner className="size-6" />
			</div>
		);
	}

	if (error) {
		return (
			<div className="container mx-auto max-w-3xl px-4 py-8">
				<p className="text-sm text-destructive">
					Failed to load settings: {error.message}
				</p>
			</div>
		);
	}

	if (!data) return null;

	return (
		<div className="container mx-auto max-w-3xl px-4 py-8 space-y-6">
			<div>
				<h1 className="text-2xl font-bold tracking-tight">Settings</h1>
				<p className="text-sm text-muted-foreground mt-1">
					Manage LogDeck configuration. Sections marked as set via environment
					variable can only be changed by updating the environment and
					restarting.
				</p>
			</div>

			<Tabs
				value={tab}
				onValueChange={(value) =>
					setTab(value as (typeof SETTINGS_TABS)[number])
				}
			>
				<TabsList className="w-full sm:w-fit">
					<TabsTrigger value="connections">Connections</TabsTrigger>
					<TabsTrigger value="access">Access</TabsTrigger>
					<TabsTrigger value="alerts">Alerts</TabsTrigger>
					<TabsTrigger value="storage">Log storage</TabsTrigger>
				</TabsList>

				<TabsContent value="connections" className="space-y-6 pt-2">
					<DockerHostsSection
						key={JSON.stringify(data.dockerHosts)}
						config={data.dockerHosts}
					/>
					<CoolifyHostsSection
						key={JSON.stringify(data.coolifyHosts)}
						config={data.coolifyHosts}
					/>
				</TabsContent>

				<TabsContent value="access" className="space-y-6 pt-2">
					<AuthSection
						key={`${data.auth.enabled}-${data.auth.adminUsername}`}
						config={data.auth}
					/>
					<ReadOnlySection config={data.readOnly} />
					<ApiTokensSection />
				</TabsContent>

				<TabsContent value="alerts" className="pt-2">
					<AlertsSection />
				</TabsContent>

				<TabsContent value="storage" className="pt-2">
					<LogStorageSection />
				</TabsContent>
			</Tabs>
		</div>
	);
}
