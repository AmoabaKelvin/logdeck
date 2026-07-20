import { getContainerById, state } from "@/lib/logdeck-demo/store";

export async function getContainerEnvVariables(
	id: string,
	_host: string,
): Promise<Record<string, string>> {
	const container = getContainerById(id);
	if (!container) throw new Error("Container not found");
	return structuredClone(state.env[container.id] ?? {});
}
