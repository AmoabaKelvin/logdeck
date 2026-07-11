package alerts

import "net/http"

// notifier delivers alert payloads to the configured webhook URL and reports
// each attempt as a models.DeliveryResult. The webhook delivery territory
// fills in the implementation.
type notifier struct {
	client *http.Client
}
