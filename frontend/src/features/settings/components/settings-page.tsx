import { parseAsStringLiteral, useQueryState } from "nuqs";

import { Spinner } from "@/components/ui/spinner";

import { useSettings } from "../hooks/use-settings";
import { AlertsSection } from "./alerts-section";
import { ApiTokensSection } from "./api-tokens-section";
import { AuthSection } from "./auth-section";
import { CoolifyHostsSection } from "./coolify-hosts-section";
import { DockerHostsSection } from "./docker-hosts-section";
import { LogStorageSection } from "./log-storage-section";
import { ReadOnlySection } from "./read-only-section";
import { ErrorNote } from "./settings-ui";

const SETTINGS_TABS = ["connections", "access", "alerts", "storage"] as const;
type SettingsTab = (typeof SETTINGS_TABS)[number];

const TAB_LABELS = {
	connections: "Connections",
	access: "Access",
	alerts: "Alerts",
	storage: "Log storage",
} satisfies Record<SettingsTab, string>;

const parseAsSettingsTab = parseAsStringLiteral(SETTINGS_TABS)
	.withDefault("connections")
	.withOptions({ history: "replace" });

// The same quiet link the app header uses, so the two navs read as one.
const navLinkClass =
	"inline-flex shrink-0 items-center rounded-md px-2.5 py-2 text-sm whitespace-nowrap sm:py-1.5 lg:w-full";

export function SettingsPage() {
	const [tab, setTab] = useQueryState("tab", parseAsSettingsTab);
	const { data, isLoading, error } = useSettings();

	return (
		<main className="app-width px-4 py-8 sm:px-6 lg:px-8">
			<div>
				<h1 className="text-2xl font-semibold tracking-tight">Settings</h1>
				<p className="mt-1 max-w-prose text-pretty text-base/6 text-muted-foreground sm:text-sm/6">
					Hosts, access, alerts and storage. Anything set via environment
					variable is shown here but changes only with a restart.
				</p>
			</div>

			<div className="mt-8 lg:grid lg:grid-cols-[11rem_minmax(0,1fr)] lg:gap-x-12">
				<nav
					aria-label="Settings sections"
					className="-mx-2.5 flex gap-0.5 overflow-x-auto border-b border-border/70 px-2.5 pb-3 lg:sticky lg:top-20 lg:mx-0 lg:flex-col lg:self-start lg:border-b-0 lg:px-0 lg:pb-0"
				>
					{SETTINGS_TABS.map((value) => {
						const isActive = value === tab;
						return (
							<button
								key={value}
								type="button"
								aria-current={isActive ? "page" : undefined}
								onClick={() => void setTab(value)}
								className={`${navLinkClass} ${
									isActive
										? "bg-muted text-foreground"
										: "text-muted-foreground hover:text-foreground"
								}`}
							>
								{TAB_LABELS[value]}
							</button>
						);
					})}
				</nav>

				<div className="mt-6 max-w-4xl lg:mt-0">
					{isLoading && (
						<div className="flex items-center gap-2 py-2 text-base text-muted-foreground sm:text-sm">
							<Spinner className="size-4" />
							Loading settings…
						</div>
					)}
					{error && (
						<ErrorNote>Failed to load settings: {error.message}</ErrorNote>
					)}
					{data && (
						<div className="divide-y divide-border/70">
							{tab === "connections" && (
								<>
									<DockerHostsSection
										key={JSON.stringify(data.dockerHosts)}
										config={data.dockerHosts}
									/>
									<CoolifyHostsSection
										key={JSON.stringify(data.coolifyHosts)}
										config={data.coolifyHosts}
									/>
								</>
							)}
							{tab === "access" && (
								<>
									<AuthSection
										key={`${data.auth.enabled}-${data.auth.adminUsername}`}
										config={data.auth}
									/>
									<ReadOnlySection config={data.readOnly} />
									<ApiTokensSection />
								</>
							)}
							{tab === "alerts" && <AlertsSection />}
							{tab === "storage" && (
								<LogStorageSection
									config={data.logStore}
									readOnly={data.readOnly.value}
								/>
							)}
						</div>
					)}
				</div>
			</div>
		</main>
	);
}
