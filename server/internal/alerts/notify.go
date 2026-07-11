package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/models"
)

const (
	deliverTimeout = 10 * time.Second
	retryDelay     = 5 * time.Second
)

// webhookPayload is the JSON body POSTed to the configured webhook. Text and
// Content carry the same human-readable summary so common receivers (Slack,
// Discord, Mattermost) render it without templates.
type webhookPayload struct {
	Source  string       `json:"source"`
	Version int          `json:"version"`
	Text    string       `json:"text"`
	Content string       `json:"content"`
	Alert   models.Alert `json:"alert"`
}

// notifier delivers alert payloads to the configured webhook URL and reports
// each attempt as a models.DeliveryResult.
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

// deliver POSTs the alert to url. Network errors and 5xx responses are
// retried once after retryDelay; other statuses are permanent. Closing skip
// (shutdown) or ctx aborts the retry wait and returns the first result.
func (n *notifier) deliver(ctx context.Context, url string, alert models.Alert, skip <-chan struct{}) models.DeliveryResult {
	body, err := json.Marshal(webhookPayload{
		Source:  "logdeck",
		Version: 1,
		Text:    alertText(alert),
		Content: alertText(alert),
		Alert:   alert,
	})
	if err != nil {
		return models.DeliveryResult{Status: "failed", Error: fmt.Sprintf("failed to marshal payload: %v", err)}
	}

	result, retryable := n.attempt(ctx, url, body)
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
	result, _ = n.attempt(ctx, url, body)
	return result
}

// attempt performs one POST and classifies the outcome. The bool reports
// whether the failure is retryable (network error or 5xx).
func (n *notifier) attempt(ctx context.Context, url string, body []byte) (models.DeliveryResult, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return models.DeliveryResult{Status: "failed", Error: err.Error()}, false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return models.DeliveryResult{Status: "failed", Error: err.Error()}, true
	}
	defer func() {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
	}()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return models.DeliveryResult{Status: "ok", HTTPStatus: resp.StatusCode}, false
	}
	result := models.DeliveryResult{
		Status:     "failed",
		HTTPStatus: resp.StatusCode,
		Error:      fmt.Sprintf("webhook returned %s", resp.Status),
	}
	return result, resp.StatusCode >= 500
}
