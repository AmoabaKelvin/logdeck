import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
	act,
	fireEvent,
	render,
	screen,
	waitFor,
	within,
} from "@testing-library/react";
import { toast } from "sonner";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { EnvVariablesResponse } from "../api/get-container-env-variables";
import { ContainerEnvPanel } from "./container-env-panel";

const savedConfiguration = {
	source: "coolify",
	resource_type: "application",
	env: { KEY: "production", BUILD_ONLY: "build" },
	variables: [
		{
			uuid: "prod",
			key: "KEY",
			value: "production",
			is_preview: false,
			is_buildtime: true,
			is_runtime: false,
		},
		{
			uuid: "preview",
			key: "KEY",
			value: "preview",
			is_preview: true,
			is_runtime: true,
		},
		{
			uuid: "hidden",
			key: "TOKEN",
			value: null,
			is_preview: false,
			is_shown_once: true,
		},
		{
			uuid: "build",
			key: "BUILD_ONLY",
			value: "build",
			is_preview: false,
			is_buildtime: true,
		},
	].map((variable) => ({
		is_literal: false,
		is_multiline: false,
		is_shared: false,
		is_shown_once: false,
		...variable,
	})),
} satisfies EnvVariablesResponse;

beforeEach(() => {
	vi.spyOn(toast, "success").mockReturnValue("");
	vi.spyOn(toast, "error").mockReturnValue("");
});
afterEach(() => {
	vi.unstubAllGlobals();
	vi.restoreAllMocks();
});

function textarea(element: HTMLElement): HTMLTextAreaElement {
	if (!(element instanceof HTMLTextAreaElement))
		throw new Error("Expected a textarea");
	return element;
}
function button(element: HTMLElement): HTMLButtonElement {
	if (!(element instanceof HTMLButtonElement))
		throw new Error("Expected a button");
	return element;
}

function renderPanel() {
	const client = new QueryClient({
		defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
	});
	const onContainerIdChange = vi.fn<(id: string) => void>();
	render(
		<QueryClientProvider client={client}>
			<ContainerEnvPanel
				containerId="container"
				containerHost="local"
				onContainerIdChange={onContainerIdChange}
			/>
		</QueryClientProvider>,
	);
	return { onContainerIdChange, client };
}

async function editableField(name: string) {
	const field = await screen.findByRole("textbox", { name });
	fireEvent.click(
		within(field.closest("div") ?? field).getByRole("button", {
			name: "Reveal value",
		}),
	);
	expect(textarea(field).readOnly).toBe(false);
	return field;
}

it("saves only the edited preview row, leaving hidden and build variables untouched", async () => {
	const requests: RequestInit[] = [];
	let configuration: EnvVariablesResponse = savedConfiguration;
	vi.stubGlobal(
		"fetch",
		vi.fn<typeof fetch>(async (_input, init) => {
			if (init?.method === "PUT") {
				requests.push(init);
				return Response.json({ saved: true, completed: 1 });
			}
			return Response.json(configuration);
		}),
	);
	const { onContainerIdChange, client } = renderPanel();
	const preview = await editableField("KEY preview value");
	fireEvent.change(preview, { target: { value: "draft-preview" } });
	configuration = {
		...savedConfiguration,
		variables: savedConfiguration.variables.map((v) =>
			v.uuid === "preview" ? { ...v, value: "another-user-value" } : v,
		),
	};
	await act(async () => {
		await client.invalidateQueries({
			queryKey: ["container-env", "container", "local"],
		});
	});
	fireEvent.change(preview, { target: { value: "edited-preview" } });
	fireEvent.click(screen.getByRole("button", { name: "Save in Coolify" }));
	await waitFor(() => expect(toast.success).toHaveBeenCalled());
	expect(requests).toHaveLength(1);
	expect(JSON.parse(String(requests[0].body))).toEqual({
		changes: [
			{
				uuid: "preview",
				expected_value: "preview",
				key: "KEY",
				is_preview: true,
				value: "edited-preview",
			},
		],
	});
	expect(onContainerIdChange).not.toHaveBeenCalled();
	expect(
		screen.queryByRole("button", { name: "Deploy production through Coolify" }),
	).toBeNull();
	expect(
		textarea(screen.getByRole("textbox", { name: "TOKEN production value" }))
			.placeholder,
	).toContain("Unknown value");
});

it("leaves an unknown secret untouched when its replacement is cleared", async () => {
	const writes: RequestInit[] = [];
	vi.stubGlobal(
		"fetch",
		vi.fn<typeof fetch>(async (_input, init) => {
			if (init?.method === "PUT") {
				writes.push(init);
				return Response.json({ saved: true, completed: 1 });
			}
			return Response.json(savedConfiguration);
		}),
	);
	renderPanel();
	const hidden = await screen.findByRole("textbox", {
		name: "TOKEN production value",
	});
	fireEvent.change(hidden, { target: { value: "replacement" } });
	fireEvent.change(hidden, { target: { value: "" } });
	expect(
		button(screen.getByRole("button", { name: "Save in Coolify" })).disabled,
	).toBe(true);
	fireEvent.change(await editableField("KEY preview value"), {
		target: { value: "edited-preview" },
	});
	fireEvent.click(screen.getByRole("button", { name: "Save in Coolify" }));
	await waitFor(() => expect(writes).toHaveLength(1));
	expect(JSON.parse(String(writes[0].body))).toEqual({
		changes: [
			{
				uuid: "preview",
				key: "KEY",
				expected_value: "preview",
				is_preview: true,
				value: "edited-preview",
			},
		],
	});
});

it("offers a separate deployment after a production save and reports a request rather than applied changes", async () => {
	const methods: string[] = [];
	vi.stubGlobal(
		"fetch",
		vi.fn<typeof fetch>(async (_input, init) => {
			if (init?.method) methods.push(init.method);
			return Response.json(
				init?.method === "PUT"
					? { saved: true, completed: 1 }
					: savedConfiguration,
			);
		}),
	);
	const { onContainerIdChange } = renderPanel();
	fireEvent.change(await editableField("KEY production value"), {
		target: { value: "edited" },
	});
	fireEvent.click(screen.getByRole("button", { name: "Save in Coolify" }));
	const deploy = await screen.findByRole("button", {
		name: "Deploy production through Coolify",
	});
	await waitFor(() => expect(button(deploy).disabled).toBe(false));
	expect(methods).toEqual(["PUT"]);
	fireEvent.change(
		screen.getByRole("textbox", { name: "KEY production value" }),
		{ target: { value: "another draft" } },
	);
	expect(button(deploy).disabled).toBe(true);
	fireEvent.click(screen.getByRole("button", { name: "Discard" }));
	expect(button(deploy).disabled).toBe(false);
	fireEvent.click(deploy);
	expect(methods).toEqual(["PUT"]);
	fireEvent.click(await screen.findByRole("button", { name: "Deploy" }));
	await waitFor(() => expect(methods).toEqual(["PUT", "POST"]));
	await waitFor(() =>
		expect(toast.success).toHaveBeenCalledWith(
			"Coolify deployment requested",
			expect.anything(),
		),
	);
	expect(onContainerIdChange).not.toHaveBeenCalled();
});

it("blocks deployment and retries after a partial failure until the user reviews refreshed saved values", async () => {
	let failed = false;
	vi.stubGlobal(
		"fetch",
		vi.fn<typeof fetch>(async (_input, init) => {
			if (init?.method === "PUT") {
				failed = true;
				return Response.json(
					{
						saved: false,
						completed: 1,
						message:
							"Deletion failed after 1 acknowledged change; some changes may have been saved",
					},
					{ status: 502 },
				);
			}
			return Response.json(
				failed
					? {
							...savedConfiguration,
							variables: savedConfiguration.variables.map((v) =>
								v.uuid === "prod" ? { ...v, value: "saved-partially" } : v,
							),
						}
					: savedConfiguration,
			);
		}),
	);
	renderPanel();
	fireEvent.change(await editableField("KEY production value"), {
		target: { value: "draft" },
	});
	fireEvent.click(screen.getByRole("button", { name: "Save in Coolify" }));
	const alert = await screen.findByRole("alert");
	expect(alert.textContent).toContain("some changes may have been saved");
	expect(
		button(screen.getByRole("button", { name: "Save in Coolify" })).disabled,
	).toBe(true);
	expect(
		screen.queryByRole("button", { name: "Deploy production through Coolify" }),
	).toBeNull();
	fireEvent.click(
		screen.getByRole("button", {
			name: "Discard draft and review saved values",
		}),
	);
	await waitFor(() =>
		expect(
			textarea(screen.getByRole("textbox", { name: "KEY production value" }))
				.value,
		).toBe("saved-partially"),
	);
});
