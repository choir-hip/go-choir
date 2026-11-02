package agentcore

import (
	"bytes"
	"context"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The no-delegate log line is suppressed for a claimed rebase assignment (tape
// path) and kept when the lookup finds no claim.
func TestIsRebaseAssignmentFollowsTheClaimOnTheTape(t *testing.T) {
	f := newRebaseCastFixture(t)
	plan, err := f.plan(t, f.spec(nil), f.rebaseObjective())
	if err != nil {
		t.Fatal(err)
	}
	records := types.CommitInboxRecordsRequest{OwnerID: f.owner, ComputerID: f.computer, CommandID: "cell-records:" + plan.AssignmentID, Records: []types.CommitmentRecord{plan.Cast}}
	records.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(records)
	if _, err := f.rt.openDelegatedCastAssignment(context.Background(), plan, records); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	f.rt.noteDelegateCell(plan.AssignmentID, 1, "", "", true)
	isRebase := func() bool {
		return f.rt.isRebaseAssignment(context.Background(), f.owner, f.computer, plan.AssignmentID, 1)
	}
	if use := f.rt.delegateUseFor(plan.AssignmentID, 1, true, isRebase); use == nil || !use.SourcePatch {
		t.Fatalf("use = %+v", use)
	}
	if strings.Contains(buf.String(), "source patch without delegate") {
		t.Fatalf("claimed rebase logged: %q", buf.String())
	}

	// Lookup failure (no such assignment) keeps the line.
	f.rt.noteDelegateCell("assignment-unknown", 1, "", "", true)
	unknown := func() bool {
		return f.rt.isRebaseAssignment(context.Background(), f.owner, f.computer, "assignment-unknown", 1)
	}
	f.rt.delegateUseFor("assignment-unknown", 1, true, unknown)
	if !strings.Contains(buf.String(), "source patch without delegate assignment=assignment-unknown") {
		t.Fatalf("lookup failure suppressed the line: %q", buf.String())
	}
}
