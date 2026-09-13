LogDeck alerts you when a container dies, gets OOM-killed, fails its health check, or starts logging errors.

LogDeck already watches every container's events and log stream. Alert rules match on what it sees and deliver each fired alert to every enabled notification channel. Manage rules and channels under **Settings > Alerts** in the UI, or with `logdeck alerts` from the terminal.

## Rule types

### Event rules

Event rules watch container lifecycle events. Three are alertable:

- `die`: the container exited. Only a **non-zero exit code** fires the rule. A clean exit (code 0) is not an alert condition. When the engine event arrives without an exit code, LogDeck inspects the container to find it.
- `oom`: the OOM killer killed the container.
- `unhealthy`: the container's health check transitioned to unhealthy. Recoveries (healthy or starting) do not fire.

An OOM kill usually emits `oom` followed right away by a `die` with code 137. For a rule that watches both, LogDeck counts that pair as one incident and alerts once.

### Log rules

Log rules match lines as they stream. A rule can set a minimum level, a regex pattern, or both. When it sets both, a line must match both:

- **Minimum level.** Matches any line at that level or more severe (`TRACE`, `DEBUG`, `INFO`, `WARN`, `ERROR`, `FATAL`, `PANIC`). Lines LogDeck cannot classify never pass a level filter.
- **Pattern.** An RE2 regular expression, tested against the parsed message and the raw line.

## Targeting

You can narrow every rule by **hosts**, **container names** (exact match), and **Compose projects**. LogDeck combines the dimensions with AND, and a dimension you leave empty matches everything. A rule with no targeting at all watches every container on every host.

## Rate thresholds and cooldowns

A single stray error is rarely worth a notification. Two controls decide when matches become alerts:

- **Threshold and window.** Fire only after N matches within W seconds ("5 errors in 60 seconds"). The window defaults to 60 seconds. A threshold of 0 or 1 fires on every match.
- **Cooldown.** The minimum time between deliveries for the same rule and container. It defaults to **300 seconds** when unset. LogDeck does not throw away matches that land during a cooldown. It counts them, and the next alert reports how many it **suppressed**.

LogDeck keeps threshold and cooldown state in memory, so it resets when LogDeck restarts.

## Channels

A channel is one notification destination. LogDeck delivers every fired alert to **every enabled channel**. Add channels under **Settings > Alerts**, or with `logdeck alerts channels add`. Four types are supported:

- **Webhook.** POSTs the JSON payload below to a URL. Slack and Discord incoming webhooks accept this shape unchanged, and so does anything else that takes a JSON POST.
- **ntfy.** POSTs the alert text as a plain-text body to a topic URL (for example `https://ntfy.sh/mytopic`), with a `Title` header.
- **Gotify.** POSTs `{title, message, priority}` to `<server URL>/message` using the channel's app token.
- **Telegram.** Calls the Bot API `sendMessage` with the alert text, using a bot token and a chat id.

The webhook payload:

```json
{
  "source": "logdeck",
  "version": 1,
  "text": "LogDeck alert: error spike: 5 matches (level >= ERROR) within 60s (prod/api)",
  "content": "LogDeck alert: error spike: 5 matches (level >= ERROR) within 60s (prod/api)",
  "alert": {
    "id": "...",
    "ruleId": "...",
    "ruleName": "error spike",
    "type": "log",
    "host": "prod",
    "containerId": "...",
    "containerName": "api",
    "reason": "5 matches (level >= ERROR) within 60s",
    "sample": "level=error msg=\"upstream timeout\"",
    "count": 5,
    "suppressed": 0,
    "firedAt": "2026-07-14T09:31:04Z"
  }
}
```

> **Why it works with Slack and Discord unchanged.** LogDeck sends the same human-readable summary twice, as `text` and as `content`. Slack (and Mattermost) render `text`. Discord renders `content`. So a Slack or Discord incoming-webhook URL works as-is, with no proxy or template in between. Receivers that want to parse the alert get the full alert object in the body.

Each delivery has a 10-second timeout. LogDeck retries network errors and 5xx responses once after 5 seconds, and treats other statuses as permanent. The alert history records one summary result per fired alert. It succeeds only if every enabled channel accepted the alert, and otherwise names the channel that failed.

Use **Test** next to a channel in Settings, or `logdeck alerts channels test <id>`, to verify it. With no channels configured, rules still evaluate and fire. LogDeck records the alerts in history and delivers them nowhere.

## Alert history

LogDeck keeps the most recent 500 fired alerts, newest first, and mirrors them to `alerts-history.json` next to the config file. History survives a restart as long as that directory is a mounted volume.

Each entry records the rule, the container and host, the reason, a sample line for log rules, how many matches LogDeck suppressed, and the delivery result. Read it under **Settings > Alerts**, with `logdeck alerts history`, or from `GET /api/v1/alerts/history`. Clear it with a single action in the UI, or with `logdeck alerts history clear`.

## Managing rules from the CLI

The CLI covers everything in the Alerts settings card, so you can script the same rules across deployments.

```bash
# Point alerts somewhere
logdeck alerts channels add --type webhook --name slack \
  --endpoint https://hooks.slack.com/services/...
logdeck alerts channels add --type telegram --secret "$TELEGRAM_BOT_TOKEN" --target "$CHAT_ID"
logdeck alerts channels list
logdeck alerts channels test <channel-id>

# Tell me when anything gets OOM-killed
logdeck alerts rules create --type event --name oom-watch --events oom

# Tell me when the api container crash-loops: 3 non-zero exits in 5 minutes
logdeck alerts rules create --type event --name api-crashloop \
  --events die --container api --threshold 3 --window 5m

# Tell me when prod starts spewing errors, at most once every 10 minutes
logdeck alerts rules create --type log --name error-spike \
  --min-level ERROR --host prod --threshold 5 --window 60s --cooldown 10m

# Match a specific failure, wherever it happens
logdeck alerts rules create --type log --name upstream-timeouts \
  --pattern "upstream (timed out|timeout)" --project checkout

# Inspect and manage
logdeck alerts rules                  # list, with targets and triggers
logdeck alerts rules disable <id>     # or enable / delete
logdeck alerts history --limit 20
```

`--host`, `--container`, and `--project` are repeatable and narrow the rule. `--window` and `--cooldown` accept durations (`60s`, `5m`) or bare seconds. See the [CLI reference](/docs/cli#alerts) for every flag.

## Storage

LogDeck persists alert rules and channels to `config.json` (`/data/config.json` by default) under `alerts`, alongside hosts and API tokens. It writes fired alerts to `alerts-history.json` beside it. Mount `/data` as a volume, or you lose both when you recreate the LogDeck container.

There are no alert-related environment variables. You configure alerting through the UI or the CLI only.
