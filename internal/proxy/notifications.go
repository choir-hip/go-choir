package proxy

import "net/http"

func (h *Handler) HandleNotificationAPI(w http.ResponseWriter, r *http.Request) {
	h.forwardMaildAuthenticated(w, r)
}
