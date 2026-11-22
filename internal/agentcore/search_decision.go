package agentcore

import (
	"encoding/json"

	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The search decision's record (docs/design/semantic-layer-2026-11-20.md
// section 4.3). Each web search the guest sends carries what the run held and
// had tried to read when it asked (yaegikernel.SearchStartPayload), counted
// before the send. The host takes it from the payload before it opens the
// stream and records it as "decision" on the search_start row of the cell's
// desk.cell event (journal cut R4: one durable record per cell, not one per
// search); the search's end is the standalone "search_done" event with
// pages_returned (this search's own results; the decision's earlier_pages_new
// is the earlier searches'). So a search never counts its own results as held (the failure
// that stopped the side measurement three times,
// docs/problems/search-sufficiency-measurement-three-reviews-no-landing-2026-11-20.md).
// Counts and the verb: never the query. The
// numbers are the guest's own report: the guest is the code the paper runs, and
// the decision is a record, not a gate (no search is refused for what the run holds).

// searchDecisionOf is the decision of the search about to be started, as it
// rides the search_start row of the cell's desk.cell event: the verb, the counts
// the guest reported, Nil when the payload
// does not decode.
func searchDecisionOf(payload json.RawMessage) map[string]any {
	var p yaegikernel.SearchStartPayload
	if json.Unmarshal(payload, &p) != nil {
		return nil
	}
	d := map[string]any{
		"search_verb": p.Verb, "earlier_searches": p.Earlier, "earlier_pages_new": p.Returned, "pages_tried": p.Tried,
		"pages_read": p.Read, "outlets_read": p.Outlets,
	}
	return d
}
