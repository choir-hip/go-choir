package agentcore

// The platform operator's acts on this computer, seen from the guest
// (docs/design/platform-operator-account-2026-11-06.md sections 5 and 9). The
// proxy's operator gate forwards an act with the operator's account id in
// server.PlatformOperatorHeader. The guest believes that marker only from the
// host peer, records a receipt on the computer's tape BEFORE the act lands, and
// refuses an act whose receipt cannot be recorded.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// OperatorReceiptsHeader is the guest's statement to the proxy that a marked
// request is recorded on its tape. A guest from before this build says nothing
// and the proxy writes nothing to it.
const OperatorReceiptsHeader = "X-Choir-Operator-Receipts"

// operatorRestartReceiptRoute is the guest route that records the receipt for
// an operator's restart request. It records and does nothing else.
const OperatorRestartReceiptRoute = "/api/platform/operator/restart-receipt"

// recordOperatorReceipt appends the receipt event and returns its id.
func (rt *Runtime) recordOperatorReceipt(ctx context.Context, ownerID string, kind types.EventKind, payload map[string]any) (string, error) {
	rec, err := rt.EmitProductEvent(ctx, ownerID, types.PrimaryDesktopID, kind, payload)
	if err != nil {
		return "", err
	}
	return rec.EventID, nil
}

func (h *APIHandler) recordReceipt(ctx context.Context, ownerID string, kind types.EventKind, payload map[string]any) (string, error) {
	if h.operatorReceipt != nil {
		return h.operatorReceipt(ctx, ownerID, kind, payload)
	}
	return h.rt.recordOperatorReceipt(ctx, ownerID, kind, payload)
}

// operatorMarker reads the marker of a request. present is false for an
// ordinary owner request. A marker that is present but not believable has
// already been answered with a refusal (ok false).
func operatorMarker(w http.ResponseWriter, r *http.Request) (operator string, present, ok bool) {
	id, present, err := server.PlatformOperator(r)
	if !present {
		return "", false, true
	}
	if err != nil {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "access denied"})
		return "", true, false
	}
	return id, true, true
}

// operatorScheduleBody is the only body an operator's start may carry. A feed
// URL (a feed the registry does not list) and a fixture run are the owner's own
// acts, and any other field is refused.
type operatorScheduleBody struct {
	SourceID        string `json:"source_id"`
	IntervalMinutes int    `json:"interval_minutes,omitempty"`
	MaxItems        int    `json:"max_items,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

const operatorReasonMax = 280

// operatorSchedule handles a marked schedule request. It returns true when it
// answered the request (including every refusal); false means the request is an
// ordinary owner request and the caller carries on.
func (h *APIHandler) operatorSchedule(w http.ResponseWriter, r *http.Request, ownerID string) bool {
	operator, present, ok := operatorMarker(w, r)
	if !ok {
		return true
	}
	if !present {
		return false
	}
	w.Header().Set(OperatorReceiptsHeader, "1")
	rt := h.rt
	switch r.Method {
	case http.MethodGet:
		return false // a read: the ordinary list, no receipt
	case http.MethodPost:
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid body"})
			return true
		}
		var body operatorScheduleBody
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&body); err != nil || dec.More() || strings.TrimSpace(body.SourceID) == "" || len(body.Reason) > operatorReasonMax {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "an operator schedule start takes source_id, interval_minutes, max_items and reason only"})
			return true
		}
		receipt, err := h.recordReceipt(r.Context(), ownerID, types.EventPlatformComputerSchedule, map[string]any{
			"operator_account_id": operator, "act": "schedule.start", "source_id": body.SourceID,
			"interval_minutes": body.IntervalMinutes, "max_items": body.MaxItems, "reason": body.Reason,
		})
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "the operator receipt could not be recorded; nothing was changed"})
			return true
		}
		sched, err := rt.startSourceSchedule(r.Context(), ownerID, rt.TextureComputerID(), body.SourceID,
			time.Duration(body.IntervalMinutes)*time.Minute, body.MaxItems, false, ownerActID(r))
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "receipt": receipt})
			return true
		}
		writeAPIJSON(w, http.StatusOK, map[string]any{"schedule": sched, "receipt": receipt})
		return true
	case http.MethodDelete:
		q := r.URL.Query()
		for k := range q {
			if k != "source_id" && k != "reason" {
				writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "an operator schedule stop takes source_id and reason only"})
				return true
			}
		}
		sourceID, reason := q.Get("source_id"), q.Get("reason")
		if strings.TrimSpace(sourceID) == "" || len(reason) > operatorReasonMax {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "source_id is required; reason is at most 280 bytes"})
			return true
		}
		receipt, err := h.recordReceipt(r.Context(), ownerID, types.EventPlatformComputerSchedule, map[string]any{
			"operator_account_id": operator, "act": "schedule.stop", "source_id": sourceID, "reason": reason,
		})
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "the operator receipt could not be recorded; nothing was changed"})
			return true
		}
		sched, err := rt.StopSourceSchedule(r.Context(), ownerID, sourceID, reason)
		if errors.Is(err, store.ErrNotFound) {
			writeAPIJSON(w, http.StatusNotFound, map[string]any{"error": "no schedule for that source", "receipt": receipt})
			return true
		}
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "receipt": receipt})
			return true
		}
		writeAPIJSON(w, http.StatusOK, map[string]any{"schedule": sched, "receipt": receipt})
		return true
	}
	writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	return true
}

// HandleOperatorRestartReceipt records the receipt for an operator's restart
// request and does nothing else; the restart itself is vmctl's planned,
// marker-backed refresh, asked by the proxy after this answers 200.
func (h *APIHandler) HandleOperatorRestartReceipt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	operator, present, ok := operatorMarker(w, r)
	if !ok {
		return
	}
	if !present {
		// Only the operator gate asks for this receipt.
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "access denied"})
		return
	}
	w.Header().Set(OperatorReceiptsHeader, "1")
	var body struct {
		Reason string `json:"reason"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil || strings.TrimSpace(body.Reason) == "" || len(body.Reason) > operatorReasonMax {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "a restart needs a reason of at most 280 bytes"})
		return
	}
	receipt, err := h.recordReceipt(r.Context(), ownerID, types.EventPlatformComputerRestart, map[string]any{
		"operator_account_id": operator, "act": "restart.requested", "reason": body.Reason,
	})
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "the operator receipt could not be recorded; nothing was restarted"})
		return
	}
	writeAPIJSON(w, http.StatusOK, map[string]any{"receipt": receipt})
}
