package maild

import (
	"net/http"
	"strings"
)

type healthResponse struct {
	Status                    string     `json:"status"`
	Service                   string     `json:"service"`
	PrimaryDomain             string     `json:"primary_domain"`
	ResendAPIKeyConfigured    bool       `json:"resend_api_key_configured"`
	WebhookSecretConfigured   bool       `json:"webhook_secret_configured"`
	RootOwnerIDConfigured     bool       `json:"root_owner_id_configured"`
	StorageRootConfigured     bool       `json:"storage_root_configured"`
	WebhookMaxBytesConfigured bool       `json:"webhook_max_bytes_configured"`
	Stats                     StoreStats `json:"stats"`
}

// HandleHealth reports safe operational state for maild without exposing
// provider secrets, owner ids, paths, message bodies, or attachment metadata.
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stats, err := h.store.Stats(r.Context())
	status := "ok"
	httpStatus := http.StatusOK
	if err != nil {
		status = "degraded"
		httpStatus = http.StatusServiceUnavailable
	}
	writeJSON(w, httpStatus, healthResponse{
		Status:                    status,
		Service:                   "maild",
		PrimaryDomain:             h.cfg.PrimaryDomain,
		ResendAPIKeyConfigured:    strings.TrimSpace(h.cfg.ResendAPIKey) != "",
		WebhookSecretConfigured:   strings.TrimSpace(h.cfg.WebhookSecret) != "",
		RootOwnerIDConfigured:     strings.TrimSpace(h.cfg.RootOwnerID) != "" && h.cfg.RootOwnerID != DefaultRootOwnerID,
		StorageRootConfigured:     strings.TrimSpace(h.cfg.StorageRoot) != "",
		WebhookMaxBytesConfigured: h.cfg.WebhookMaxBytes > 0,
		Stats:                     stats,
	})
}
