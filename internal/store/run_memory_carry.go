package store

// The rule the carry reader (LatestActorRunMemoryEntries) and the retirement
// selector (selectRunMemoryPlan) share, in one place so they cannot drift:
// which kinds a carry can use, how a run's several records resolve to one, and
// the order in which sources are taken. A carry takes the newest run whose
// record is completed or passivated and that holds a message or compaction
// entry; retirement keeps the newest runMemoryKeepPerAgent of those.

import (
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// runMemoryCarryKinds are the entry kinds a carry reads (carryActorConversation
// takes the newest compaction and the messages after it). A run holding none of
// them has nothing to carry.
var runMemoryCarryKinds = []string{string(types.RunMemoryEntryMessage), string(types.RunMemoryEntryCompaction)}

// runMemoryHasCarryBody reports whether any entry is of a kind a carry uses.
func runMemoryHasCarryBody(entries []types.RunMemoryEntry) bool {
	for _, e := range entries {
		for _, k := range runMemoryCarryKinds {
			if string(e.Kind) == k {
				return true
			}
		}
	}
	return false
}

// runCarryFinishedAt is a run's place in the carry order: FinishedAt, else
// UpdatedAt, else CreatedAt.
func runCarryFinishedAt(r types.RunRecord) time.Time {
	switch {
	case r.FinishedAt != nil && !r.FinishedAt.IsZero():
		return *r.FinishedAt
	case !r.UpdatedAt.IsZero():
		return r.UpdatedAt
	}
	return r.CreatedAt
}

// carryRunBefore reports whether run a comes before run b in the carry order:
// the later finish first, and on equal finishes the greater run id first (a
// total, deterministic order).
func carryRunBefore(aAt time.Time, aRun string, bAt time.Time, bRun string) bool {
	if !aAt.Equal(bAt) {
		return aAt.After(bAt)
	}
	return aRun > bRun
}

// carrySourceState is a record state a carry can take its source from.
func carrySourceState(s types.RunState) bool {
	return s == types.RunCompleted || s == types.RunPassivated
}

// runCopyBetter reports whether copy a is preferred to copy b when both are
// live records of one run. The preference is a total order, so the result does
// not depend on the order the copies are read in: a copy that is not terminal
// first (it keeps the run), then the later finish, then a carry-source state,
// then the state name, then the computer id.
func runCopyBetter(a, b types.RunRecord) bool {
	if a.State.Terminal() != b.State.Terminal() {
		return !a.State.Terminal()
	}
	if !a.State.Terminal() {
		// Two copies that are not terminal: the carry-source state, then the name.
		if carrySourceState(a.State) != carrySourceState(b.State) {
			return carrySourceState(a.State)
		}
	} else if af, bf := runCarryFinishedAt(a), runCarryFinishedAt(b); !af.Equal(bf) {
		return af.After(bf)
	} else if carrySourceState(a.State) != carrySourceState(b.State) {
		return carrySourceState(a.State)
	}
	if a.State != b.State {
		return a.State < b.State
	}
	return a.ComputerID < b.ComputerID
}

// resolveRunCopies reduces the copies of one run's record to the one both the
// reader and the selector act on (runCopyBetter): a copy that is not terminal
// beats a terminal one, among terminal copies the newest finish wins, and ties
// go to the carry-source state and then by name, never by read order.
func resolveRunCopies(best types.RunRecord, haveBest bool, r types.RunRecord) types.RunRecord {
	if !haveBest || runCopyBetter(r, best) {
		return r
	}
	return best
}
