package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Calibration read marks (C6 of
// docs/design/calibration-wiring-permutations-2026-11-04.md). A desk reads
// its record through choir.Calibration(); the cell's result says it did,
// and the host writes a read mark: a calibration.read run event (run, time,
// the how-manyth read, whether the run had predicted yet, the record's hash)
// and, on each rule claim the run states afterwards, the time of its first
// read. The read metrics (read_any, read_before_predict, read_latency_s,
// reads_per_run) are read off those two.
//
// Failure modes, written before the code:
//
//	M1 A read is inferred from choir.Pack(), which desks call for other
//	   reasons -> only choir.Calibration() marks a read.
//	M2 A read after the prediction counts as feedback -> the mark says
//	   whether the run had predicted (an engineer's Precommit, management's
//	   engineering Cast with its predictions), and each claim carries the
//	   first read that preceded it.
//	M3 A read in a cell that failed is lost -> every cell's result carries
//	   its mark, failed cells included.
//	M4 The state outlives its run -> it is dropped when the run ends; a
//	   crash ends the run, so it is never needed across a restart.
//	M5 The mark fails the cell -> it is best effort and logs.

// Claim refs a rule precommit carries when its run read its record first.
const (
	calibrationReadAtRef = "calibration_read_at"
	calibrationReadsRef  = "calibration_reads"
)

// calibrationRunReads is one run's read state.
type calibrationRunReads struct {
	mu          sync.Mutex
	firstReadAt time.Time
	reads       int
	predictedAt time.Time
	// restateAsked are the rule kinds a two-stage engineer was asked to
	// restate (E1-E), each once.
	restateAsked map[string]bool
}

func (rt *Runtime) calibrationReadsOf(runID string) *calibrationRunReads {
	runID = strings.TrimSpace(runID)
	if rt == nil || runID == "" {
		return nil
	}
	v, _ := rt.calibrationRunReads.LoadOrStore(runID, &calibrationRunReads{})
	return v.(*calibrationRunReads)
}

// calibrationDigestHash names the record a read saw, without its text.
func calibrationDigestHash(d *types.CalibrationDigest) string {
	if d == nil || d.Text == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(d.Text))
	return hex.EncodeToString(sum[:6])
}

// noteCalibrationRead writes a cell's read mark (M3, M5).
func (rt *Runtime) noteCalibrationRead(ctx context.Context, exec toolregistry.ExecutionContext, shown *types.CalibrationDigest, mode string) {
	state := rt.calibrationReadsOf(exec.RunID)
	if state == nil {
		return
	}
	now := time.Now().UTC()
	state.mu.Lock()
	state.reads++
	if state.firstReadAt.IsZero() {
		state.firstReadAt = now
	}
	reads, predictedAt := state.reads, state.predictedAt
	state.mu.Unlock()
	payload := map[string]any{
		"run_id": exec.RunID, "agent_id": exec.AgentID, "read_at": now, "read_n": reads,
		"before_predict": predictedAt.IsZero(), "mode": mode,
		"digest_hash": calibrationDigestHash(shown), "shown": shown != nil && shown.Text != "",
	}
	if shown != nil {
		payload["digest_n"] = shown.N
	}
	if !predictedAt.IsZero() {
		payload["first_predict_at"] = predictedAt
	}
	rt.recordDeskEvent(ctx, exec, types.EventCalibrationRead, payload)
}

// noteCalibrationPredicted marks the run's first prediction (M2).
func (rt *Runtime) noteCalibrationPredicted(runID string, at time.Time) {
	state := rt.calibrationReadsOf(runID)
	if state == nil {
		return
	}
	state.mu.Lock()
	if state.predictedAt.IsZero() {
		state.predictedAt = at.UTC()
	}
	state.mu.Unlock()
}

// stampCalibrationRead puts the run's first read on a claim it states,
// when the run read its record before it (M2).
func (rt *Runtime) stampCalibrationRead(rec *types.CommitmentRecord, runID string) {
	state := rt.calibrationReadsOf(runID)
	if state == nil || rec == nil {
		return
	}
	state.mu.Lock()
	first, reads := state.firstReadAt, state.reads
	state.mu.Unlock()
	if first.IsZero() {
		return
	}
	if rec.SubjectRefs == nil {
		rec.SubjectRefs = map[string]string{}
	}
	rec.SubjectRefs[calibrationReadAtRef] = first.Format(time.RFC3339Nano)
	rec.SubjectRefs[calibrationReadsRef] = strconv.Itoa(reads)
}

// dropCalibrationReads forgets an ended run's read state (M4).
func (rt *Runtime) dropCalibrationReads(runID string) {
	if rt != nil {
		rt.calibrationRunReads.Delete(strings.TrimSpace(runID))
	}
}
