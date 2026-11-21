package agentcore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/toolregistry"
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

// briefHeldForSession is the search zero a cell's frame carries: the brief's
// pages for the run's root session only. The brief's text is delivered to the
// root run (research_brief.go); a choir.Research child is handed an objective
// and nothing of the brief, so it is not held by pages it was never shown (a
// child's execution context has no RunRecord, the root's has).
func (rt *Runtime) briefHeldForSession(exec toolregistry.ExecutionContext) []yaegikernel.HeldPage {
	if exec.RunRecord == nil {
		return nil
	}
	return rt.briefHeldFor(exec.RunID)
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

// logSearchFresh writes one console line per SearchFresh send with the reason's
// length and a short hash, so the proof script (the status-search-reads script)
// can count distinct reasons and boilerplate shows. The reason itself stays on the
// tape row (it is the run's own sentence and can carry a query), never the console.
func logSearchFresh(runID string, decision map[string]any) {
	if why, _ := decision["fresh_why"].(string); why != "" {
		sum := sha256.Sum256([]byte(why))
		log.Printf("runtime: search fresh run=%s why_len=%d why_hash=%s", runID, len([]rune(why)), hex.EncodeToString(sum[:4]))
	}
}

// treeRead is what one research tree (the root run and every choir.Research
// child: they share the run id) has tried to read. Each session of the tree has
// its own guest store, so without this a child's reading was invisible to its
// siblings: parent plus four children read the same five pages five times. The
// host sees every read of the tree and hands the tried set to each cell's frame
// (yaegikernel.SessionFrame.Tree) and, when a guest asks before it refuses, to
// the probe. What is shared is what has been TRIED, not what has been
// returned: a session's window is its own searches' pages, so a sibling's
// search never holds this one.
type treeRead struct {
	mu    sync.Mutex
	tried map[string]bool
	order []string
}

// openTree makes the run's tree the first time a cell of the run begins. Notes
// of reads and searches only ever find an open tree (treeReadFor): one that
// lands after the run's terminal observer has closed the tree is dropped, so
// the tree is not recreated for ever.
func (rt *Runtime) openTree(runID string) {
	if rt == nil || strings.TrimSpace(runID) == "" {
		return
	}
	if rt.treeIsClosed(runID) {
		return
	}
	if _, loaded := rt.treeRead.LoadOrStore(runID, &treeRead{tried: map[string]bool{}}); !loaded {
		rt.noteGateStateLostIfResumed(runID, rt.runStartedBeforeBoot(runID))
	}
}

// noteGateStateLostIfResumed says on the console that a run this process
// resumed (a planned restart parked it) begins with an empty gate: the tree's
// tried pages and the brief's pages are in memory, not rebuilt from the journal,
// so the first segment's reads and pages are not in it. It only loosens the
// gate. The proof script counts such runs.
func (rt *Runtime) noteGateStateLostIfResumed(runID string, resumed bool) {
	if resumed {
		log.Printf("runtime: search gate state lost at resume run=%s", runID)
	}
}

// treeClosedKeep is how long a closed run's id is remembered, so a cell that
// begins after the run's terminal observer does not recreate its tree.
const treeClosedKeep = time.Hour

func (rt *Runtime) treeIsClosed(runID string) bool {
	_, ok := rt.treeClosed.Load(runID)
	return ok
}

// closeTree forgets the run's tree when the run ends and refuses to open it again.
func (rt *Runtime) closeTree(runID string) {
	if rt == nil {
		return
	}
	rt.treeRead.Delete(runID)
	now := time.Now()
	rt.treeClosed.Store(runID, now)
	rt.treeClosed.Range(func(id, at any) bool {
		if now.Sub(at.(time.Time)) > treeClosedKeep {
			rt.treeClosed.Delete(id)
		}
		return true
	})
}

// treeReadFor is the run's open tree, or nil.
func (rt *Runtime) treeReadFor(runID string) *treeRead {
	if rt == nil || strings.TrimSpace(runID) == "" {
		return nil
	}
	v, ok := rt.treeRead.Load(runID)
	if !ok {
		return nil
	}
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

// searchProbeOf reports whether a search_start payload is a probe of the tree
// (yaegikernel.SearchStartPayload.Probe): it starts nothing.
func searchProbeOf(payload json.RawMessage) bool {
	var p yaegikernel.SearchStartPayload
	return json.Unmarshal(payload, &p) == nil && p.Probe
}

// searchProbe answers a probe with the tree as it stands now: the guest asks
// before it refuses a search, so a page a sibling session tried since the cell
// began is not demanded again.
func (rt *Runtime) searchProbe(runID string) (json.RawMessage, error) {
	return json.Marshal(yaegikernel.SearchStartResult{Tree: rt.treeStateFor(runID)})
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
	if len(t.order) == 0 {
		return nil
	}
	// Bounded here, so the frame and the probe's answer are one size.
	return yaegikernel.BoundTree(&yaegikernel.TreeState{Tried: append([]string(nil), t.order...)})
}
