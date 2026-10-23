package autoputer

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/server"
)

// operationStampHealWriteDeadline lifts the short global server write deadline
// for this route: it runs up to two replay probes before it answers.
const operationStampHealWriteDeadline = 20 * time.Minute

// stampHealer is the runtime's operator heal for historic intent-error stamps.
type stampHealer interface {
	HealSelfDevelopmentIntentStamps(ctx context.Context, computerID string, apply, verify bool) (agentcore.SelfDevelopmentStampHealReport, error)
}

// operationStampHealHandler is the explicit, per-computer operator act that
// makes a computer replay-equivalent again after it stamped
// self_development_operations.last_intent_error before that write went on the
// tape (docs/problems/selfdev-operations-replay-not-equivalent-2026-10-23.md).
//
//	GET  /internal/self-development/operation-stamp-heal              dry run: what would be healed
//	POST /internal/self-development/operation-stamp-heal              apply one state-preserving batch, then re-probe
//	POST /internal/self-development/operation-stamp-heal?verify=0     apply without the second probe
//
// Both methods run the replay probe (about 20 s on a small tape; a full-chain
// replay on a computer with no published base). A refused heal (a row moved
// since the plan, or another heal is running) is 409 with nothing appended.
// Only the host peer may call it: the guest's own children (loopback) are
// refused. It is never a boot step.
func operationStampHealHandler(healer stampHealer, computerID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !server.HostPeerCaller(r) {
			http.Error(w, "host peer required", http.StatusForbidden)
			return
		}
		var apply bool
		switch r.Method {
		case http.MethodGet:
		case http.MethodPost:
			apply = true
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(operationStampHealWriteDeadline))
		verify := r.URL.Query().Get("verify") != "0"
		report, err := healer.HealSelfDevelopmentIntentStamps(r.Context(), computerID, apply, verify)
		switch {
		case errors.Is(err, selfdev.ErrConflict), errors.Is(err, agentcore.ErrStampHealRunning):
			http.Error(w, err.Error(), http.StatusConflict)
		case errors.Is(err, selfdev.ErrStampHealUnbound), errors.Is(err, agentcore.ErrReplayCompletenessUnavailable):
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
		case err != nil:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		default:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(report)
		}
	}
}
