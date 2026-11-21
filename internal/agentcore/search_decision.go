package agentcore

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The read-first rule's record (docs/design/semantic-layer-2026-11-20.md
// section 4.3). Each web search the guest sends carries what the run held and
// had tried to read when it asked (yaegikernel.SearchStartPayload), counted
// before the send. The host takes it from the payload before it opens the
// stream and records it as "decision" on the search_start row of the cell's
// desk.cell event (journal cut R4: one durable record per cell, not one per
// search); the search's end is the standalone "search_done" event with
// pages_returned. So a search never counts its own results as held (the failure
// that stopped the side measurement three times,
// docs/problems/search-sufficiency-measurement-three-reviews-no-landing-2026-11-20.md).
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

// searchDecisionOf is the decision of the search about to be started, as it
// rides the search_start row of the cell's desk.cell event: the verb, the counts
// the guest reported, and the reason SearchFresh was given. Nil when the payload
// does not decode.
func searchDecisionOf(payload json.RawMessage) map[string]any {
	var p yaegikernel.SearchStartPayload
	if json.Unmarshal(payload, &p) != nil {
		return nil
	}
	d := map[string]any{
		"search_verb": p.Verb, "earlier_searches": p.Earlier, "pages_returned": p.Returned, "pages_tried": p.Tried,
		"pages_read": p.Read, "outlets_read": p.Outlets, "held_brief": p.HeldBrief, "held_tried": p.HeldTried,
	}
	if why := strings.TrimSpace(p.FreshWhy); why != "" {
		d["fresh_why"] = why
	}
	return d
}

// logSearchFresh writes one console line per SearchFresh send with the reason
// the run gave, so the proof script (scripts/status-search-reads) can count the
// reasons by distinct text and boilerplate shows. The reason is the run's own
// sentence, never the query.
func logSearchFresh(runID string, decision map[string]any) {
	if why, _ := decision["fresh_why"].(string); why != "" {
		log.Printf("runtime: search fresh run=%s why=%q", runID, why)
	}
}
