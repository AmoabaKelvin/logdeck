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

import { AuthContext } from "@/contexts/auth-context";
import type { ContainerInfo } from "../types";
import { ImageUpdateSection } from "./image-update-section";

const container: ContainerInfo = {
	id: "web",
	names: ["/web"],
	image: "nginx:latest",
	image_id: "sha256:old",
	command: "nginx",
	created: 0,
	state: "running",
	status: "Up",
	host: "local",
	image_update: { status: "available", checked_at: Date.now() / 1000 },
};

function mount(
	overrides: Partial<ContainerInfo>,
	onRecreated = vi.fn<(newContainerId: string) => void>(),
	role = "admin",
) {
	const client = new QueryClient({
		defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
	});
	render(
		<QueryClientProvider client={client}>
			<AuthContext.Provider
				value={{
					user: { username: "someone", role },
					token: "token",
					isAuthenticated: true,
					isLoading: false,
					isAuthEnabled: true,
					login: async () => {},
					loginWithToken: async () => {},
					logout: () => {},
				}}
			>
				<ImageUpdateSection
					container={{ ...container, ...overrides }}
					containerId="web"
					isReadOnly={false}
					onContainerRecreated={onRecreated}
				/>
			</AuthContext.Provider>
		</QueryClientProvider>,
	);
	return onRecreated;
}

beforeEach(() => {
	vi.spyOn(toast, "success").mockReturnValue("");
	vi.spyOn(toast, "error").mockReturnValue("");
});
afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
	vi.restoreAllMocks();
});

it("pulls and recreates a standalone container after confirmation", async () => {
	const fetch = vi.fn<(input: RequestInfo | URL) => Promise<Response>>(
		async () =>
			Response.json({ updated: true, new_container_id: "replacement" }),
	);
	vi.stubGlobal("fetch", fetch);
	const onRecreated = mount({});

	expect(screen.getByText("Update available")).toBeTruthy();
	fireEvent.click(screen.getByRole("button", { name: "Pull and recreate" }));
	fireEvent.click(
		await screen.findByRole("button", { name: "Pull and recreate" }),
	);

	await waitFor(() => expect(onRecreated).toHaveBeenCalledWith("replacement"));
	expect(String(fetch.mock.calls[0][0])).toContain(
		"/containers/web/image/update?host=local",
	);
});

it("explains Compose instead of recreating", () => {
	mount({
		labels: {
			"com.docker.compose.project": "shop",
			"com.docker.compose.project.working_dir": "/srv/shop",
		},
	});

	expect(
		screen.getByText(
			/docker compose pull && docker compose up -d in \/srv\/shop/,
		),
	).toBeTruthy();
	expect(
		screen.queryByRole("button", { name: "Pull and recreate" }),
	).toBeNull();
});

it("offers the check to admins but not to read-only users", () => {
	mount({});
	expect(screen.getByRole("button", { name: "Check now" })).toBeTruthy();
	cleanup();

	mount({}, undefined, "read");
	expect(screen.queryByRole("button", { name: "Check now" })).toBeNull();
});
