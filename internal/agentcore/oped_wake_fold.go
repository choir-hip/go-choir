package agentcore

import (
	"sort"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// The wake decision of one story (handoff 4.C "Rejects can wake the desk too,
// with backoff" and "Coalesce"; contract 2 section 3.4). A pure fold over the
// story's op-ed facts, so the owner can see on the tape why a wake did or did
// not happen:
//
//   - an accepted op-ed wakes the desk;
//   - a rejected one never does by itself; unread rejects, counted by
//     distinct author, wake it once they pass a threshold, and the threshold
//     and the minimum gap between such wakes double after a wake that changed
//     nothing and reset after one that did;
//   - a desk that is working holds a wake; what arrives meanwhile joins the
//     wake that follows.
const (
	rejectWakeBaseThreshold = 3
	rejectWakeBaseGap       = 24 * time.Hour
	rejectWakeMaxDoublings  = 6
	// opedAcceptedWakeGap is the least time between two accepted-op-ed wakes
	// of one story; op-eds arriving inside it join the next wake.
	opedAcceptedWakeGap = 30 * time.Minute
)

// opedStoryItem is one op-ed on a story as the tape holds it.
type opedStoryItem struct {
	VersionID   string
	AuthorLabel string
	// Judged is true once a verdict is on the tape; Verdict is the latest.
	Judged  bool
	Verdict types.OpedVerdictValue
	// Woken is true once a wake carried the op-ed to the desk (accepted ones)
	// or the desk was shown it (rejected ones).
	Woken bool
}

// opedPriorWake is one earlier reject-backoff wake of the story, oldest first.
type opedPriorWake struct {
	At      time.Time
	Changed bool
}

type opedWakeInput struct {
	Now              time.Time
	Busy             bool
	Items            []opedStoryItem
	PriorRejectWakes []opedPriorWake
	// LastAcceptedWake is when an accepted op-ed last woke this story's desk.
	LastAcceptedWake time.Time
}

type opedWakeOutput struct {
	Woken bool
	// Hold means a wake is due but the desk is working; nothing is written
	// and the next evaluation tries again.
	Hold bool
	Why  types.OpedWakeWhy
	// WakeVersionIDs are the op-eds the wake record(s) name (accepted ones
	// for an accepted wake, the unread rejects for a backoff wake).
	WakeVersionIDs []string
	// ReadVersionIDs are every op-ed this wake shows the desk, rejects
	// included; they count as read afterwards.
	ReadVersionIDs []string
	Counts         types.OpedWakeCounts
}

func isRejectVerdict(v types.OpedVerdictValue) bool {
	switch v {
	case types.OpedRejectedAbusive, types.OpedRejectedRedundant, types.OpedRejectedUnclear, types.OpedRejectedOffTopic:
		return true
	}
	return false
}

func foldOpedWake(in opedWakeInput) opedWakeOutput {
	var accepted, rejects []opedStoryItem
	counts := types.OpedWakeCounts{}
	authors := map[string]bool{}
	for _, it := range in.Items {
		counts.Arrivals++
		switch {
		case it.Judged && it.Verdict == types.OpedAccepted:
			counts.Accepted++
			if !it.Woken {
				accepted = append(accepted, it)
			}
		case it.Judged && isRejectVerdict(it.Verdict):
			counts.Rejected++
			if !it.Woken {
				rejects = append(rejects, it)
				authors[it.AuthorLabel] = true
			}
		}
	}
	counts.UnreadRejects, counts.DistinctRejectAuthors = len(rejects), len(authors)

	// Backoff state: the no-change wakes since the last one that changed something.
	noChange := 0
	var last time.Time
	for _, w := range in.PriorRejectWakes {
		if w.Changed {
			noChange = 0
		} else {
			noChange++
		}
		last = w.At
	}
	doublings := noChange
	if doublings > rejectWakeMaxDoublings {
		doublings = rejectWakeMaxDoublings
	}
	counts.NoChangeWakes = noChange
	counts.Threshold = rejectWakeBaseThreshold << doublings
	gap := rejectWakeBaseGap << doublings
	counts.GapSeconds = int(gap / time.Second)

	ids := func(items []opedStoryItem) []string {
		out := make([]string, 0, len(items))
		for _, it := range items {
			out = append(out, it.VersionID)
		}
		sort.Strings(out)
		return out
	}
	switch {
	case len(accepted) > 0:
		if in.Busy || (!in.LastAcceptedWake.IsZero() && in.Now.Sub(in.LastAcceptedWake) < opedAcceptedWakeGap) {
			return opedWakeOutput{Hold: true, Why: types.OpedWakeCoalesced, Counts: counts}
		}
		return opedWakeOutput{Woken: true, Why: types.OpedWakeAccepted, WakeVersionIDs: ids(accepted),
			ReadVersionIDs: ids(append(append([]opedStoryItem{}, accepted...), rejects...)), Counts: counts}
	case len(rejects) == 0:
		return opedWakeOutput{Why: types.OpedWakeBelowThreshold, Counts: counts}
	case counts.DistinctRejectAuthors < counts.Threshold:
		return opedWakeOutput{Why: types.OpedWakeBelowThreshold, Counts: counts}
	case !last.IsZero() && in.Now.Sub(last) < gap:
		return opedWakeOutput{Why: types.OpedWakeGapNotElapsed, Counts: counts}
	case in.Busy:
		return opedWakeOutput{Hold: true, Why: types.OpedWakeCoalesced, Counts: counts}
	}
	return opedWakeOutput{Woken: true, Why: types.OpedWakeRejectBackoff, WakeVersionIDs: ids(rejects), ReadVersionIDs: ids(rejects), Counts: counts}
}
