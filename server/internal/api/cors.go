package api

import (
	"os"
	"strings"
)

// corsAllowedOrigins returns the allowed CORS origins from the
// CORS_ALLOWED_ORIGINS env var (comma-separated). When unset, it defaults to
// the Vite dev server origins — production serves the SPA from the same
// binary and needs no cross-origin access.
func corsAllowedOrigins() []string {
	if raw := os.Getenv("CORS_ALLOWED_ORIGINS"); raw != "" {
		origins := []string{}
		for _, o := range strings.Split(raw, ",") {
			if o = strings.TrimSpace(o); o != "" {
				origins = append(origins, o)
			}
		}
		if len(origins) > 0 {
			return origins
		}
	}
	return []string{"http://localhost:5173", "http://127.0.0.1:5173"}
}
