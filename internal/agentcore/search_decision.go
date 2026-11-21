package agentcore

import (
	"encoding/json"
	"log"
	"strings"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The read-first rule's record (docs/design/semantic-layer-2026-11-20.md
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
		"search_verb": p.Verb, "earlier_searches": p.Earlier, "earlier_pages_new": p.Returned, "pages_tried": p.Tried,
		"pages_read": p.Read, "outlets_read": p.Outlets, "held_brief": p.HeldBrief, "held_tried": p.HeldTried,
	}
	if why := strings.TrimSpace(p.FreshWhy); why != "" {
		d["fresh_why"] = why
	}
	return d
}

// logSearchFresh writes one console line per SearchFresh send with the reason
// the run gave, so the proof script (the status-search-reads script) can count the
// reasons by distinct text and boilerplate shows. The reason is the run's own
// sentence, never the query.
func logSearchFresh(runID string, decision map[string]any) {
	if why, _ := decision["fresh_why"].(string); why != "" {
		log.Printf("runtime: search fresh run=%s why=%q", runID, why)
	}
}

// treeRead is what one research tree (the root run and every choir.Research
// child: they share the run id) has tried to read and been returned. Each
// session of the tree has its own guest store, so without this a child's first
// search is judged alone: parent plus four children are five independent first
// searches and twenty re-reads of the same five pages. The host sees every read
// and every search of the tree and hands the union to each cell's frame
// (yaegikernel.SessionFrame.Tree).
type treeRead struct {
	mu     sync.Mutex
	tried  map[string]bool
	seen   map[string]bool
	order  []string
	window []yaegikernel.HeldPage
}

// treeWindowPages is how many pages new to the tree a search contributes to the
// window: the same five the guest's rule looks at.
const treeWindowPages = 5

func (rt *Runtime) treeReadFor(runID string) *treeRead {
	if rt == nil || strings.TrimSpace(runID) == "" {
		return nil
	}
	v, _ := rt.treeRead.LoadOrStore(runID, &treeRead{tried: map[string]bool{}, seen: map[string]bool{}})
	return v.(*treeRead)
}

// noteTreeTried records that a session of the run's tree is about to read url.
func (rt *Runtime) noteTreeTried(runID, url string) {
	t := rt.treeReadFor(runID)
	key := yaegikernel.PageKey(url)
	if t == nil || key == "" {
		return
	}
	t.mu.Lock()
	if !t.tried[key] {
		t.tried[key] = true
		t.order = append(t.order, key)
	}
	t.mu.Unlock()
}

// noteTreeRead is called before the host serves a read of any session of the
// tree: a Read of a url, or a ReadCapture of a held page by its capture id.
func (rt *Runtime) noteTreeRead(runID string, action yaegikernel.BrokerAction, payload json.RawMessage) {
	switch action {
	case yaegikernel.ActionRead:
		var p yaegikernel.ReadPayload
		if json.Unmarshal(payload, &p) == nil {
			rt.noteTreeTried(runID, p.URL)
		}
	case yaegikernel.ActionReadCapture:
		var p yaegikernel.ReadCapturePayload
		if json.Unmarshal(payload, &p) != nil {
			return
		}
		for _, h := range rt.briefHeldFor(runID) {
			if h.CaptureID != "" && h.CaptureID == strings.TrimSpace(p.CaptureID) {
				rt.noteTreeTried(runID, h.URL)
			}
		}
	}
}

// noteTreeSearch records the pages a finished search of the tree returned: the
// first treeWindowPages not yet returned to the tree join the window.
func (rt *Runtime) noteTreeSearch(runID string, urls []string) {
	t := rt.treeReadFor(runID)
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	added := 0
	for _, u := range urls {
		key := yaegikernel.PageKey(u)
		if key == "" || t.seen[key] {
			continue
		}
		t.seen[key] = true
		if added < treeWindowPages {
			t.window = append(t.window, yaegikernel.HeldPage{URL: u})
			added++
		}
	}
}

// treeStateFor is the tree's state for a cell's frame, or nil when empty.
func (rt *Runtime) treeStateFor(runID string) *yaegikernel.TreeState {
	if rt == nil || strings.TrimSpace(runID) == "" {
		return nil
	}
	v, ok := rt.treeRead.Load(runID)
	if !ok {
		return nil
	}
	t := v.(*treeRead)
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.order) == 0 && len(t.window) == 0 {
		return nil
	}
	return &yaegikernel.TreeState{Tried: append([]string(nil), t.order...), Window: append([]yaegikernel.HeldPage(nil), t.window...)}
}
