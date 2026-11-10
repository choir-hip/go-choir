package agentcore

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The wake of a story's desk by an op-ed (handoff 4.C; contract 2 section 3.4,
// amendment A2). Host code folds the story's op-ed records into a decision
// (oped_wake_decision, its own record, so the owner can see why a wake did or
// did not happen) and, when it wakes, commits one oped_wake system record per
// op-ed to the story document's inbox. That record is on the waking list in
// RecordWakesReader; nothing else the op-ed path writes is.

// opedMeta is what the tape says about one op-ed of a story.
type opedMeta struct {
	VersionID    string
	AuthorLabel  string
	ManifestHash string
	RoutePath    string
	RespondsTo   types.OpedRespondsTo
	Verdict      types.OpedVerdictValue
	VerdictRun   int
	Reason       string
	Arrived      time.Time
}

// opedStoryState is the fold's input read from the tape.
type opedStoryState struct {
	Items     []opedStoryItem
	Meta      map[string]*opedMeta
	Decisions []types.CommitmentRecord // by n ascending
	LastN     int
	Prior     []opedPriorWake // reject-backoff wakes, oldest first (Changed not yet set)
}

func opedRecordTime(rec types.CommitmentRecord) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, rec.Provenance.CommittedAt)
	return t
}

func (rt *Runtime) storyOpedState(ctx context.Context, owner, computer, doc string) (opedStoryState, error) {
	recs, err := rt.store.ListCommitmentRecordsBySubjectRef(ctx, owner, computer, "story_doc_id", doc, 1000)
	if err != nil {
		return opedStoryState{}, err
	}
	st := opedStoryState{Meta: map[string]*opedMeta{}}
	woken := map[string]bool{}
	for _, r := range recs {
		if r.Oped == nil {
			continue
		}
		switch r.Oped.Kind {
		case types.OpedRecordArrival:
			a := r.Oped.Arrival
			m := st.Meta[a.VersionID]
			if m == nil {
				m = &opedMeta{VersionID: a.VersionID}
				st.Meta[a.VersionID] = m
			}
			m.AuthorLabel, m.ManifestHash, m.RoutePath, m.RespondsTo, m.Arrived = a.AuthorLabel, a.TextHash, a.RoutePath, a.RespondsTo, opedRecordTime(r)
		case types.OpedRecordVerdict:
			v := r.Oped.Verdict
			m := st.Meta[v.VersionID]
			if m == nil {
				m = &opedMeta{VersionID: v.VersionID}
				st.Meta[v.VersionID] = m
			}
			if v.Run >= m.VerdictRun {
				m.Verdict, m.VerdictRun, m.Reason = v.Verdict, v.Run, v.Reason
			}
		case types.OpedRecordWakeDecision:
			d := r.Oped.WakeDecision
			st.Decisions = append(st.Decisions, r)
			if d.N > st.LastN {
				st.LastN = d.N
			}
			if d.Woken {
				for _, v := range d.VersionIDs {
					woken[v] = true
				}
				if d.Why == types.OpedWakeRejectBackoff {
					st.Prior = append(st.Prior, opedPriorWake{At: opedRecordTime(r)})
				}
			}
		}
	}
	sort.Slice(st.Decisions, func(i, j int) bool {
		return st.Decisions[i].Oped.WakeDecision.N < st.Decisions[j].Oped.WakeDecision.N
	})
	sort.Slice(st.Prior, func(i, j int) bool { return st.Prior[i].At.Before(st.Prior[j].At) })
	versions := make([]string, 0, len(st.Meta))
	for v, m := range st.Meta {
		if m.Arrived.IsZero() && m.AuthorLabel == "" {
			continue // a verdict without its arrival in view
		}
		versions = append(versions, v)
	}
	sort.Slice(versions, func(i, j int) bool {
		a, b := st.Meta[versions[i]], st.Meta[versions[j]]
		if !a.Arrived.Equal(b.Arrived) {
			return a.Arrived.Before(b.Arrived)
		}
		return a.VersionID < b.VersionID
	})
	for _, v := range versions {
		m := st.Meta[v]
		st.Items = append(st.Items, opedStoryItem{VersionID: v, AuthorLabel: m.AuthorLabel, Judged: m.VerdictRun > 0,
			Verdict: m.Verdict, Woken: woken[v]})
	}
	return st, nil
}

// docChangedSince reports whether the desk committed a revision of the story
// after at whose body differs from what stood at at: the wake changed
// something. A document it cannot read counts as unchanged.
func (rt *Runtime) docChangedSince(ctx context.Context, owner, computer, doc string, at time.Time) bool {
	revs, err := rt.store.ListRevisionsByScope(ctx, doc, owner, computer, 200)
	if err != nil || len(revs) == 0 {
		return false
	}
	sort.Slice(revs, func(i, j int) bool { return revs[i].CreatedAt.Before(revs[j].CreatedAt) })
	standing := ""
	for _, r := range revs {
		if !r.CreatedAt.After(at) {
			standing = r.Content
		}
	}
	for _, r := range revs {
		if r.CreatedAt.After(at) && r.AuthorKind == types.AuthorAppAgent && r.Content != standing {
			return true
		}
	}
	return false
}

func (rt *Runtime) storyDeskBusy(ctx context.Context, owner, computer, doc string) bool {
	_, active, err := rt.TextureActiveRunByAgent(ctx, owner, computer, deskmanifest.DocumentInbox(doc))
	return err == nil && active
}

func opedCountsEqual(a, b types.OpedWakeCounts) bool {
	return a.Arrivals == b.Arrivals && a.Accepted == b.Accepted && a.Rejected == b.Rejected && a.UnreadRejects == b.UnreadRejects &&
		a.DistinctRejectAuthors == b.DistinctRejectAuthors && a.Threshold == b.Threshold && a.GapSeconds == b.GapSeconds
}

// evaluateOpedWake decides one story's wake and commits what it decides. It
// reports whether the story still has an op-ed the desk has not been shown.
func (rt *Runtime) evaluateOpedWake(ctx context.Context, owner, computer, doc string) (open bool, err error) {
	st, err := rt.storyOpedState(ctx, owner, computer, doc)
	if err != nil {
		return true, err
	}
	for i := range st.Prior {
		st.Prior[i].Changed = rt.docChangedSince(ctx, owner, computer, doc, st.Prior[i].At)
	}
	out := foldOpedWake(opedWakeInput{Now: time.Now().UTC(), Busy: rt.storyDeskBusy(ctx, owner, computer, doc), Items: st.Items, PriorRejectWakes: st.Prior})
	if out.Hold {
		return true, nil
	}
	pending := out.Counts.UnreadRejects > 0 || len(out.WakeVersionIDs) > 0
	if !out.Woken {
		if n := len(st.Decisions); n > 0 && opedCountsEqual(st.Decisions[n-1].Oped.WakeDecision.Counts, out.Counts) {
			return pending, nil
		}
		if out.Counts.Arrivals == 0 {
			return false, nil
		}
		d := types.OpedWakeDecision{StoryDocID: doc, N: st.LastN + 1, Woken: false, Why: out.Why, Counts: out.Counts}
		if err := rt.emitOpedWithSubject(ctx, owner, computer, types.CommitmentOped{Kind: types.OpedRecordWakeDecision, WakeDecision: &d},
			map[string]string{"story_doc_id": doc}); err != nil {
			return pending, err
		}
		return pending, nil
	}
	n := st.LastN + 1
	decisionID := types.OpedWakeDecisionRecordID(doc, n)
	// The wake records first, each deterministic per op-ed: a crash between
	// them and the decision repeats them as no-ops and writes the decision.
	for _, v := range out.WakeVersionIDs {
		m := st.Meta[v]
		w := types.OpedWake{StoryDocID: doc, VersionID: v, Why: out.Why, RespondsTo: m.RespondsTo, DecisionID: decisionID}
		subject := map[string]string{"oped_version_id": v, "story_doc_id": doc, "manifest_hash": m.ManifestHash,
			"author_label": m.AuthorLabel, "verdict": string(m.Verdict)}
		if err := rt.emitOpedWithSubject(ctx, owner, computer, types.CommitmentOped{Kind: types.OpedRecordWake, Wake: &w}, subject); err != nil &&
			!rt.opedRecordExists(ctx, owner, computer, types.OpedWakeRecordID(v)) {
			return true, fmt.Errorf("wake the desk for %s: %w", v, err)
		}
	}
	d := types.OpedWakeDecision{StoryDocID: doc, N: n, Woken: true, Why: out.Why, Counts: out.Counts, VersionIDs: out.ReadVersionIDs}
	if err := rt.emitOpedWithSubject(ctx, owner, computer, types.CommitmentOped{Kind: types.OpedRecordWakeDecision, WakeDecision: &d},
		map[string]string{"story_doc_id": doc}); err != nil {
		return true, err
	}
	return false, nil
}

// evaluateAllOpedWakes decides the wake of every story the poll still has an
// op-ed open on, and keeps the poll's list of open stories.
func (rt *Runtime) evaluateAllOpedWakes(ctx context.Context, poll *store.PlatformPoll, owner, computer string) {
	docs := make([]string, 0, len(poll.OpenStories))
	for doc := range poll.OpenStories {
		docs = append(docs, doc)
	}
	sort.Strings(docs)
	for _, doc := range docs {
		if strings.TrimSpace(doc) == "" {
			delete(poll.OpenStories, doc)
			continue
		}
		open, err := rt.evaluateOpedWake(ctx, owner, computer, doc)
		if err != nil {
			log.Printf("oped wake: story %s: %v", doc, err)
			continue
		}
		if !open {
			delete(poll.OpenStories, doc)
		}
	}
}

// OpedRef is one op-ed as the desk is shown it. The text itself is fetched by
// the host from the platform and pinned by ManifestHash; nothing here is the
// op-ed's words except AuthorLabel (a handle) and Reason (the gate's own
// template text, marked untrusted all the same).
type OpedRef struct {
	VersionID    string
	AuthorLabel  string
	ManifestHash string
	RoutePath    string
	Verdict      types.OpedVerdictValue
	Reason       string
	Why          types.OpedWakeWhy
	RespondsTo   types.OpedRespondsTo
}

// OpedWakeView is what a woken story desk is shown: the op-eds the unread
// oped_wake records name (Wake) and the other op-eds of the same decisions the
// desk is shown with them, rejected ones included (Carried).
type OpedWakeView struct {
	Wake    []OpedRef
	Carried []OpedRef
}

func refFrom(m *opedMeta, why types.OpedWakeWhy) OpedRef {
	return OpedRef{VersionID: m.VersionID, AuthorLabel: m.AuthorLabel, ManifestHash: m.ManifestHash, RoutePath: m.RoutePath,
		Verdict: m.Verdict, Reason: m.Reason, Why: why, RespondsTo: m.RespondsTo}
}

// OpedWakeContext reads the unread oped_wake records of a story document's
// inbox and the op-eds they bring. It writes nothing; the desk's turn marks
// the records read.
func (rt *Runtime) OpedWakeContext(ctx context.Context, owner, computer, doc string) (OpedWakeView, error) {
	var view OpedWakeView
	if rt == nil || rt.store == nil {
		return view, nil
	}
	inbox := deskmanifest.DocumentInbox(doc)
	cursor, err := rt.store.ReaderCursor(ctx, owner, computer, inbox, inbox)
	if err != nil {
		return view, err
	}
	head, err := rt.store.InboxHead(ctx, owner, computer, inbox)
	if err != nil || head <= cursor {
		return view, err
	}
	recs, err := rt.store.ListInboxRecords(ctx, owner, computer, inbox, cursor, head)
	if err != nil {
		return view, err
	}
	var wakes []types.OpedWake
	for _, r := range recs {
		if r.Oped != nil && r.Oped.Kind == types.OpedRecordWake && r.Oped.Wake != nil {
			wakes = append(wakes, *r.Oped.Wake)
		}
	}
	if len(wakes) == 0 {
		return view, nil
	}
	st, err := rt.storyOpedState(ctx, owner, computer, doc)
	if err != nil {
		return view, err
	}
	named := map[string]bool{}
	decisions := map[string]bool{}
	for _, w := range wakes {
		named[w.VersionID] = true
		decisions[w.DecisionID] = true
		if m := st.Meta[w.VersionID]; m != nil {
			view.Wake = append(view.Wake, refFrom(m, w.Why))
		}
	}
	for _, d := range st.Decisions {
		if !decisions[d.RecordID] {
			continue
		}
		for _, v := range d.Oped.WakeDecision.VersionIDs {
			if m := st.Meta[v]; m != nil && !named[v] {
				named[v] = true
				view.Carried = append(view.Carried, refFrom(m, d.Oped.WakeDecision.Why))
			}
		}
	}
	return view, nil
}
