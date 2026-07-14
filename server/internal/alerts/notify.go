package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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
	// out of band (ntfy Title header, Gotify title field).
	alertTitle = "LogDeck alert"
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

// requestSpec is the fully-resolved HTTP request for one channel delivery.
type requestSpec struct {
	method  string
	url     string
	headers map[string]string
	body    []byte
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

// buildRequestSpec resolves the HTTP request for delivering alert to channel.
// The correct request shape per type is the core of channel support:
//   - webhook:  JSON {source,version,text,content,alert} POSTed to the URL.
//   - ntfy:     the alert text as a plain-text body, with a Title header.
//   - gotify:   JSON {title,message,priority} POSTed to <URL>/message?token=.
//   - telegram: JSON {chat_id,text} POSTed to the Bot API sendMessage method.
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
		endpoint := strings.TrimRight(ch.URL, "/") + "/message?token=" + url.QueryEscape(ch.Token)
		return requestSpec{
			method:  http.MethodPost,
			url:     endpoint,
			headers: map[string]string{"Content-Type": "application/json"},
			body:    body,
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
		}, nil
	default:
		return requestSpec{}, fmt.Errorf("unknown channel type %q", ch.Type)
	}
}

// channelOutcome pairs a channel with the result of delivering to it.
type channelOutcome struct {
	channel config.AlertChannel
	result  models.DeliveryResult
}

// enabledChannels returns the enabled channels in order.
func enabledChannels(channels []config.AlertChannel) []config.AlertChannel {
	out := make([]config.AlertChannel, 0, len(channels))
	for _, ch := range channels {
		if ch.Enabled {
			out = append(out, ch)
		}
	}
	return out
}

// deliverAll delivers alert to every channel in order and returns a single
// summary result: "ok" when all succeeded, otherwise "failed" with an error
// naming each channel that failed. Callers pass the already-enabled channels.
func (n *notifier) deliverAll(ctx context.Context, channels []config.AlertChannel, alert models.Alert, skip <-chan struct{}) models.DeliveryResult {
	outcomes := make([]channelOutcome, 0, len(channels))
	for _, ch := range channels {
		outcomes = append(outcomes, channelOutcome{channel: ch, result: n.deliver(ctx, ch, alert, skip)})
	}
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

// channelLabel names a channel for a delivery summary: its name if set, else
// its type.
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
	spec, err := buildRequestSpec(ch, alert)
	if err != nil {
		return models.DeliveryResult{Status: "failed", Error: err.Error()}
	}

	result, retryable := n.attempt(ctx, spec)
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
	result, _ = n.attempt(ctx, spec)
	return result
}

// attempt performs one request and classifies the outcome. The bool reports
// whether the failure is retryable (network error or 5xx).
func (n *notifier) attempt(ctx context.Context, spec requestSpec) (models.DeliveryResult, bool) {
	req, err := http.NewRequestWithContext(ctx, spec.method, spec.url, bytes.NewReader(spec.body))
	if err != nil {
		return models.DeliveryResult{Status: "failed", Error: err.Error()}, false
	}
	for k, v := range spec.headers {
		req.Header.Set(k, v)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return models.DeliveryResult{Status: "failed", Error: err.Error()}, true
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
