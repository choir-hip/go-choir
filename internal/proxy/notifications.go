package proxy

import (
	"net/http"
	"strings"
)

func (h *Handler) HandleNotificationAPI(w http.ResponseWriter, r *http.Request) {
	h.forwardMaildAuthenticated(w, r)
}
func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
