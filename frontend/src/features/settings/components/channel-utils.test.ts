import { describe, expect, it } from "vitest";

import type { AlertChannel } from "../api/get-alert-channels";
import {
	buildChannelPayload,
	channelDestination,
	EMPTY_CHANNEL_DRAFT,
} from "./channel-utils";

describe("buildChannelPayload", () => {
	it("builds a webhook payload from a valid URL", () => {
		const result = buildChannelPayload({
			...EMPTY_CHANNEL_DRAFT,
			type: "webhook",
			name: "Slack",
			url: "https://example.com/hook",
		});
		expect(result).toEqual({
			payload: {
				type: "webhook",
				enabled: true,
				name: "Slack",
				url: "https://example.com/hook",
			},
		});
	});

	it("rejects a webhook without a URL", () => {
		expect(
			buildChannelPayload({ ...EMPTY_CHANNEL_DRAFT, type: "webhook" }),
		).toEqual({ error: expect.stringMatching(/url is required/i) });
	});

	it("rejects a non-http URL", () => {
		expect(
			buildChannelPayload({
				...EMPTY_CHANNEL_DRAFT,
				type: "ntfy",
				url: "ftp://ntfy.sh/topic",
			}),
		).toEqual({ error: expect.stringMatching(/http/i) });
	});

	it("requires a token for gotify", () => {
		expect(
			buildChannelPayload({
				...EMPTY_CHANNEL_DRAFT,
				type: "gotify",
				url: "https://gotify.example.com",
			}),
		).toEqual({ error: expect.stringMatching(/token/i) });
	});

	it("requires both a token and a chat id for telegram", () => {
		expect(
			buildChannelPayload({
				...EMPTY_CHANNEL_DRAFT,
				type: "telegram",
				token: "bot:secret",
			}),
		).toEqual({ error: expect.stringMatching(/chat id/i) });

		expect(
			buildChannelPayload({
				...EMPTY_CHANNEL_DRAFT,
				type: "telegram",
				token: "bot:secret",
				target: "-1001",
			}),
		).toEqual({
			payload: {
				type: "telegram",
				enabled: true,
				token: "bot:secret",
				target: "-1001",
			},
		});
	});
});

describe("channelDestination", () => {
	it("summarizes a telegram channel as its chat id", () => {
		const ch: AlertChannel = {
			id: "c1",
			type: "telegram",
			enabled: true,
			target: "-1001",
		};
		expect(channelDestination(ch)).toBe("chat -1001");
	});

	it("shows the url for other channel types", () => {
		const ch: AlertChannel = {
			id: "c2",
			type: "webhook",
			enabled: true,
			url: "https://example.com/hook",
		};
		expect(channelDestination(ch)).toBe("https://example.com/hook");
	});
});
