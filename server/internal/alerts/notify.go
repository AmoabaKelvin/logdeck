package alerts

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/models"
)

const (
	deliverTimeout = 10 * time.Second
	retryDelay     = 5 * time.Second
	// gotifyPriority is the default message priority sent to Gotify servers.
	gotifyPriority = 5
	// alertTitle is the notification title used by channel types that carry one
	// out of band (ntfy Title header, Gotify title field) and the heading of
	// the formatted ones (Slack, Discord).
	alertTitle = "LogDeck alert"

	pagerDutyEventsURL = "https://events.pagerduty.com/v2/enqueue"
	opsgenieAPIURL     = "https://api.opsgenie.com"

	// Discord embed colors for a firing and a recovered alert.
	discordRed   = 0xe5484d
	discordGreen = 0x30a46c
)

// webhookPayload is the JSON body POSTed to a generic webhook channel. Text and
// Content carry the same human-readable summary so common receivers (Slack,
// Discord, Mattermost) render it without templates.
type webhookPayload struct {
	Source  string       `json:"source"`
	Version int          `json:"version"`
	Text    string       `json:"text"`
	Content string       `json:"content"`
	Alert   models.Alert `json:"alert"`
}

// gotifyPayload is the JSON body POSTed to a Gotify server's /message endpoint.
type gotifyPayload struct {
	Title    string `json:"title"`
	Message  string `json:"message"`
	Priority int    `json:"priority"`
}

// telegramPayload is the JSON body POSTed to the Telegram Bot API sendMessage
// method. ChatID is a string so numeric ids and @channel handles both work.
type telegramPayload struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

// slackPayload is a Slack incoming-webhook message: Text is the notification
// fallback, Blocks the rendered message.
type slackPayload struct {
	Text   string       `json:"text"`
	Blocks []slackBlock `json:"blocks"`
}

type slackBlock struct {
	Type   string      `json:"type"`
	Text   *slackText  `json:"text,omitempty"`
	Fields []slackText `json:"fields,omitempty"`
}

type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// discordPayload is a Discord webhook message carrying one embed.
type discordPayload struct {
	Embeds []discordEmbed `json:"embeds"`
}

type discordEmbed struct {
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Color       int            `json:"color"`
	Fields      []discordField `json:"fields,omitempty"`
}

type discordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

// pagerDutyEvent is a PagerDuty Events API v2 event. A resolve carries no
// payload.
type pagerDutyEvent struct {
	RoutingKey  string            `json:"routing_key"`
	EventAction string            `json:"event_action"`
	DedupKey    string            `json:"dedup_key"`
	Payload     *pagerDutyPayload `json:"payload,omitempty"`
}

type pagerDutyPayload struct {
	Summary       string       `json:"summary"`
	Source        string       `json:"source"`
	Severity      string       `json:"severity"`
	Timestamp     string       `json:"timestamp,omitempty"`
	Component     string       `json:"component,omitempty"`
	Class         string       `json:"class,omitempty"`
	CustomDetails models.Alert `json:"custom_details"`
}

// opsgenieAlert is the body of an Opsgenie create-alert request. Alias is the
// dedup key a later close refers to.
type opsgenieAlert struct {
	Message     string `json:"message"`
	Alias       string `json:"alias"`
	Description string `json:"description"`
	Source      string `json:"source"`
	Entity      string `json:"entity,omitempty"`
}

// requestSpec is the fully-resolved HTTP request for one channel delivery.
type requestSpec struct {
	method  string
	url     string
	headers map[string]string
	body    []byte
	// secret, when set, is stripped from any delivery error before it is
	// stored. Telegram embeds its bot token in the request path, so a network
	// error's *url.Error would otherwise persist the token to alert history.
	secret string
}

// redactSecret removes a channel secret from an error message so it is never
// written to alert history or shown in the UI.
func redactSecret(msg, secret string) string {
	if secret == "" {
		return msg
	}
	return strings.ReplaceAll(msg, secret, "***")
}

// notifier delivers alert payloads to notification channels and reports each
// attempt as a models.DeliveryResult.
type notifier struct {
	client     *http.Client
	retryDelay time.Duration
}

func newNotifier() *notifier {
	return &notifier{
		client:     &http.Client{Timeout: deliverTimeout},
		retryDelay: retryDelay,
	}
}

// alertText builds the human-readable summary for an alert.
func alertText(a models.Alert) string {
	target := a.Host
	if a.ContainerName != "" {
		target += "/" + a.ContainerName
	}
	if target != "" {
		return fmt.Sprintf("LogDeck alert: %s: %s (%s)", a.RuleName, a.Reason, target)
	}
	return fmt.Sprintf("LogDeck alert: %s: %s", a.RuleName, a.Reason)
}

// dedupKey identifies the incident an alert belongs to in PagerDuty and
// Opsgenie, so repeat alerts for the same rule and target group into one
// incident and a recovery resolves the one its failure opened.
func dedupKey(a models.Alert) string {
	return strings.Join([]string{"logdeck", a.RuleID, a.Host, a.ContainerName}, ":")
}

// truncate shortens s to at most n runes, for receivers with length limits.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// alertDetail is one labelled fact about an alert, rendered by the formatted
// channel types.
type alertDetail struct {
	name, value string
}

// alertDetails lists the alert's facts beyond its rule name and reason.
func alertDetails(a models.Alert) []alertDetail {
	var out []alertDetail
	if a.Host != "" {
		out = append(out, alertDetail{"Host", a.Host})
	}
	if a.ContainerName != "" {
		out = append(out, alertDetail{"Container", a.ContainerName})
	}
	if a.Suppressed > 0 {
		out = append(out, alertDetail{"Suppressed", strconv.Itoa(a.Suppressed)})
	}
	if a.Sample != "" {
		out = append(out, alertDetail{"Sample", truncate(a.Sample, 1000)})
	}
	if a.FiredAt != "" {
		out = append(out, alertDetail{"Fired at", a.FiredAt})
	}
	return out
}

// alertBody is the plain-text alert used as an email body and an Opsgenie
// description.
func alertBody(a models.Alert) string {
	var b strings.Builder
	b.WriteString(alertText(a) + "\n\nRule: " + a.RuleName + "\nReason: " + a.Reason + "\n")
	for _, d := range alertDetails(a) {
		b.WriteString(d.name + ": " + d.value + "\n")
	}
	return b.String()
}

// slackEscape escapes the characters Slack mrkdwn treats as markup, so log
// text cannot inject links or mentions.
var slackEscape = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

func slackMessage(a models.Alert) slackPayload {
	blocks := []slackBlock{{
		Type: "section",
		Text: &slackText{Type: "mrkdwn", Text: "*" + slackEscape.Replace(alertTitle+": "+a.RuleName) + "*\n" + slackEscape.Replace(a.Reason)},
	}}
	var fields []slackText
	for _, d := range alertDetails(a) {
		fields = append(fields, slackText{Type: "mrkdwn", Text: "*" + d.name + "*\n" + slackEscape.Replace(d.value)})
	}
	if len(fields) > 0 {
		blocks = append(blocks, slackBlock{Type: "section", Fields: fields})
	}
	return slackPayload{Text: slackEscape.Replace(alertText(a)), Blocks: blocks}
}

func discordMessage(a models.Alert) discordPayload {
	embed := discordEmbed{
		Title:       truncate(alertTitle+": "+a.RuleName, 256),
		Description: truncate(a.Reason, 4096),
		Color:       discordRed,
	}
	if a.Recovered {
		embed.Color = discordGreen
	}
	for _, d := range alertDetails(a) {
		embed.Fields = append(embed.Fields, discordField{Name: d.name, Value: d.value, Inline: d.name != "Sample"})
	}
	return discordPayload{Embeds: []discordEmbed{embed}}
}

// pagerDutyMessage triggers an incident for a fired alert and resolves it for
// a recovery. Alert kinds without recoveries leave the incident open until
// someone resolves it in PagerDuty.
func pagerDutyMessage(ch config.AlertChannel, a models.Alert) pagerDutyEvent {
	event := pagerDutyEvent{RoutingKey: ch.Token, EventAction: "resolve", DedupKey: dedupKey(a)}
	if a.Recovered {
		return event
	}
	source := a.Host
	if source == "" {
		source = "logdeck"
	}
	event.EventAction = "trigger"
	event.Payload = &pagerDutyPayload{
		Summary:       truncate(alertText(a), 1024),
		Source:        source,
		Severity:      "critical",
		Timestamp:     a.FiredAt,
		Component:     a.ContainerName,
		Class:         a.Type,
		CustomDetails: a,
	}
	return event
}

// buildRequestSpec resolves the HTTP request for delivering alert to channel.
func buildRequestSpec(ch config.AlertChannel, alert models.Alert) (requestSpec, error) {
	text := alertText(alert)
	switch ch.Type {
	case "webhook":
		body, err := json.Marshal(webhookPayload{
			Source:  "logdeck",
			Version: 1,
			Text:    text,
			Content: text,
			Alert:   alert,
		})
		if err != nil {
			return requestSpec{}, fmt.Errorf("failed to marshal payload: %v", err)
		}
		return requestSpec{
			method:  http.MethodPost,
			url:     ch.URL,
			headers: map[string]string{"Content-Type": "application/json"},
			body:    body,
		}, nil
	case "ntfy":
		return requestSpec{
			method: http.MethodPost,
			url:    ch.URL,
			headers: map[string]string{
				"Title":    alertTitle,
				"Priority": "high",
				"Tags":     "warning",
			},
			body: []byte(text),
		}, nil
	case "gotify":
		body, err := json.Marshal(gotifyPayload{Title: alertTitle, Message: text, Priority: gotifyPriority})
		if err != nil {
			return requestSpec{}, fmt.Errorf("failed to marshal payload: %v", err)
		}
		// The token goes in the X-Gotify-Key header, not the query string, so it
		// never appears in a URL that could be logged or persisted in an error.
		return requestSpec{
			method: http.MethodPost,
			url:    strings.TrimRight(ch.URL, "/") + "/message",
			headers: map[string]string{
				"Content-Type": "application/json",
				"X-Gotify-Key": ch.Token,
			},
			body: body,
		}, nil
	case "telegram":
		body, err := json.Marshal(telegramPayload{ChatID: ch.Target, Text: text})
		if err != nil {
			return requestSpec{}, fmt.Errorf("failed to marshal payload: %v", err)
		}
		return requestSpec{
			method:  http.MethodPost,
			url:     "https://api.telegram.org/bot" + ch.Token + "/sendMessage",
			headers: map[string]string{"Content-Type": "application/json"},
			body:    body,
			secret:  ch.Token,
		}, nil
	case "slack", "discord":
		var payload any = slackMessage(alert)
		if ch.Type == "discord" {
			payload = discordMessage(alert)
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return requestSpec{}, fmt.Errorf("failed to marshal payload: %v", err)
		}
		// The webhook URL is the credential, so keep it out of stored errors.
		return requestSpec{
			method:  http.MethodPost,
			url:     ch.URL,
			headers: map[string]string{"Content-Type": "application/json"},
			body:    body,
			secret:  ch.URL,
		}, nil
	case "pagerduty":
		body, err := json.Marshal(pagerDutyMessage(ch, alert))
		if err != nil {
			return requestSpec{}, fmt.Errorf("failed to marshal payload: %v", err)
		}
		endpoint := ch.URL
		if endpoint == "" {
			endpoint = pagerDutyEventsURL
		}
		return requestSpec{
			method:  http.MethodPost,
			url:     endpoint,
			headers: map[string]string{"Content-Type": "application/json"},
			body:    body,
		}, nil
	case "opsgenie":
		// A fired alert creates an Opsgenie alert and a recovery closes it,
		// both by alias.
		base := strings.TrimRight(ch.URL, "/")
		if base == "" {
			base = opsgenieAPIURL
		}
		endpoint := base + "/v2/alerts"
		var payload any = opsgenieAlert{
			Message:     truncate(alertText(alert), 130),
			Alias:       dedupKey(alert),
			Description: truncate(alertBody(alert), 15000),
			Source:      "LogDeck",
			Entity:      alert.Host,
		}
		if alert.Recovered {
			endpoint += "/" + url.PathEscape(dedupKey(alert)) + "/close?identifierType=alias"
			payload = map[string]string{"source": "LogDeck"}
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return requestSpec{}, fmt.Errorf("failed to marshal payload: %v", err)
		}
		return requestSpec{
			method: http.MethodPost,
			url:    endpoint,
			headers: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "GenieKey " + ch.Token,
			},
			body: body,
		}, nil
	default:
		return requestSpec{}, fmt.Errorf("unknown channel type %q", ch.Type)
	}
}

// sendEmail delivers alert over SMTP and classifies the outcome like attempt:
// the bool reports whether the failure is retryable, which is anything but a
// permanent (5xx) SMTP reply.
func sendEmail(ctx context.Context, ch config.AlertChannel, alert models.Alert) (models.DeliveryResult, bool) {
	err := sendMail(ctx, ch, alert)
	if err == nil {
		return models.DeliveryResult{Status: "ok"}, false
	}
	var reply *textproto.Error
	permanent := errors.As(err, &reply) && reply.Code >= 500
	return models.DeliveryResult{Status: "failed", Error: redactSecret(err.Error(), ch.Token)}, !permanent
}

// sendMail sends one alert email. ch.URL is smtp://host[:port], which
// upgrades with STARTTLS when the server offers it (port 587 by default), or
// smtps://host[:port] for implicit TLS (port 465 by default). net/smtp only
// sends a password over TLS or to localhost.
func sendMail(ctx context.Context, ch config.AlertChannel, alert models.Alert) error {
	u, err := url.Parse(ch.URL)
	if err != nil {
		return fmt.Errorf("invalid SMTP URL: %v", err)
	}
	from, err := mail.ParseAddress(ch.From)
	if err != nil {
		return fmt.Errorf("invalid from address: %v", err)
	}
	to, err := mail.ParseAddressList(ch.Target)
	if err != nil {
		return fmt.Errorf("invalid recipients: %v", err)
	}
	host, port := u.Hostname(), u.Port()
	if port == "" {
		port = "587"
		if u.Scheme == "smtps" {
			port = "465"
		}
	}

	dialer := net.Dialer{Timeout: deliverTimeout}
	raw, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		return err
	}
	defer raw.Close()
	// net/smtp takes no context, so a deadline bounds the whole exchange like
	// the HTTP client timeout, and cancelling ctx (shutdown) closes the
	// connection.
	_ = raw.SetDeadline(time.Now().Add(deliverTimeout))
	stop := context.AfterFunc(ctx, func() { raw.Close() })
	defer stop()

	tlsConfig := &tls.Config{ServerName: host}
	conn := raw
	if u.Scheme == "smtps" {
		conn = tls.Client(raw, tlsConfig)
	}
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer c.Close()
	if ok, _ := c.Extension("STARTTLS"); ok && u.Scheme == "smtp" {
		if err := c.StartTLS(tlsConfig); err != nil {
			return err
		}
	}
	if ch.Username != "" {
		if err := c.Auth(smtp.PlainAuth("", ch.Username, ch.Token, host)); err != nil {
			return err
		}
	}
	if err := c.Mail(from.Address); err != nil {
		return err
	}
	for _, addr := range to {
		if err := c.Rcpt(addr.Address); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(emailMessage(from, to, alert)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	// The server has accepted the message by now. A failed QUIT must not read
	// as a failed delivery, or the retry would send it twice.
	_ = c.Quit()
	return nil
}

// emailMessage renders the alert as a plain-text message.
func emailMessage(from *mail.Address, to []*mail.Address, a models.Alert) []byte {
	recipients := make([]string, len(to))
	for i, addr := range to {
		recipients[i] = addr.String()
	}
	// Keep a stray line break in a rule name or reason from ending the
	// Subject header early.
	subject := strings.NewReplacer("\r", " ", "\n", " ").Replace(alertText(a))

	var b bytes.Buffer
	b.WriteString("From: " + from.String() + "\r\n")
	b.WriteString("To: " + strings.Join(recipients, ", ") + "\r\n")
	b.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	qp := quotedprintable.NewWriter(&b)
	_, _ = qp.Write([]byte(strings.ReplaceAll(alertBody(a), "\n", "\r\n")))
	_ = qp.Close()
	return b.Bytes()
}

type channelOutcome struct {
	channel config.AlertChannel
	result  models.DeliveryResult
}

func enabledChannels(channels []config.AlertChannel) []config.AlertChannel {
	out := make([]config.AlertChannel, 0, len(channels))
	for _, ch := range channels {
		if ch.Enabled {
			out = append(out, ch)
		}
	}
	return out
}

// deliverAll delivers alert to every channel and returns a single summary
// result: "ok" when all succeeded, otherwise "failed" with an error naming each
// channel that failed. Channels are delivered concurrently so one slow channel
// (up to timeout + retry) cannot exhaust the shared delivery budget and starve
// the rest. Callers pass the already-enabled channels.
func (n *notifier) deliverAll(ctx context.Context, channels []config.AlertChannel, alert models.Alert, skip <-chan struct{}) models.DeliveryResult {
	outcomes := make([]channelOutcome, len(channels))
	var wg sync.WaitGroup
	for i, ch := range channels {
		wg.Add(1)
		go func(i int, ch config.AlertChannel) {
			defer wg.Done()
			outcomes[i] = channelOutcome{channel: ch, result: n.deliver(ctx, ch, alert, skip)}
		}(i, ch)
	}
	wg.Wait()
	return summarizeDeliveries(outcomes)
}

// summarizeDeliveries collapses per-channel outcomes into one history result.
// The HTTP status is carried through only when a single channel was attempted,
// where it is unambiguous.
func summarizeDeliveries(outcomes []channelOutcome) models.DeliveryResult {
	var failed []string
	for _, o := range outcomes {
		if o.result.Status == "ok" {
			continue
		}
		detail := o.result.Error
		if detail == "" {
			detail = o.result.Status
		}
		failed = append(failed, channelLabel(o.channel)+": "+detail)
	}
	result := models.DeliveryResult{Status: "ok"}
	if len(failed) > 0 {
		result = models.DeliveryResult{Status: "failed", Error: strings.Join(failed, "; ")}
	}
	if len(outcomes) == 1 {
		result.HTTPStatus = outcomes[0].result.HTTPStatus
	}
	return result
}

func channelLabel(ch config.AlertChannel) string {
	if ch.Name != "" {
		return ch.Name
	}
	return ch.Type
}

// deliver sends the alert to one channel. Network errors and 5xx responses are
// retried once after retryDelay; other statuses are permanent. Closing skip
// (shutdown) or ctx aborts the retry wait and returns the first result.
func (n *notifier) deliver(ctx context.Context, ch config.AlertChannel, alert models.Alert, skip <-chan struct{}) models.DeliveryResult {
	send := func() (models.DeliveryResult, bool) { return sendEmail(ctx, ch, alert) }
	if ch.Type != "email" {
		spec, err := buildRequestSpec(ch, alert)
		if err != nil {
			return models.DeliveryResult{Status: "failed", Error: err.Error()}
		}
		send = func() (models.DeliveryResult, bool) { return n.attempt(ctx, spec) }
	}

	result, retryable := send()
	if !retryable {
		return result
	}
	select {
	case <-time.After(n.retryDelay):
	case <-skip:
		return result
	case <-ctx.Done():
		return result
	}
	result, _ = send()
	return result
}

// attempt performs one request and classifies the outcome. The bool reports
// whether the failure is retryable (network error or 5xx).
func (n *notifier) attempt(ctx context.Context, spec requestSpec) (models.DeliveryResult, bool) {
	req, err := http.NewRequestWithContext(ctx, spec.method, spec.url, bytes.NewReader(spec.body))
	if err != nil {
		return models.DeliveryResult{Status: "failed", Error: redactSecret(err.Error(), spec.secret)}, false
	}
	for k, v := range spec.headers {
		req.Header.Set(k, v)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return models.DeliveryResult{Status: "failed", Error: redactSecret(err.Error(), spec.secret)}, true
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
	}()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return models.DeliveryResult{Status: "ok", HTTPStatus: resp.StatusCode}, false
	}
	result := models.DeliveryResult{
		Status:     "failed",
		HTTPStatus: resp.StatusCode,
		Error:      fmt.Sprintf("channel returned %s", resp.Status),
	}
	return result, resp.StatusCode >= 500
}
