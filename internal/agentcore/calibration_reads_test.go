package agentcore

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// C6, M2: a read mark says whether the run had predicted; a claim stated
// after a read carries the first read's time, one stated by a run that
// never read carries none; the state goes with the run (M4).
func TestCalibrationReadMarksAndClaimStamps(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	run := types.RunRecord{RunID: "run-cal", AgentID: "engineering:a1", OwnerID: "user-alice", ComputerID: rt.TextureComputerID()}
	if err := s.CreateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	exec := toolregistry.ExecutionContext{RunID: run.RunID, AgentID: run.AgentID, OwnerID: run.OwnerID, ComputerID: run.ComputerID, RunRecord: &run}
	never := types.CommitmentRecord{RecordID: "never"}
	rt.stampCalibrationRead(&never, "run-other")
	if never.SubjectRefs[calibrationReadAtRef] != "" {
		t.Fatalf("a run that never read stamped %v", never.SubjectRefs)
	}
	digest := &types.CalibrationDigest{N: 25, Text: "record"}
	rt.noteCalibrationRead(ctx, exec, digest, calibrationDeliveryPush)
	claim := types.CommitmentRecord{RecordID: "c1"}
	rt.stampCalibrationRead(&claim, run.RunID)
	rt.noteCalibrationPredicted(run.RunID, time.Now())
	rt.noteCalibrationRead(ctx, exec, digest, calibrationDeliveryPush)
	if claim.SubjectRefs[calibrationReadAtRef] == "" || claim.SubjectRefs[calibrationReadsRef] != "1" {
		t.Fatalf("claim refs %v", claim.SubjectRefs)
	}
	evs, err := s.ListEvents(ctx, run.RunID, 100)
	if err != nil {
		t.Fatal(err)
	}
	var before []any
	for _, ev := range evs {
		if ev.Kind != types.EventCalibrationRead {
			continue
		}
		var p map[string]any
		_ = json.Unmarshal(ev.Payload, &p)
		if p["digest_hash"] == "" || p["mode"] != calibrationDeliveryPush {
			t.Fatalf("read mark %v", p)
		}
		before = append(before, p["before_predict"])
	}
	if len(before) != 2 || before[0] == before[1] {
		t.Fatalf("before_predict over two reads = %v, want one true and one false", before)
	}
	run.State = types.RunCompleted
	rt.recheckRecordsInbox(ctx, &run)
	if _, ok := rt.calibrationRunReads.Load(run.RunID); ok {
		t.Fatal("the read state outlived its run")
	}
}
