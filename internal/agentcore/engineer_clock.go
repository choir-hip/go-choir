package agentcore

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The engineer's clock (docs/problems/engineer-has-no-clock-2026-11-04.md).
//
// An implementation engineer is told to Freeze inside a minute budget it
// cannot see. Its cell's choir.Context() carries elapsed_min, budget_min and
// budget_left_min, counted from roundClockAnchor, the anchor the
// freeze_minutes settlement uses. One harness line rides a tool result when
// elapsed crosses budget minus engineerClockWarnLead. Management's
// prediction never enters: the clock has no field for it.

// engineerClockWarnLead is how long before the budget ends the line fires.
const engineerClockWarnLead = 20 * time.Minute

// engineerClock is one run's clock at an instant.
type engineerClock struct {
	Elapsed, Budget time.Duration
	Revision        int
	Anchor          time.Time
	Key             string // once-per-assignment-and-revision key
}

func (c engineerClock) cell() *yaegikernel.CellClock {
	return &yaegikernel.CellClock{ElapsedSec: int64(c.Elapsed / time.Second), BudgetSec: int64(c.Budget / time.Second), Revision: c.Revision, AnchorUnixMS: c.Anchor.UnixMilli()}
}

// engineerClockAt computes the clock from an anchor stamp; ok is false when
// the stamp or the budget is unusable (C3).
func engineerClockAt(anchor string, now time.Time, budget time.Duration) (time.Duration, time.Time, bool) {
	t, ok := parseCommitmentStamp(anchor)
	// A future anchor is skew, not "just started": no clock.
	if !ok || budget <= 0 || t.After(now) {
		return 0, time.Time{}, false
	}
	return now.Sub(t), t, true
}

// engineerClockLine is the harness line, or "" before the lead. Minutes are
// whole; K never goes below 0.
func engineerClockLine(elapsed, budget time.Duration) string {
	if budget <= 0 || elapsed < budget-engineerClockWarnLead {
		return ""
	}
	left := int((budget - elapsed) / time.Minute)
	if left < 0 {
		left = 0
	}
	return fmt.Sprintf("%d minutes used of %d; Freeze within the next %d minutes or say why in a Report.",
		int(elapsed/time.Minute), int(budget/time.Minute), left)
}

// engineerClockForRun reads the clock of an implementation engineer's run;
// nil when it has none (C3): any other slot or run, an unreadable
// assignment, or an unusable anchor. It never fails the cell.
func (rt *Runtime) engineerClockForRun(ctx context.Context, rec *types.RunRecord, now time.Time) *engineerClock {
	if rt == nil || rt.store == nil || rec == nil {
		return nil
	}
	id := metadataStringValue(rec.Metadata, "assignment_id")
	attempt := metadataIntValue(rec.Metadata, "assignment_attempt")
	if id == "" || attempt <= 0 || metadataStringValue(rec.Metadata, "assignment_kind") != string(types.EngineeringAssignmentImplementation) {
		return nil
	}
	assignment, err := rt.store.GetEngineeringAssignment(ctx, rec.OwnerID, rec.ComputerID, id, uint64(attempt))
	if err != nil || assignment.Binding.Kind != types.EngineeringAssignmentImplementation {
		if err != nil {
			log.Printf("engineer clock: assignment %s: %v", id, err)
		}
		return nil
	}
	var cast types.CommitmentRecord
	if castID := strings.TrimSpace(assignment.Binding.ParentControlID); castID != "" {
		// The settlement reads this record and fails without it, so a clock
		// cannot quietly count from the assignment's creation instead:
		// no clock beats a wrong one.
		rc, err := rt.store.GetCommitmentRecord(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, castID)
		if err != nil || rc == nil {
			log.Printf("engineer clock: assignment %s: cast %s unreadable (%v); no clock", id, castID, err)
			return nil
		}
		cast = *rc
	}
	budget := rt.activationBudgetForRun(rec)
	elapsed, anchor, ok := engineerClockAt(roundClockAnchor(cast, assignment), now, budget)
	if !ok {
		return nil
	}
	return &engineerClock{Elapsed: elapsed, Budget: budget, Anchor: anchor, Key: fmt.Sprintf("%s:%d", id, attempt)}
}

// takeEngineerClockLine returns the line the first time it is due for this
// clock's assignment (and revision), "" after (C4). Call it only when the
// line will be attached to a result that is returned.
func (rt *Runtime) takeEngineerClockLine(c *engineerClock, start time.Time) string {
	if rt == nil || c == nil {
		return ""
	}
	line := engineerClockLine(c.Elapsed+time.Since(start), c.Budget)
	if line == "" {
		return ""
	}
	key := fmt.Sprintf("%s:%d", c.Key, c.Revision)
	if _, loaded := rt.engineerClockFired.LoadOrStore(key, struct{}{}); loaded {
		return ""
	}
	return line
}
