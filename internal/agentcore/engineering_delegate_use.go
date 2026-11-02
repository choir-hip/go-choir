package agentcore

import (
	"context"
	"log"
	"regexp"
	"strings"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/claudedelegate"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Delegate use per assignment attempt (owner, 2026-11-02: "With delegate,
// should strongly go to Claude"; docs/design/delegate-first-for-source-changes-2026-11-02.md).
// The engineering prompt makes the delegate the strong default for source
// edits; this file measures whether it was used. It reads the run's own
// cells (the Go source the engineer ran and what it printed), because
// choir.Exec runs inside the capsule and leaves no host-side argv. The
// counts are therefore best effort: a start counts when a cell that
// completed without error names choir-delegate "start"; a success counts
// once per run id whose printed status was state done with subtype success.

var (
	delegateStartRE = regexp.MustCompile(`"choir-delegate"\s*,\s*\[\]string\{\s*"start"`)
	delegateIDRE    = regexp.MustCompile(`"id"\s*:\s*"([^"]+)"`)
	delegateDoneRE  = regexp.MustCompile(`"state"\s*:\s*"done"`)
	delegateOKRE    = regexp.MustCompile(`"subtype"\s*:\s*"success"`)
)

// delegateCellCalls returns how many delegate runs the cell source starts and
// the ids of the runs its output shows finished with success.
func delegateCellCalls(source, stdout string) (starts int, succeeded []string) {
	if !strings.Contains(source, "choir-delegate") && !strings.Contains(stdout, `"subtype"`) {
		return 0, nil
	}
	starts = len(delegateStartRE.FindAllStringIndex(source, -1))
	// A cell that prints the status through %q or a struct escapes its quotes.
	out := strings.ReplaceAll(stdout, `\"`, `"`)
	if delegateDoneRE.MatchString(out) && delegateOKRE.MatchString(out) {
		if m := delegateIDRE.FindStringSubmatch(out); m != nil {
			succeeded = append(succeeded, m[1])
		}
	}
	return starts, succeeded
}

type delegateTally struct {
	mu       sync.Mutex
	calls    int
	done     map[string]bool
	patch    bool
	loggedAt bool
}

func (rt *Runtime) delegateTallyFor(assignmentID string, attempt uint64) *delegateTally {
	t, _ := rt.delegateTallies.LoadOrStore(assignmentSagaKey(assignmentID, attempt), &delegateTally{done: map[string]bool{}})
	return t.(*delegateTally)
}

// noteDelegateCell folds one completed cell into the attempt's tally. frozen
// marks that the cell committed a freeze, which is the source patch.
func (rt *Runtime) noteDelegateCell(assignmentID string, attempt uint64, source, stdout string, frozen bool) {
	if assignmentID == "" {
		return
	}
	starts, succeeded := delegateCellCalls(source, stdout)
	if starts == 0 && len(succeeded) == 0 && !frozen {
		rt.delegateTallyFor(assignmentID, attempt) // an attempt with no calls still reads as zero
		return
	}
	t := rt.delegateTallyFor(assignmentID, attempt)
	t.mu.Lock()
	defer t.mu.Unlock()
	t.calls += starts
	for _, id := range succeeded {
		t.done[id] = true
	}
	t.patch = t.patch || frozen
}

// delegateUseFor is the attempt's delegate use for its report, or nil where
// the delegate was not available (other computers keep today's behavior). It
// logs once when a patch was produced with the delegate available and unused
// (rebase assignments excluded).
// It does not clear the tally: a partial report keeps the assignment open.
func (rt *Runtime) delegateUseFor(assignmentID string, attempt uint64, available bool, isRebase func() bool) *types.EngineeringDelegateUse {
	if !available {
		return nil
	}
	t := rt.delegateTallyFor(assignmentID, attempt)
	t.mu.Lock()
	defer t.mu.Unlock()
	use := &types.EngineeringDelegateUse{Available: true, SourcePatch: t.patch, Calls: t.calls, Succeeded: len(t.done)}
	if t.calls > 0 || len(t.done) > 0 {
		use.Model = claudedelegate.Model
	}
	// A rebase assignment replays a carried patch onto the new base and never
	// calls the delegate, so the line would be noise there; delegate_use is
	// still recorded.
	if t.patch && t.calls == 0 && !t.loggedAt && !(isRebase != nil && isRebase()) {
		t.loggedAt = true
		log.Printf("engineering: source patch without delegate assignment=%s attempt=%d", assignmentID, attempt)
	}
	return use
}

// isRebaseAssignment reports whether the assignment answers a U4 rebase
// notice: its claim names it (rebaseLineage). Any lookup failure reads as not
// a rebase, which only keeps the log line.
func (rt *Runtime) isRebaseAssignment(ctx context.Context, ownerID, computerID, assignmentID string, attempt uint64) bool {
	if rt == nil || rt.store == nil || assignmentID == "" {
		return false
	}
	assignment, err := rt.store.GetEngineeringAssignment(ctx, ownerID, computerID, assignmentID, attempt)
	if err != nil {
		return false
	}
	_, ok := rt.rebaseLineage(ctx, assignment)
	return ok
}
