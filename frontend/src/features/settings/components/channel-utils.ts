import type { AlertChannelPayload } from "../api/create-alert-channel";
import type { AlertChannel, AlertChannelType } from "../api/get-alert-channels";

export const CHANNEL_TYPES: AlertChannelType[] = [
	"webhook",
	"slack",
	"discord",
	"email",
	"ntfy",
	"gotify",
	"telegram",
	"pagerduty",
	"opsgenie",
];

const TYPE_LABELS = {
	webhook: "Webhook",
	ntfy: "ntfy",
	gotify: "Gotify",
	telegram: "Telegram",
	slack: "Slack",
	discord: "Discord",
	email: "Email",
	pagerduty: "PagerDuty",
	opsgenie: "Opsgenie",
} satisfies Record<AlertChannelType, string>;

export function channelTypeLabel(type: AlertChannelType): string {
	return TYPE_LABELS[type];
}

export function channelDestination(channel: AlertChannel): string {
	if (channel.type === "telegram") {
		return channel.target ? `chat ${channel.target}` : "—";
	}
	if (channel.type === "email") {
		return channel.target || "—";
	}
	return channel.url || "—";
}

export interface ChannelDraft {
	type: AlertChannelType;
	name: string;
	url: string;
	token: string;
	target: string;
	username: string;
	from: string;
}

export const EMPTY_CHANNEL_DRAFT: ChannelDraft = {
	type: "webhook",
	name: "",
	url: "",
	token: "",
	target: "",
	username: "",
	from: "",
};

export type ChannelField = "url" | "username" | "token" | "from" | "target";

export interface ChannelFieldSpec {
	field: ChannelField;
	label: string;
	placeholder: string;
	optional?: boolean;
}

// The inputs each channel type takes, in display order. Token is always
// rendered as a password input.
const CHANNEL_FIELDS = {
	webhook: [
		{ field: "url", label: "URL", placeholder: "https://example.com/webhook" },
	],
	ntfy: [
		{ field: "url", label: "URL", placeholder: "https://ntfy.sh/mytopic" },
	],
	gotify: [
		{
			field: "url",
			label: "Server URL",
			placeholder: "https://gotify.example.com",
		},
		{ field: "token", label: "App token", placeholder: "App token" },
	],
	telegram: [
		{ field: "token", label: "Bot token", placeholder: "Bot token" },
		{ field: "target", label: "Chat id", placeholder: "-1001234567890" },
	],
	slack: [
		{
			field: "url",
			label: "Webhook URL",
			placeholder: "https://hooks.slack.com/services/...",
		},
	],
	discord: [
		{
			field: "url",
			label: "Webhook URL",
			placeholder: "https://discord.com/api/webhooks/...",
		},
	],
	email: [
		{
			field: "url",
			label: "SMTP server",
			placeholder: "smtp://smtp.example.com:587",
		},
		{
			field: "username",
			label: "Username",
			placeholder: "Username",
			optional: true,
		},
		{
			field: "token",
			label: "Password",
			placeholder: "Password",
			optional: true,
		},
		{
			field: "from",
			label: "From",
			placeholder: "LogDeck <alerts@example.com>",
		},
		{
			field: "target",
			label: "To",
			placeholder: "oncall@example.com, ops@example.com",
		},
	],
	pagerduty: [
		{ field: "token", label: "Routing key", placeholder: "Integration key" },
		{
			field: "url",
			label: "Events API URL",
			placeholder: "https://events.pagerduty.com/v2/enqueue",
			optional: true,
		},
	],
	opsgenie: [
		{ field: "token", label: "API key", placeholder: "API key" },
		{
			field: "url",
			label: "API URL",
			placeholder: "https://api.opsgenie.com",
			optional: true,
		},
	],
} satisfies Record<AlertChannelType, ChannelFieldSpec[]>;

export function channelFields(type: AlertChannelType): ChannelFieldSpec[] {
	return CHANNEL_FIELDS[type];
}

function isHttpUrl(value: string): boolean {
	try {
		const parsed = new URL(value);
		return parsed.protocol === "http:" || parsed.protocol === "https:";
	} catch {
		return false;
	}
}

function isSmtpUrl(value: string): boolean {
	return /^smtps?:\/\/[^/\s]+/i.test(value);
}

// Mirrors the server's per-type validation so users get feedback before the
// request. The server also checks email addresses.
export function buildChannelPayload(
	draft: ChannelDraft,
): { error: string } | { payload: AlertChannelPayload } {
	const payload: AlertChannelPayload = {
		type: draft.type,
		enabled: true,
	};
	const name = draft.name.trim();
	if (name) payload.name = name;

	for (const spec of channelFields(draft.type)) {
		const value = draft[spec.field].trim();
		if (!value) {
			if (spec.optional) continue;
			return { error: `${spec.label} is required` };
		}
		if (spec.field === "url") {
			if (draft.type === "email" && !isSmtpUrl(value))
				return { error: "SMTP server must be an smtp:// or smtps:// URL" };
			if (draft.type !== "email" && !isHttpUrl(value))
				return { error: "URL must be a valid http or https URL" };
		}
		payload[spec.field] = value;
	}

	return { payload };
}
