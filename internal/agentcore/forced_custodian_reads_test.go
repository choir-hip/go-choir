package agentcore

import (
	"context"
	"sort"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Enforce flip S1, D2 (sections 6.4, 8.3) and the background audit (6.7),
// written first. R1 to R5 are host reads made while the run context names a
// desk. withKindReader declares only when the context names no reader, so
// under a desk's context it did nothing and the desk's own identity was
// counted. Each site now forces the custodian with WithCustodianReader on the
// single read, and the tests drive each one under the desk's context.
//
//	FM3 R1 or R2 reads a document the cast or operation does not name.
//	FM4 R3's Texture identity leaks into a non-Texture caller.

func readerCrossings() []string {
	var out []string
	for key, n := range objectgraph.ReaderGateCounts() {
		out = append(out, key+" x"+countStr(n))
	}
	sort.Strings(out)
	return out
}

func requireNoReaderCrossings(t *testing.T, what string) {
	t.Helper()
	if got := readerCrossings(); len(got) != 0 {
		t.Errorf("%s: reader gate counted %v at %v", what, got, objectgraph.ReaderGateSites())
	}
}

// R1: management plans the engineering cast from the document the cast names,
// and the open composes Texture's story document with management's rows.
func TestR1ManagementPlansAndOpensTheCastWithoutACrossing(t *testing.T) {
	f := newRebaseCastFixture(t)
	objectgraph.ResetReaderGateCounts()
	ctx := withDeskReader(context.Background(), agentprofile.Management)
	plan, err := f.planRawCtx(t, ctx, withTestRoundPredictions(t, f.spec(nil)), f.rebaseObjective())
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	requireNoReaderCrossings(t, "planEngineeringCast under the management context")
	records := types.CommitInboxRecordsRequest{OwnerID: f.owner, ComputerID: f.computer, CommandID: "cell-records:" + plan.AssignmentID, Records: []types.CommitmentRecord{plan.Cast}}
	records.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(records)
	if _, err := f.rt.openDelegatedCastAssignment(ctx, plan, records); err != nil {
		t.Fatalf("open: %v", err)
	}
	requireNoReaderCrossings(t, "openDelegatedCastAssignment under the management context")
}

// FM3: the forced reader is the custodian's for the one document read, and
// the read is by the id the cast names. A document the cast does not name is
// not reachable through it, and the forced reader does not leak to the rest of
// the call: a management read of another kind after it is still management's.
func TestFM3TheForcedReadIsScopedToTheOneRead(t *testing.T) {
	ctx := withDeskReader(context.Background(), agentprofile.Management)
	forced := objectgraph.WithCustodianReader(ctx, "choir.texture_document")
	got, _ := objectgraph.ReaderFrom(forced)
	want, _ := objectgraph.CustodianOf("choir.texture_document")
	if got != want {
		t.Fatalf("forced reader = %s, want %s", got, want)
	}
	if still, _ := objectgraph.ReaderFrom(ctx); still == want {
		t.Fatal("forcing the custodian changed the caller's own context")
	}
}

// R2: the owner act resolves the operation's document on a host context that
// declares no reader. The read is the custodian's.
func TestR2OperationDocumentReadsAsTheCustodian(t *testing.T) {
	rt, _ := testRuntime(t)
	trajectoryID, _ := seedOwnerActDocument(t, rt.store, "owner-r2", rt.cfg.ComputerID, "doc-r2")
	stop := objectgraph.TrackUndeclaredReadsFromProduct()
	defer stop()
	objectgraph.ResetUndeclaredReads()
	objectgraph.ResetReaderGateCounts()
	doc, why, err := rt.operationDocument(context.Background(), "owner-r2", rt.cfg.ComputerID, selfdev.Operation{TrajectoryID: trajectoryID})
	if err != nil || why != "" || doc.DocID != "doc-r2" {
		t.Fatalf("operationDocument = %+v %q %v", doc, why, err)
	}
	if got := objectgraph.UndeclaredReads(); len(got) != 0 {
		t.Errorf("operationDocument read with no declared reader: %v", got)
	}
	requireNoReaderCrossings(t, "operationDocument")
}

// R4: the calibration snapshot reads the citation checks and support verdicts
// as their custodian, whichever desk's run builds it first (management and
// engineering both hold calibration.read, and the snapshot is cached per
// computer).
func TestR4CalibrationSnapshotReadsAsTheCustodianUnderEitherDesk(t *testing.T) {
	for _, desk := range []string{agentprofile.Engineering, agentprofile.Management} {
		rt, _ := testRuntime(t)
		objectgraph.ResetReaderGateCounts()
		ctx := withDeskReader(context.Background(), desk)
		_, _ = rt.calibrationSnapshotFor(ctx, "owner-r4", "computer-r4", 0)
		requireNoReaderCrossings(t, "calibrationSnapshotFor under "+desk)
	}
}

// R5: the lineage notice declares its own reader. Engineering is already the
// custodian of the lineage kind; the host goroutine that builds the notice
// carried no reader at all.
func TestR5LineageNoticeDeclaresItsReader(t *testing.T) {
	rt, _ := testRuntime(t)
	stop := objectgraph.TrackUndeclaredReadsFromProduct()
	defer stop()
	objectgraph.ResetUndeclaredReads()
	objectgraph.ResetReaderGateCounts()
	rec := &types.RunRecord{RunID: "run-r5", OwnerID: "owner-r5", ComputerID: "computer-r5", Prompt: "x"}
	_ = rt.buildLineageNotice(rec)
	if got := objectgraph.UndeclaredReads(); len(got) != 0 {
		t.Errorf("buildLineageNotice read with no declared reader: %v", got)
	}
	requireNoReaderCrossings(t, "buildLineageNotice")
}
