import { Switch } from "@/components/ui/switch";

import { useUpdateReadOnly } from "../hooks/use-settings";
import type { ReadOnlyConfig } from "../types";
import { showResultToast } from "./mutation-toast";
import { EnvBadge, SettingsSection } from "./settings-ui";

interface ReadOnlySectionProps {
	config: ReadOnlyConfig;
}

export function ReadOnlySection({ config }: ReadOnlySectionProps) {
	const isEnv = config.source === "env";
	const mutation = useUpdateReadOnly();

	return (
		<SettingsSection
			title="Read-only mode"
			description="Disables start, stop, restart and remove for everyone. Logs stay readable."
			badge={isEnv && <EnvBadge />}
			action={
				<Switch
					id="read-only"
					aria-label="Enable read-only mode"
					checked={config.value}
					onCheckedChange={(checked) =>
						mutation.mutate(checked, showResultToast)
					}
					disabled={isEnv || mutation.isPending}
					className="relative after:absolute after:-inset-x-1 after:-inset-y-3"
				/>
			}
		/>
	);
}
