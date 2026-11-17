import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
	createMemoryHistory,
	createRootRoute,
	createRoute,
	createRouter,
	RouterProvider,
} from "@tanstack/react-router";
import {
	cleanup,
	fireEvent,
	render,
	screen,
	waitFor,
} from "@testing-library/react";
import { toast } from "sonner";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { DokployRollout } from "../api/dokploy-env";
import { ContainerEnvPanel } from "./container-env-panel";

const resource = {
	type: "compose",
	mapping_revision: "mapping",
	id: "compose",
	name: "Web stack",
	appName: "web",
	serverId: null,
	project: "Project",
	environment: "Production",
};
// The panel links to Settings, so it renders inside a router.
function mount(
	containerId = "container",
	onContainerIdChange?: (id: string) => void,
) {
	const client = new QueryClient({
		defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
	});
	const rootRoute = createRootRoute({
		component: () => (
			<ContainerEnvPanel
				containerId={containerId}
				containerHost="local"
				onContainerIdChange={onContainerIdChange}
			/>
		),
	});
	const router = createRouter({
		routeTree: rootRoute.addChildren([
			createRoute({ getParentRoute: () => rootRoute, path: "/settings" }),
		]),
		history: createMemoryHistory({ initialEntries: ["/"] }),
	});
	render(
		<QueryClientProvider client={client}>
			<RouterProvider router={router} />
		</QueryClientProvider>,
	);
}
// One deployment matches the container, so the picker preselects it.
const inventory = {
	source: "dokploy",
	env: {},
	resources: [resource],
	suggested_resources: ["compose/compose"],
	instance_url: "https://dokploy.test",
};
beforeEach(() => {
	vi.spyOn(toast, "success").mockReturnValue("");
	vi.spyOn(toast, "error").mockReturnValue("");
});
afterEach(() => {
	localStorage.clear();
	vi.unstubAllGlobals();
	vi.restoreAllMocks();
});

it("confirms ownership, saves the raw Compose configuration, then requests a separate deployment", async () => {
	const writes: { url: string; body: string }[] = [];
	vi.stubGlobal(
		"fetch",
		vi.fn(async (input: RequestInfo | URL, options?: RequestInit) => {
			const url = String(input);
			if (options?.method === "PUT") {
				writes.push({ url, body: String(options.body) });
				return Response.json({
					saved: true,
					applied: false,
					configuration: {
						text: "# keep\nTOKEN=${{project.TOKEN}}\nNEW=value",
						revision: "saved",
						createEnvFile: true,
					},
				});
			}
			if (options?.method === "POST") {
				writes.push({ url, body: "" });
				return Response.json({ applied: false }, { status: 202 });
			}
			if (url.includes("confirmed=true"))
				return Response.json({
					source: "dokploy",
					env: {},
					resource,
					configuration: {
						text: "# keep\nTOKEN=${{project.TOKEN}}",
						revision: "initial",
						createEnvFile: true,
					},
					instance_url: "https://dokploy.test",
				});
			return Response.json(inventory);
		}),
	);
	mount();
	fireEvent.click(
		await screen.findByRole("button", { name: "Use this deployment" }),
	);
	await screen.findByText(/Variables reach containers only/);
	expect(screen.queryByRole("textbox")).toBeNull();
	fireEvent.click(screen.getByRole("button", { name: "Reveal and edit" }));
	fireEvent.change(screen.getByRole("textbox", { name: "Saved environment" }), {
		target: { value: "# keep\nTOKEN=${{project.TOKEN}}\nNEW=value" },
	});
	fireEvent.click(screen.getByRole("button", { name: "Save in Dokploy" }));
	expect(writes).toHaveLength(0);
	await screen.findByText(
		/Adds NEW\. This changes the saved configuration of the whole Compose deployment/,
	);
	fireEvent.click(screen.getByRole("button", { name: "Save" }));
	await waitFor(() =>
		expect(toast.success).toHaveBeenCalledWith(
			"Environment saved in Dokploy",
			expect.anything(),
		),
	);
	expect(writes).toHaveLength(1);
	expect(JSON.parse(writes[0].body)).toEqual({
		text: "# keep\nTOKEN=${{project.TOKEN}}\nNEW=value",
		revision: "initial",
	});
	expect(writes[0].url).toContain("resource=compose");
	fireEvent.click(
		screen.getByRole("button", { name: "Deploy saved configuration" }),
	);
	expect(writes).toHaveLength(1);
	fireEvent.click(await screen.findByRole("button", { name: "Deploy" }));
	await waitFor(() => expect(writes).toHaveLength(2));
	expect(writes[1].url).toContain("/env/deploy?");
});

it("keeps the draft but blocks retry and deployment after an uncertain save until configuration reload succeeds", async () => {
	let rejectSave = true;
	vi.stubGlobal(
		"fetch",
		vi.fn(async (input: RequestInfo | URL, options?: RequestInit) => {
			if (options?.method === "PUT" && rejectSave)
				return Response.json(
					{ message: "Save could not be confirmed; reload" },
					{ status: 502 },
				);
			if (String(input).includes("confirmed=true"))
				return Response.json({
					source: "dokploy",
					env: {},
					resource,
					configuration: {
						text: "KEY=old",
						revision: "revision",
						createEnvFile: false,
					},
					instance_url: "https://dokploy.test",
				});
			return Response.json(inventory);
		}),
	);
	mount();
	fireEvent.click(
		await screen.findByRole("button", { name: "Use this deployment" }),
	);
	await screen.findByText(/Variables reach containers only/);
	fireEvent.click(screen.getByRole("button", { name: "Reveal and edit" }));
	fireEvent.change(screen.getByRole("textbox"), {
		target: { value: "KEY=new" },
	});
	fireEvent.click(screen.getByRole("button", { name: "Save in Dokploy" }));
	fireEvent.click(await screen.findByRole("button", { name: "Save" }));
	expect((await screen.findByRole("alert")).textContent).toContain(
		"Save could not be confirmed; reload",
	);
	expect(screen.getByRole("textbox")).toHaveProperty("value", "KEY=new");
	expect(screen.getByRole("textbox")).toHaveProperty("disabled", false);
	expect(
		screen
			.getByRole("button", { name: "Save in Dokploy" })
			.hasAttribute("disabled"),
	).toBe(true);
	expect(
		screen
			.getByRole("button", { name: "Deploy saved configuration" })
			.hasAttribute("disabled"),
	).toBe(true);
	rejectSave = false;
	fireEvent.click(screen.getByRole("button", { name: "Reload configuration" }));
	fireEvent.click(
		await screen.findByRole("button", { name: "Discard and reload" }),
	);
	await waitFor(() =>
		expect(screen.getByRole("textbox")).toHaveProperty("value", "KEY=old"),
	);
});

it("refreshes deployment inventory before confirming a changed mapping", async () => {
	let mappingRevision = "old-mapping";
	const reads: string[] = [];
	vi.stubGlobal(
		"fetch",
		vi.fn(async (input: RequestInfo | URL) => {
			const url = new URL(String(input), "http://localhost");
			const current = { ...resource, mapping_revision: mappingRevision };
			if (url.searchParams.has("confirmed")) {
				const revision = url.searchParams.get("mapping_revision") ?? "";
				reads.push(revision);
				if (revision !== mappingRevision)
					return new Response("Choose the deployment again", { status: 409 });
				return Response.json({
					source: "dokploy",
					env: {},
					resource: current,
					configuration: {
						text: "KEY=value",
						revision: "env-revision",
						createEnvFile: true,
					},
					instance_url: "https://dokploy.test",
				});
			}
			return Response.json({ ...inventory, resources: [current] });
		}),
	);
	mount();
	fireEvent.click(
		await screen.findByRole("button", { name: "Use this deployment" }),
	);
	await screen.findByText(/Variables reach containers only/);
	mappingRevision = "new-mapping";
	fireEvent.click(screen.getByRole("button", { name: "Change deployment" }));
	fireEvent.click(
		await screen.findByRole("button", { name: "Use this deployment" }),
	);
	await waitFor(() => expect(reads).toEqual(["old-mapping", "new-mapping"]));
	await screen.findByText(/Variables reach containers only/);
});

it("reopens a confirmed deployment without asking again, until the mapping is changed", async () => {
	vi.stubGlobal(
		"fetch",
		vi.fn(async (input: RequestInfo | URL) => {
			if (String(input).includes("confirmed=true"))
				return Response.json({
					source: "dokploy",
					env: {},
					resource,
					configuration: {
						text: "KEY=value",
						revision: "revision",
						createEnvFile: true,
					},
					instance_url: "https://dokploy.test",
				});
			return Response.json(inventory);
		}),
	);
	mount();
	fireEvent.click(
		await screen.findByRole("button", { name: "Use this deployment" }),
	);
	await screen.findByText(/Variables reach containers only/);
	cleanup();

	mount();
	await screen.findByText(/Variables reach containers only/);
	fireEvent.click(screen.getByRole("button", { name: "Change deployment" }));
	await screen.findByRole("button", { name: "Use this deployment" });
	cleanup();

	mount();
	await screen.findByRole("button", { name: "Use this deployment" });
	expect(screen.queryByText(/Variables reach containers only/)).toBeNull();
});

it("remembers a Swarm service's deployment after a deployment replaces its task", async () => {
	vi.stubGlobal(
		"fetch",
		vi.fn(async (input: RequestInfo | URL) => {
			if (String(input).includes("confirmed=true"))
				return Response.json({
					source: "dokploy",
					env: {},
					resource,
					configuration: {
						text: "KEY=value",
						revision: "revision",
						createEnvFile: true,
					},
					instance_url: "https://dokploy.test",
				});
			return Response.json({ ...inventory, mapping_key: "web" });
		}),
	);
	mount("old-task");
	fireEvent.click(
		await screen.findByRole("button", { name: "Use this deployment" }),
	);
	await screen.findByText(/Variables reach containers only/);
	cleanup();

	mount("new-task");
	await screen.findByText(/Variables reach containers only/);
	expect(
		screen.queryByRole("button", { name: "Use this deployment" }),
	).toBeNull();
});

it("moves a confirmation kept for the container to its workload", async () => {
	localStorage.setItem(
		"logdeck.dokployMappings",
		"local/old-task\tcompose/compose",
	);
	vi.stubGlobal(
		"fetch",
		vi.fn(async (input: RequestInfo | URL) => {
			if (String(input).includes("confirmed=true"))
				return Response.json({
					source: "dokploy",
					env: {},
					resource,
					configuration: {
						text: "KEY=value",
						revision: "revision",
						createEnvFile: true,
					},
					instance_url: "https://dokploy.test",
				});
			return Response.json({ ...inventory, mapping_key: "web" });
		}),
	);
	mount("old-task");
	await screen.findByText(/Variables reach containers only/);
	expect(localStorage.getItem("logdeck.dokployMappings")).toBe(
		"local/web\tcompose/compose",
	);
});

it("offers the runtime editor for plain Compose when Dokploy cannot be read", async () => {
	vi.stubGlobal(
		"fetch",
		vi.fn(async (input: RequestInfo | URL) =>
			Response.json(
				String(input).includes("platform=docker")
					? {
							source: "docker",
							env: { REGION: "runtime" },
							plain_compose: true,
						}
					: {
							...inventory,
							resources: [],
							suggested_resources: [],
							plain_compose_allowed: true,
							inventory_error: "Dokploy is unreachable",
						},
			),
		),
	);
	mount();
	await screen.findByText("Dokploy is unreachable");
	fireEvent.click(
		screen.getByRole("button", { name: "Not managed by Dokploy" }),
	);
	fireEvent.click(
		await screen.findByRole("button", { name: "Use runtime editor" }),
	);
	expect(
		await screen.findByRole("textbox", { name: "Value for REGION" }),
	).toHaveProperty("value", "runtime");
	fireEvent.click(screen.getByRole("button", { name: "Map to Dokploy" }));
	await screen.findByText("Dokploy is unreachable");
});

// Dokploy reports a deployment done before Swarm has replaced the task, and
// Swarm may roll it back, so the panel follows the rollout to its end.
const application = {
	...resource,
	type: "application",
	id: "app",
	name: "Web",
};
function stubRollout(answers: (() => Response)[]) {
	const rolloutUrls: string[] = [];
	vi.stubGlobal(
		"fetch",
		vi.fn(async (input: RequestInfo | URL, options?: RequestInit) => {
			const url = String(input);
			if (url.includes("/dokploy/rollout")) {
				rolloutUrls.push(url);
				return answers[Math.min(rolloutUrls.length, answers.length) - 1]();
			}
			if (options?.method === "POST")
				return Response.json(
					{ applied: false, after_deployment: "2026-11-17T12:00:00.5Z" },
					{ status: 202 },
				);
			if (url.includes("confirmed=true"))
				return Response.json({
					source: "dokploy",
					env: {},
					resource: application,
					configuration: {
						text: "KEY=saved",
						revision: "r",
						createEnvFile: true,
					},
					instance_url: "https://dokploy.test",
				});
			return Response.json({
				...inventory,
				resources: [application],
				suggested_resources: ["application/app"],
				workload_service: "web",
			});
		}),
	);
	return rolloutUrls;
}
const phase = (body: DokployRollout) => () => Response.json(body);

async function requestDeployment() {
	fireEvent.click(
		await screen.findByRole("button", { name: "Use this deployment" }),
	);
	fireEvent.click(
		await screen.findByRole("button", { name: "Deploy saved configuration" }),
	);
	fireEvent.click(await screen.findByRole("button", { name: "Deploy" }));
}

it("follows a deployment through Swarm, then moves to the container it started", async () => {
	vi.useFakeTimers({ shouldAdvanceTime: true });
	try {
		const rolloutUrls = stubRollout([
			phase({ phase: "updating" }),
			phase({ phase: "completed", container_id: "replacement" }),
		]);
		const moved = vi.fn<(id: string) => void>();
		mount("container", moved);
		await requestDeployment();
		await screen.findByText(/Swarm is starting the new task/);
		const query = new URL(rolloutUrls[0], "http://x").searchParams;
		expect(query.get("after")).toBe("2026-11-17T12:00:00.5Z");
		expect(query.get("service")).toBe("web");
		expect(moved).not.toHaveBeenCalled();
		await vi.advanceTimersByTimeAsync(3000);
		await waitFor(() => expect(moved).toHaveBeenCalledWith("replacement"));
		// Finished: no more polling, one announcement.
		await vi.advanceTimersByTimeAsync(10_000);
		expect(rolloutUrls).toHaveLength(2);
		expect(moved).toHaveBeenCalledTimes(1);
		expect(
			vi
				.mocked(toast.success)
				.mock.calls.filter(([title]) => title === "Deployed"),
		).toHaveLength(1);
	} finally {
		vi.useRealTimers();
	}
});

it("reports a Swarm rollback instead of a finished deployment", async () => {
	vi.useFakeTimers({ shouldAdvanceTime: true });
	try {
		const rolloutUrls = stubRollout([
			phase({ phase: "rolled_back", message: "rollback completed" }),
		]);
		const moved = vi.fn<(id: string) => void>();
		mount("container", moved);
		await requestDeployment();
		await screen.findByText(
			/Swarm rolled the update back: rollback completed\. The running container still has the previous configuration/,
		);
		await vi.advanceTimersByTimeAsync(10_000);
		expect(rolloutUrls).toHaveLength(1);
		expect(toast.success).not.toHaveBeenCalledWith(
			"Deployed",
			expect.anything(),
		);
		expect(moved).not.toHaveBeenCalled();
		expect(
			screen
				.getByRole("button", { name: "Deploy saved configuration" })
				.hasAttribute("disabled"),
		).toBe(false);
	} finally {
		vi.useRealTimers();
	}
});

it("stops following when the deployment's mapping changed", async () => {
	vi.useFakeTimers({ shouldAdvanceTime: true });
	try {
		const rolloutUrls = stubRollout([
			() =>
				new Response("The Dokploy connection or deployment name changed", {
					status: 409,
				}),
		]);
		mount();
		await requestDeployment();
		await screen.findByText(
			/Stopped following the deployment: The Dokploy connection/,
		);
		await vi.advanceTimersByTimeAsync(10_000);
		expect(rolloutUrls).toHaveLength(1);
		expect(
			screen
				.getByRole("button", { name: "Deploy saved configuration" })
				.hasAttribute("disabled"),
		).toBe(false);
	} finally {
		vi.useRealTimers();
	}
});
