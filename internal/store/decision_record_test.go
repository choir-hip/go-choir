package store

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// R0, the decision record. Ways it can fail, each with its test:
//  1. a row is written but cannot be read back by capture, so no capture's
//     choice can be asked for;
//  2. a replay of the same decision writes a second row;
//  3. a row is changed after it was written (not append-only);
//  4. a desk can write the kind (the custodian is the decisions service);
//  5. a second decision of one capture overwrites the first;
//  6. a row without a decider or stage is accepted (the redesign's stages
//     could not be told apart);
//  7. the rows of another computer or owner are read.
func decisionRow(capture, command string, at time.Time) types.DecisionRecord {
	return types.DecisionRecord{OwnerID: "o1", ComputerID: "c1", CaptureID: capture, Stage: types.DecisionStageManagementRoute,
		Decision: types.DecisionFeed, Target: "doc-1", DeciderKind: types.DeciderDesk, Decider: "management",
		RunID: "run-1", CommandID: command, Candidates: []string{"doc-1", "doc-2"}, CandidatesShown: 2, At: at}
}

func TestDecisionRecordReadsBackByCaptureAndStage(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	at := time.Date(2026, 11, 24, 3, 0, 0, 0, time.UTC)
	if n, err := s.AppendDecisionRecords(ctx, []types.DecisionRecord{decisionRow("cap-a", "cmd-1", at), decisionRow("cap-b", "cmd-1", at)}); err != nil || n != 2 {
		t.Fatalf("append = %d, %v", n, err)
	}
	got, err := s.ListDecisionRecords(ctx, "o1", "c1", types.DecisionFilter{CaptureID: "cap-a"})
	if err != nil || len(got) != 1 {
		t.Fatalf("by capture = %v, %v", got, err)
	}
	r := got[0]
	if r.Decision != "feed" || r.Target != "doc-1" || r.Decider != "management" || r.DeciderKind != types.DeciderDesk || len(r.Candidates) != 2 || r.Probability != nil || !r.At.Equal(at) {
		t.Fatalf("row = %+v", r)
	}
	if all, _ := s.ListDecisionRecords(ctx, "o1", "c1", types.DecisionFilter{Stage: types.DecisionStageManagementRoute}); len(all) != 2 {
		t.Fatalf("by stage = %d, want 2", len(all))
	}
	if none, _ := s.ListDecisionRecords(ctx, "o1", "c1", types.DecisionFilter{Stage: "grouping"}); len(none) != 0 {
		t.Fatalf("another stage read %d rows", len(none))
	}
	if other, _ := s.ListDecisionRecords(ctx, "o2", "c1", types.DecisionFilter{}); len(other) != 0 {
		t.Fatalf("another owner read %d rows", len(other))
	}
	if other, _ := s.ListDecisionRecords(ctx, "o1", "c2", types.DecisionFilter{}); len(other) != 0 {
		t.Fatalf("another computer read %d rows", len(other))
	}
}

func TestDecisionRecordReplayAndLaterDecisionAreAppendOnly(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	at := time.Date(2026, 11, 24, 3, 0, 0, 0, time.UTC)
	first := decisionRow("cap-a", "cmd-1", at)
	if _, err := s.AppendDecisionRecords(ctx, []types.DecisionRecord{first}); err != nil {
		t.Fatal(err)
	}
	changed := first
	// A replay is the same decision at another time; a different decision in
	// the same command is another row (TestTwoDecisionsOnOneCapture...).
	changed.At, changed.Probability = at.Add(time.Hour), nil
	if n, err := s.AppendDecisionRecords(ctx, []types.DecisionRecord{changed}); err != nil || n != 0 {
		t.Fatalf("replay wrote %d rows, %v", n, err)
	}
	later := decisionRow("cap-a", "cmd-2", at.Add(2*time.Hour))
	later.Decision = types.DecisionOpen
	if n, err := s.AppendDecisionRecords(ctx, []types.DecisionRecord{later}); err != nil || n != 1 {
		t.Fatalf("second decision = %d, %v", n, err)
	}
	got, _ := s.ListDecisionRecords(ctx, "o1", "c1", types.DecisionFilter{CaptureID: "cap-a"})
	if len(got) != 2 || got[0].Decision != "open" || got[1].Decision != "feed" {
		t.Fatalf("rows = %+v, want open then the first feed unchanged", got)
	}
}

func TestDecisionRecordRefusesRowsThatCannotTellStagesApart(t *testing.T) {
	s := openTestStore(t)
	for name, mut := range map[string]func(*types.DecisionRecord){
		"no decider": func(r *types.DecisionRecord) { r.Decider = "" },
		"no kind":    func(r *types.DecisionRecord) { r.DeciderKind = "" },
		"no stage":   func(r *types.DecisionRecord) { r.Stage = "" },
		"no capture": func(r *types.DecisionRecord) { r.CaptureID = "" },
	} {
		r := decisionRow("cap-a", "cmd-1", time.Now())
		mut(&r)
		if _, err := s.AppendDecisionRecords(context.Background(), []types.DecisionRecord{r}); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestDecisionRecordKindBelongsToTheDecisionsServiceOnly(t *testing.T) {
	defer objectgraph.SetModeForTest(objectgraph.WriterGateEnforce)()
	obj := objectgraph.Object{CanonicalID: "d1", ObjectKind: "choir.decision_record"}
	if err := objectgraph.CheckWriteAs(objectgraph.WithWriter(context.Background(), objectgraph.WriterDecisions), []objectgraph.Object{obj}, nil); err != nil {
		t.Fatalf("the decisions service refused: %v", err)
	}
	for _, w := range []objectgraph.Writer{objectgraph.WriterManagement, objectgraph.WriterTexture, objectgraph.WriterResearch, objectgraph.WriterEngineering} {
		if err := objectgraph.CheckWriteAs(objectgraph.WithWriter(context.Background(), w), []objectgraph.Object{obj}, nil); err == nil {
			t.Errorf("%s wrote the decision record", w)
		}
	}
}

// Failure 8: in product (enforce) a replay from a management cell, or from a
// context that declares nobody, reads the row it wrote under the decisions
// reader; without that, the replay is refused and drops the batch's new rows.
func TestDecisionRecordReplayIsNotRefusedUnderEnforceFromAnyContext(t *testing.T) {
	s := openTestStore(t)
	at := time.Date(2026, 11, 24, 3, 0, 0, 0, time.UTC)
	if _, err := s.AppendDecisionRecords(context.Background(), []types.DecisionRecord{decisionRow("cap-a", "cmd-1", at)}); err != nil {
		t.Fatal(err)
	}
	defer objectgraph.SetModeForTest(objectgraph.WriterGateEnforce)()
	for name, ctx := range map[string]context.Context{
		"management": objectgraph.WithWriter(objectgraph.WithReader(context.Background(), objectgraph.WriterManagement), objectgraph.WriterManagement),
		"undeclared": context.Background(),
	} {
		fresh := "cap-new-" + name
		n, err := s.AppendDecisionRecords(ctx, []types.DecisionRecord{decisionRow("cap-a", "cmd-1", at), decisionRow(fresh, "cmd-1", at)})
		if err != nil || n != 1 {
			t.Errorf("%s replay = %d, %v; want the new row written and the old skipped", name, n, err)
		}
		if n, err := s.AppendDecisionRecords(ctx, []types.DecisionRecord{decisionRow(fresh, "cmd-1", at)}); err != nil || n != 0 {
			t.Errorf("%s pure replay = %d, %v; want 0 and no refusal", name, n, err)
		}
	}
}

// Failure 9: two decisions about one capture in one command (a feed to one
// story and a decline, or a feed to each of two stories) are two rows.
func TestTwoDecisionsOnOneCaptureInOneCommandAreTwoRows(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	at := time.Date(2026, 11, 24, 3, 0, 0, 0, time.UTC)
	a := decisionRow("cap-a", "cell-records:1", at)
	b := decisionRow("cap-a", "cell-records:1", at)
	b.Target = "doc-2"
	c := decisionRow("cap-a", "cell-records:1", at)
	c.Decision, c.Target = types.DecisionDecline, ""
	if n, err := s.AppendDecisionRecords(ctx, []types.DecisionRecord{a, b, c}); err != nil || n != 3 {
		t.Fatalf("append = %d, %v; want 3 rows", n, err)
	}
	if n, err := s.AppendDecisionRecords(ctx, []types.DecisionRecord{a, b, c}); err != nil || n != 0 {
		t.Fatalf("replay = %d, %v; want 0", n, err)
	}
	if got, _ := s.ListDecisionRecords(ctx, "o1", "c1", types.DecisionFilter{CaptureID: "cap-a"}); len(got) != 3 {
		t.Fatalf("rows = %d, want 3", len(got))
	}
}

// A concurrent writer appending one row of a batch between the check and the
// write must not drop the batch's other new rows.
func TestAConflictOnOneRowDoesNotDropTheBatchesOtherRows(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	at := time.Date(2026, 11, 24, 3, 0, 0, 0, time.UTC)
	rows := []types.DecisionRecord{decisionRow("cap-a", "cmd-1", at), decisionRow("cap-b", "cmd-1", at), decisionRow("cap-c", "cmd-1", at)}
	decisionBetweenCheckAndWrite = func() {
		decisionBetweenCheckAndWrite = nil
		if _, err := s.AppendDecisionRecords(ctx, rows[1:2]); err != nil {
			t.Errorf("the concurrent writer: %v", err)
		}
	}
	defer func() { decisionBetweenCheckAndWrite = nil }()
	n, err := s.AppendDecisionRecords(ctx, rows)
	if err != nil || n != 2 {
		t.Fatalf("append = %d, %v; want the two rows the other writer did not write", n, err)
	}
	if got, _ := s.ListDecisionRecords(ctx, "o1", "c1", types.DecisionFilter{}); len(got) != 3 {
		t.Fatalf("rows = %d, want 3", len(got))
	}
}
