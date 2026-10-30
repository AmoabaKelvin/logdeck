Log redaction masks secrets in container log lines before LogDeck stores them or sends them anywhere. Turn it on under **Settings > Redaction**. It is off until you enable it.

A redacted line looks like this:

```text
POST /login user=jane password=[REDACTED]
Authorization: Bearer [REDACTED]
connecting to postgres://app:[REDACTED]@db:5432/app
```

The patterns catch common secret formats. They cannot catch every secret, so treat redaction as a safety net, not a guarantee.

## What it covers

LogDeck redacts each line as it reads it from Docker or Podman, before it parses the line. The redacted text is the only version the rest of LogDeck sees:

- The live log view, history mode, and aggregated stack logs.
- The persistent [log store](/docs/log-history), so stored history holds the redacted line.
- The HTTP API, and with it the [CLI](/docs/cli) and the [MCP server](/docs/mcp).
- Downloads and copied lines in the UI, which are built from what the API returned.
- The sample line in [alert](/docs/alerting) history and notifications.

Because redaction runs before parsing, the message, the raw line, and any parsed fields all show the redacted value, including in multi-line entries such as stack traces. The timestamp Docker adds to each line is never changed.

Each physical line is redacted on its own, so a secret split across several lines, such as a PEM private key, is not caught.

## Built-in patterns

| Pattern               | Catches                                                                                                                   | Default |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------- | ------- |
| Authorization headers | The value of `Authorization` or `Proxy-Authorization`, keeping the scheme (`Bearer`, `Basic`, ...)                        | On      |
| Bearer tokens         | Any token after the word `Bearer`                                                                                         | On      |
| Passwords and secrets | Values of keys such as `password`, `secret`, `token`, `api_key`, `access_key` in `key=value`, `key: value`, and JSON form | On      |
| Passwords in URLs     | The password in `scheme://user:password@host`                                                                             | On      |
| Common API keys       | AWS access key IDs and GitHub, GitLab, Slack, Stripe, OpenAI, Anthropic, and Google API keys                              | On      |
| JSON Web Tokens       | Anything shaped like a JWT                                                                                                | On      |
| Email addresses       | Email addresses                                                                                                           | Off     |

Patterns that match a key keep the key and hide only the value.

## Custom patterns

Add your own under **Custom patterns**. Each one has:

- **Pattern**: an RE2 regular expression, the syntax Go uses. LogDeck rejects an invalid pattern, or one that can match empty text, with the reason when you save.
- **Replacement**: optional. Empty uses the section's replacement text, `[REDACTED]` by default. `$1` keeps the first capture group, so `(customer_id=)\d+` with `${1}***` turns `customer_id=4211` into `customer_id=***`.
- **Containers, Compose projects, Hosts**: optional targeting with the same rules as [alert targeting](/docs/alerting#targeting). Empty applies everywhere. Built-in patterns always apply everywhere.

Built-in patterns run first, then custom patterns from top to bottom. Each one runs on the text the previous one produced.

## Preview

Paste sample lines into **Preview** to see what the patterns do before you save or enable them. The preview runs your unsaved patterns as if redaction were on and applies every enabled pattern, whatever its targeting. LogDeck does not save or log the sample.

## Alerts

Alert rules match the original, unredacted line. Turning redaction on never changes which alerts fire, even for a pattern rule that matches text redaction hides. The sample line an alert keeps in its history and sends in notifications is redacted.

## Existing data

Saved changes apply to every line LogDeck reads afterwards, including log views that are already open. Nothing is rewritten after the fact: logs stored before you enabled a pattern, alert history, and anything already shown, downloaded, or sent keep their original text.

To remove stored history that may contain secrets, delete it under **Settings > Log storage**, for one container or for all of them. Clear alert history under **Settings > Alerts**.

## Who can change it

With authentication on, only admins can change redaction: a signed-in user or an admin API token. Read-scoped API tokens can neither read nor change the settings. The settings live in `config.json` under [`redaction`](/docs/configuration#redaction). LogDeck refuses to start if that key holds an invalid pattern, rather than store and serve the logs it was meant to hide.

## Performance

With redaction off, or when no pattern applies to a container, reading a line costs the same as before. With the default patterns on, a line without secrets costs about 0.3 microseconds more to process, and a line with a secret a few microseconds more.
