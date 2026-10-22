package gateway

import (
	"context"
	"log"
	"net/http"
	"strings"
)

// SEC-16: a host job that scores with Jev reaches it through the gateway, so
// the gateway is the one holder and meter of the OpenRouter key. The route is
// a host-internal route, not a computer route: it takes the host-internal
// credential (loopback, the marker and hostauth), never a computer's bearer,
// and the caller's label comes from the gateway's own table.

// hostCallerHeader names the host job making the call.
const hostCallerHeader = "X-Choir-Host-Caller"

// hostJudgmentCallers maps a caller name to the label its receipts and logs
// carry. A caller is added here in a reviewed change, never by a request.
var hostJudgmentCallers = map[string]string{
	"front-importance": "host:front-importance",
}

// HandleHostJudgment serves POST /provider/v1/host/judgments: the pinned Jev
// Decisions transport for an allowlisted host job. It never accepts a model
// or a scope, and it answers with the exact upstream body, like HandleJudgment.
func (h *Handler) HandleHostJudgment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeGatewayJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
		return
	}
	if !isAuthorizedCredentialCaller(r) {
		writeGatewayJSON(w, http.StatusForbidden, ErrorResponse{Error: "operator route"})
		return
	}
	label, ok := hostJudgmentCallers[strings.TrimSpace(r.Header.Get(hostCallerHeader))]
	if !ok {
		writeGatewayJSON(w, http.StatusForbidden, ErrorResponse{Error: "unknown host caller"})
		return
	}
	if h.jevTransport == nil || !h.jevTransport.Enabled() {
		writeGatewayJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: "Jev judgments are disabled"})
		return
	}
	var request JudgmentRequest
	if err := h.decodeBody(w, r, bodyRouteJudgment, label, &request, false); err != nil {
		if isBodyTooLarge(err) {
			writeGatewayJSON(w, http.StatusRequestEntityTooLarge, ErrorResponse{Error: err.Error()})
			return
		}
		writeGatewayJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid judgment request"})
		return
	}
	if strings.TrimSpace(request.Scope) != "" {
		writeGatewayJSON(w, http.StatusBadRequest, ErrorResponse{Error: "scope is not accepted on the host route"})
		return
	}
	if err := validateJudgmentRequest(request); err != nil {
		writeGatewayJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), jevTransportTimeout)
	defer cancel()
	response, err := h.jevTransport.Decide(ctx, label, request)
	if err != nil {
		log.Printf("gateway: host judgment from %s failed (questions=%d)", label, len(request.Questions))
		if _, disabled := err.(jevTransportDisabledError); disabled {
			writeGatewayJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: "Jev judgments are disabled"})
			return
		}
		writeGatewayJSON(w, http.StatusBadGateway, ErrorResponse{Error: "Jev judgment provider unavailable"})
		return
	}
	log.Printf("gateway: host judgment from %s served (questions=%d bytes=%d)", label, len(request.Questions), len(response))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(response)
}
