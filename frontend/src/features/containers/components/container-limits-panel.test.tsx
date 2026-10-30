import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
	cleanup,
	fireEvent,
	render,
	screen,
	waitFor,
} from "@testing-library/react";
import { toast } from "sonner";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

import { ContainerLimitsPanel } from "./container-limits-panel";
import { writeDokployMapping } from "./dokploy-mapping";

function mount(labels: Record<string, string>) {
	const client = new QueryClient({
		defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
	});
	render(
		<QueryClientProvider client={client}>
			<ContainerLimitsPanel
				containerId="container"
				containerHost="local"
				isReadOnly={false}
				labels={labels}
				stats={undefined}
				inspect={undefined}
			/>
		</QueryClientProvider>,
	);
}

const running = {
	memoryBytes: 1024 ** 3,
	nanoCPUs: 0,
	restartPolicy: { name: "any", maximumRetryCount: 0 },
};

beforeEach(() => {
	vi.spyOn(toast, "success").mockReturnValue("");
	vi.spyOn(toast, "error").mockReturnValue("");
});
afterEach(() => {
	cleanup();
	localStorage.clear();
	vi.unstubAllGlobals();
	vi.restoreAllMocks();
});

it("saves limits in the Dokploy deployment confirmed in the Environment panel, then deploys separately", async () => {
	writeDokployMapping(
		"local",
		"com.docker.swarm.service.name=web",
		"application/app",
	);
	const requests: { url: string; method: string; body?: string }[] = [];
	vi.stubGlobal(
		"fetch",
		vi.fn(async (input: RequestInfo | URL, options?: RequestInit) => {
			const url = String(input);
			const method = options?.method ?? "GET";
			requests.push({ url, method, body: options?.body?.toString() });
			if (method === "PUT")
				return Response.json({ saved: true, applied: false });
			if (method === "POST") return Response.json({ applied: false });
			return Response.json({
				...running,
				owner: {
					platform: "dokploy",
					persistent: true,
					configured: { memoryBytes: 1024 ** 3, nanoCPUs: 0 },
				},
			});
		}),
	);
	mount({ "com.docker.swarm.service.name": "web" });

	const memory = await screen.findByDisplayValue("1g");
	expect(requests[0].url).toContain("resource=app&resource_type=application");
	// Dokploy sets the restart policy, so it is not offered here.
	expect(screen.getByRole("button", { name: /any/i })).toHaveProperty(
		"disabled",
		true,
	);
	fireEvent.change(memory, { target: { value: "512m" } });
	fireEvent.click(screen.getByRole("button", { name: "Save in Dokploy" }));

	fireEvent.click(
		await screen.findByRole("button", { name: "Deploy through Dokploy" }),
	);
	fireEvent.click(await screen.findByRole("button", { name: "Deploy" }));
	await waitFor(() =>
		expect(requests.some((r) => r.method === "POST")).toBe(true),
	);
	const save = requests.find((r) => r.method === "PUT");
	expect(JSON.parse(save?.body ?? "")).toEqual({
		memoryBytes: 512 * 1024 ** 2,
		nanoCPUs: 0,
	});
	const deploy = requests.find((r) => r.method === "POST");
	expect(deploy?.url).toContain(
		"/resources/deploy?host=local&resource=app&resource_type=application",
	);
});

it("locks limits a platform owns but LogDeck cannot save, rather than updating them live", async () => {
	vi.stubGlobal(
		"fetch",
		vi.fn(async () =>
			Response.json({
				...running,
				owner: {
					platform: "coolify",
					persistent: false,
					reason:
						"Coolify services set their limits in the service's Compose file.",
					url: "https://coolify.test",
				},
			}),
		),
	);
	mount({ "coolify.managed": "true" });

	expect(
		await screen.findByText(/Coolify services set their limits/),
	).toBeTruthy();
	expect(screen.getByDisplayValue("1g")).toHaveProperty("disabled", true);
	expect(
		screen.getByRole("button", { name: "Save in Coolify" }),
	).toHaveProperty("disabled", true);
	expect(
		screen.getByRole("link", { name: /Open Coolify/ }).getAttribute("href"),
	).toBe("https://coolify.test");
});
