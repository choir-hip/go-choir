package agentcore

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Calibration of the gate comes from the desk's own reading (handoff 4.C): a
// rejected op-ed the desk goes on to use is a false reject; an accepted one
// that ends in "read it, no change" is a false accept. It is derived on read
// from the tape and the story's revisions; no model is asked, and nothing is
// written. The gate's question wording and thresholds are tuned from these.

// OpedCalibrationSignal is one observed gate error.
type OpedCalibrationSignal struct {
	VersionID string `json:"version_id"`
	// Kind is "false_reject" or "false_accept".
	Kind string `json:"kind"`
}

// opedWakeAt is a wake that carried op-eds to the desk: when, which, and
// whether the desk has read it.
type opedWakeAt struct {
	At         time.Time
	VersionIDs []string
	Read       bool
}

func revisionSourceIDs(rev types.Revision) map[string]bool {
	out := map[string]bool{}
	if len(rev.Provenance) == 0 {
		return out
	}
	var prov types.Provenance
	if json.Unmarshal(rev.Provenance, &prov) != nil {
		return out
	}
	for _, l := range prov.ParentLinks {
		if l.Kind == types.ParentLinkSource && l.PublicationVersionID != "" {
			out[l.PublicationVersionID] = true
		}
	}
	return out
}

func deriveOpedCalibration(meta map[string]*opedMeta, wakes []opedWakeAt, revs []types.Revision) []OpedCalibrationSignal {
	revs = append([]types.Revision(nil), revs...)
	sort.Slice(revs, func(i, j int) bool { return revs[i].CreatedAt.Before(revs[j].CreatedAt) })
	cited := map[string]bool{}
	for _, r := range revs {
		if r.AuthorKind != types.AuthorAppAgent {
			continue
		}
		for id := range revisionSourceIDs(r) {
			cited[id] = true
		}
	}
	// A wake's turn is the desk's revisions between it and the next wake.
	wakes = append([]opedWakeAt(nil), wakes...)
	sort.Slice(wakes, func(i, j int) bool { return wakes[i].At.Before(wakes[j].At) })
	changedIn := func(from, until time.Time) bool {
		standing := ""
		for _, r := range revs {
			if !r.CreatedAt.After(from) {
				standing = r.Content
			}
		}
		for _, r := range revs {
			if r.CreatedAt.After(from) && (until.IsZero() || !r.CreatedAt.After(until)) && r.AuthorKind == types.AuthorAppAgent && r.Content != standing {
				return true
			}
		}
		return false
	}
	var out []OpedCalibrationSignal
	seen := map[string]bool{}
	add := func(v, kind string) {
		if !seen[v+kind] {
			seen[v+kind] = true
			out = append(out, OpedCalibrationSignal{VersionID: v, Kind: kind})
		}
	}
	for i, w := range wakes {
		var until time.Time
		if i+1 < len(wakes) {
			until = wakes[i+1].At
		}
		for _, v := range w.VersionIDs {
			m := meta[v]
			if m == nil || m.VerdictRun == 0 {
				continue
			}
			switch {
			case isRejectVerdict(m.Verdict) && cited[v]:
				add(v, "false_reject")
			case m.Verdict == types.OpedAccepted && w.Read && !cited[v] && !changedIn(w.At, until):
				add(v, "false_accept")
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].VersionID < out[j].VersionID })
	return out
}

// OpedCalibration reads the gate's errors on one story from the desk's own
// reading of its op-eds.
func (rt *Runtime) OpedCalibration(ctx context.Context, owner, computer, doc string) ([]OpedCalibrationSignal, error) {
	st, err := rt.storyOpedState(ctx, owner, computer, doc)
	if err != nil {
		return nil, err
	}
	inbox := deskmanifest.DocumentInbox(doc)
	cursor, err := rt.store.ReaderCursor(ctx, owner, computer, inbox, inbox)
	if err != nil {
		return nil, err
	}
	var wakes []opedWakeAt
	recs, err := rt.store.ListInboxRecords(ctx, owner, computer, inbox, 0, 0)
	if err != nil {
		return nil, err
	}
	byVersion := map[string]*opedWakeAt{}
	for _, r := range recs {
		if r.Oped == nil || r.Oped.Wake == nil {
			continue
		}
		w := byVersion[r.Oped.Wake.VersionID]
		if w == nil {
			w = &opedWakeAt{At: opedRecordTime(r), VersionIDs: []string{r.Oped.Wake.VersionID}}
			byVersion[r.Oped.Wake.VersionID] = w
		}
		w.Read = r.InboxSeq <= cursor
	}
	for _, w := range byVersion {
		wakes = append(wakes, *w)
	}
	revs, err := rt.store.ListRevisionsByScope(ctx, doc, owner, computer, 200)
	if err != nil {
		return nil, err
	}
	return deriveOpedCalibration(st.Meta, wakes, revs), nil
}
