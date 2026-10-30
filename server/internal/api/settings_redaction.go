package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/AmoabaKelvin/logdeck/internal/redact"
)

// maxRedactionSample bounds a preview sample. Lines are redacted one by one,
// so this is about a few thousand lines of text.
const maxRedactionSample = 256 << 10

type redactionResponse struct {
	config.RedactionConfig
	// Catalog describes the built-in patterns the config can enable.
	Catalog []redact.Builtin `json:"catalog"`
	// DefaultReplacement applies when Replacement is empty.
	DefaultReplacement string `json:"defaultReplacement"`
}

// GetRedaction handles GET /api/v1/settings/redaction.
func (ar *APIRouter) GetRedaction(w http.ResponseWriter, r *http.Request) {
	cfg := ar.manager.Redaction()
	if cfg.Builtins == nil {
		// Never saved: offer the defaults, still disabled.
		cfg.Builtins = redact.DefaultBuiltins()
	}
	if cfg.Rules == nil {
		cfg.Rules = []config.RedactionRule{}
	}
	WriteJsonResponse(w, http.StatusOK, redactionResponse{
		RedactionConfig:    cfg,
		Catalog:            redact.Catalog(),
		DefaultReplacement: redact.DefaultReplacement,
	})
}

// UpdateRedaction handles PUT /api/v1/settings/redaction. It replaces the
// whole redaction config. Rules are compiled before anything is saved, so an
// invalid one is rejected with the reason and the running rules stay as they
// were. Once saved, the new rules apply from the next line every open stream
// reads.
func (ar *APIRouter) UpdateRedaction(w http.ResponseWriter, r *http.Request) {
	var req config.RedactionConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Builtins == nil {
		req.Builtins = []string{} // saved, so the defaults no longer apply
	}

	ar.redactionMu.Lock()
	defer ar.redactionMu.Unlock()

	set, err := redact.Compile(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := ar.manager.SetRedaction(req); err != nil {
		http.Error(w, err.Error(), settingsErrorStatus(err))
		return
	}
	redact.Activate(set)

	WriteJsonResponse(w, http.StatusOK, map[string]any{"message": "redaction settings updated"})
}

// PreviewRedaction handles POST /api/v1/settings/redaction/preview. It runs
// the given, unsaved config over the sample as if redaction were enabled, and
// ignores targeting. Neither the config nor the sample is stored or logged.
func (ar *APIRouter) PreviewRedaction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Config config.RedactionConfig `json:"config"`
		Sample string                 `json:"sample"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRedactionSample+64<<10)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if len(req.Sample) > maxRedactionSample {
		http.Error(w, fmt.Sprintf("sample is larger than %d KB", maxRedactionSample>>10), http.StatusBadRequest)
		return
	}

	req.Config.Enabled = true
	set, err := redact.Compile(req.Config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	WriteJsonResponse(w, http.StatusOK, map[string]string{"output": set.Preview(req.Sample)})
}
