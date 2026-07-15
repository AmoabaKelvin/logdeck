import type { AlertChannelPayload } from "../api/create-alert-channel";
import type { AlertChannel, AlertChannelType } from "../api/get-alert-channels";

export const CHANNEL_TYPES: AlertChannelType[] = [
	"webhook",
	"ntfy",
	"gotify",
	"telegram",
];

const TYPE_LABELS: Record<AlertChannelType, string> = {
	webhook: "Webhook",
	ntfy: "ntfy",
	gotify: "Gotify",
	telegram: "Telegram",
};

export function channelTypeLabel(type: AlertChannelType): string {
	return TYPE_LABELS[type] ?? type;
}

export function channelDestination(channel: AlertChannel): string {
	if (channel.type === "telegram") {
		return channel.target ? `chat ${channel.target}` : "—";
	}
	return channel.url || "—";
}

export interface ChannelDraft {
	type: AlertChannelType;
	name: string;
	url: string;
	token: string;
	target: string;
}

export const EMPTY_CHANNEL_DRAFT: ChannelDraft = {
	type: "webhook",
	name: "",
	url: "",
	token: "",
	target: "",
};

function isHttpUrl(value: string): boolean {
	try {
		const parsed = new URL(value);
		return parsed.protocol === "http:" || parsed.protocol === "https:";
	} catch {
		return false;
	}
}

// Mirrors the server's per-type validation so users get feedback before the request.
export function buildChannelPayload(
	draft: ChannelDraft,
): { error: string } | { payload: AlertChannelPayload } {
	const name = draft.name.trim();
	const url = draft.url.trim();
	const token = draft.token.trim();
	const target = draft.target.trim();

	const payload: AlertChannelPayload = {
		type: draft.type,
		enabled: true,
	};
	if (name) payload.name = name;

	switch (draft.type) {
		case "webhook":
		case "ntfy": {
			if (!url) return { error: "A destination URL is required" };
			if (!isHttpUrl(url))
				return { error: "URL must be a valid http or https URL" };
			payload.url = url;
			break;
		}
		case "gotify": {
			if (!url) return { error: "The Gotify server URL is required" };
			if (!isHttpUrl(url))
				return { error: "URL must be a valid http or https URL" };
			if (!token) return { error: "A Gotify app token is required" };
			payload.url = url;
			payload.token = token;
			break;
		}
		case "telegram": {
			if (!token) return { error: "A Telegram bot token is required" };
			if (!target) return { error: "A Telegram chat id is required" };
			payload.token = token;
			payload.target = target;
			break;
		}
	}

	return { payload };
}
