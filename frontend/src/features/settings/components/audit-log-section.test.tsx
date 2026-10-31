import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";

import { AuditLogSection } from "./audit-log-section";

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

it("returns to the last page when a refresh leaves the current page past the end", async () => {
	let total = 21;
	vi.stubGlobal("fetch", async (url: string) => {
		const offset = Number(
			new URL(url, "http://test").searchParams.get("offset"),
		);
		const entries =
			offset < total
				? [
						{
							time: "2026-01-01T00:00:00Z",
							actor: "admin",
							action: "container.stop",
							method: "POST",
							route: "/api/v1/containers/{id}/stop",
							target: `at-${offset}`,
							status: 200,
							durationMs: 12,
						},
					]
				: [];
		return new Response(JSON.stringify({ entries, total }));
	});
	const client = new QueryClient({
		defaultOptions: { queries: { retry: false } },
	});
	render(
		<QueryClientProvider client={client}>
			<AuditLogSection />
		</QueryClientProvider>,
	);

	fireEvent.click(await screen.findByLabelText("Go to next page"));
	await screen.findByText("at-20");

	total = 20;
	await client.invalidateQueries();

	expect(await screen.findByText("at-0")).toBeTruthy();
});
