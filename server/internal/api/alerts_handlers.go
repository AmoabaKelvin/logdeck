package api

import "net/http"

// Alert endpoints are registered ahead of the alerting territories; every
// handler returns 501 until its implementation lands.

func alertsNotImplemented(w http.ResponseWriter) {
	WriteJsonResponse(w, http.StatusNotImplemented, map[string]string{"error": "not implemented"})
}

// ListAlertRules handles GET /api/v1/alerts/rules.
func (ar *APIRouter) ListAlertRules(w http.ResponseWriter, r *http.Request) {
	alertsNotImplemented(w)
}

// CreateAlertRule handles POST /api/v1/alerts/rules.
func (ar *APIRouter) CreateAlertRule(w http.ResponseWriter, r *http.Request) {
	alertsNotImplemented(w)
}

// UpdateAlertRule handles PUT /api/v1/alerts/rules/{id}.
func (ar *APIRouter) UpdateAlertRule(w http.ResponseWriter, r *http.Request) {
	alertsNotImplemented(w)
}

// DeleteAlertRule handles DELETE /api/v1/alerts/rules/{id}.
func (ar *APIRouter) DeleteAlertRule(w http.ResponseWriter, r *http.Request) {
	alertsNotImplemented(w)
}

// GetAlertsWebhook handles GET /api/v1/alerts/webhook.
func (ar *APIRouter) GetAlertsWebhook(w http.ResponseWriter, r *http.Request) {
	alertsNotImplemented(w)
}

// UpdateAlertsWebhook handles PUT /api/v1/alerts/webhook.
func (ar *APIRouter) UpdateAlertsWebhook(w http.ResponseWriter, r *http.Request) {
	alertsNotImplemented(w)
}

// TestAlertsWebhook handles POST /api/v1/alerts/test.
func (ar *APIRouter) TestAlertsWebhook(w http.ResponseWriter, r *http.Request) {
	alertsNotImplemented(w)
}

// GetAlertHistory handles GET /api/v1/alerts/history.
func (ar *APIRouter) GetAlertHistory(w http.ResponseWriter, r *http.Request) {
	alertsNotImplemented(w)
}

// ClearAlertHistory handles DELETE /api/v1/alerts/history.
func (ar *APIRouter) ClearAlertHistory(w http.ResponseWriter, r *http.Request) {
	alertsNotImplemented(w)
}
