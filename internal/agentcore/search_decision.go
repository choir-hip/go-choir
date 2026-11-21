package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The read-first rule's record (docs/design/semantic-layer-2026-11-20.md
// section 4.3). Each web search the guest sends carries what the run held and
// had tried to read when it asked (yaegikernel.SearchStartPayload). The host
// writes it on the run's tape BEFORE it opens the stream, as a desk egress
// event with verb "search_decision", and the search's end as the same
// "search_done" event as before with pages_returned: so a search never counts
// its own results as held (the failure that stopped the side measurement three
// times, docs/problems/search-sufficiency-measurement-three-reviews-no-landing-2026-11-20.md).
// Counts, the verb and the reason SearchFresh was given: never the query. The
// numbers are the guest's own report: the guest is the code the paper runs, and
// the decision is a record, not a gate (the gate is the guest's refusal).

// noteBriefHeld keeps the pages the run's corpus brief delivered, to hand to its
// cells as search zero.
func (rt *Runtime) noteBriefHeld(runID string, held []yaegikernel.HeldPage) {
	if rt == nil || strings.TrimSpace(runID) == "" || len(held) == 0 {
		return
	}
	rt.briefHeld.Store(runID, append([]yaegikernel.HeldPage(nil), held...))
}

// briefHeldFor returns the run's search zero, or nil.
func (rt *Runtime) briefHeldFor(runID string) []yaegikernel.HeldPage {
	if rt == nil || strings.TrimSpace(runID) == "" {
		return nil
	}
	v, ok := rt.briefHeld.Load(runID)
	if !ok {
		return nil
	}
	held, _ := v.([]yaegikernel.HeldPage)
	return held
}

// searchDecisionEvent is the payload of the "search_decision" desk event.
func searchDecisionEvent(p yaegikernel.SearchStartPayload, runID, sessionLabel string, pid int, at time.Time) map[string]any {
	ev := map[string]any{
		"verb": "search_decision", "run_id": runID, "desk_session": sessionLabel, "worker_pid": pid,
		"decided_at": at.UTC(), "search_verb": p.Verb,
		"earlier_searches": p.Earlier, "pages_returned": p.Returned, "pages_tried": p.Tried,
		"pages_read": p.Read, "outlets_read": p.Outlets, "held_brief": p.HeldBrief, "held_tried": p.HeldTried,
	}
	if why := strings.TrimSpace(p.FreshWhy); why != "" {
		ev["fresh_why"] = why
	}
	return ev
}

// recordSearchDecision writes the decision of the search about to be started.
func (rt *Runtime) recordSearchDecision(ctx context.Context, exec toolregistry.ExecutionContext, payload json.RawMessage, sessionLabel string, pid int) {
	var p yaegikernel.SearchStartPayload
	if json.Unmarshal(payload, &p) != nil {
		return
	}
	rt.recordDeskEvent(ctx, exec, types.EventDeskEgress, searchDecisionEvent(p, exec.RunID, sessionLabel, pid, time.Now()))
}
