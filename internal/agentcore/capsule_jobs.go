package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsulejob"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Capsule jobs, runtime side (docs/design/capsule-long-exec-2026-10-15.md).
// A cell stages a job start or stop (choir.Start, choir.Stop); the reducer
// hands it to the capsule executor after the cell's acts commit. When a job
// ends the executor calls handleCapsuleJobDone, which commits a job record
// into the desk's inbox (engineering:<assignment>, from job:<id>) and wakes
// the run: an engineering turn that ended while its job ran is held by the
// completion guard (engineeringJobWait) and resumed with the outcome.

// Notify keys the reducer gives a job start, returned on its completion.
const (
	jobNotifyOwner    = "owner_id"
	jobNotifyComputer = "computer_id"
	jobNotifyInbox    = "inbox"
	jobNotifyRun      = "run_id"
)

// capsuleJobTable is the executor's view of a run's jobs the completion
// guard reads; tests replace it.
type capsuleJobTable interface {
	RunningJob(agentRunID string) (yaegikernel.JobView, time.Time, bool)
	JobViews(agentRunID string) []yaegikernel.JobView
}

func (rt *Runtime) capsuleJobs() capsuleJobTable {
	if rt.jobTable != nil {
		return rt.jobTable
	}
	if rt.capsuleExecutor == nil {
		return nil
	}
	return rt.capsuleExecutor
}

// jobWaitSlack is how long past a job's wall clock the guard waits for its
// end (the supervisor's grace and reap, the watcher's poll).
const jobWaitSlack = time.Minute

// validateJobIntent checks a staged job start or stop at the reducer's
// trust boundary.
func validateJobIntent(fromDesk string, in yaegikernel.StagedIntent) error {
	if fromDesk != agentprofile.Engineering {
		return fmt.Errorf("reduce: job %s: capsule jobs are engineering's", in.LocalID)
	}
	if !capsulejob.ValidID(in.JobID) {
		return fmt.Errorf("reduce: job %s: invalid job id %q", in.LocalID, in.JobID)
	}
	switch in.JobOp {
	case yaegikernel.JobOpStart:
		if _, err := (capsulejob.Spec{ID: in.JobID, Argv: in.Argv, Cwd: in.Cwd, WallClockMS: in.WallClockMS, MemoryMiB: in.MemoryMiB}).Normalize(); err != nil {
			return fmt.Errorf("reduce: job %s: %w", in.LocalID, err)
		}
	case yaegikernel.JobOpStop:
	default:
		return fmt.Errorf("reduce: job %s: operation %q is not start or stop", in.LocalID, in.JobOp)
	}
	return nil
}

// commitJobIntent hands a staged job start or stop to the executor. It
// never fails the cell: a refused start (another job running, the capsule
// gone) is reported in the cell's job_results, and nothing else in the cell
// depends on it.
func (r *rlmCallReduction) commitJobIntent(ctx context.Context, in yaegikernel.StagedIntent) map[string]any {
	out := map[string]any{"job_id": in.JobID, "op": in.JobOp}
	toolCtx, err := requireCapsuleMutationRole(ctx)
	if err == nil && (toolCtx.Executor == nil || toolCtx.CapsuleHandle == "") {
		err = fmt.Errorf("capsule job authority unavailable")
	}
	if err != nil {
		out["error"] = err.Error()
		return out
	}
	jobCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
	defer cancel()
	var view yaegikernel.JobView
	switch in.JobOp {
	case yaegikernel.JobOpStart:
		view, err = toolCtx.Executor.StartJob(jobCtx, toolCtx.AgentRunID, toolCtx.CapsuleHandle, jobStartRequest(in, map[string]string{
			jobNotifyOwner: r.scope.OwnerID, jobNotifyComputer: r.scope.ComputerID,
			jobNotifyInbox: r.scope.DeskAgentID, jobNotifyRun: r.scope.RunID,
		}))
	default:
		view, err = toolCtx.Executor.StopJob(jobCtx, toolCtx.AgentRunID, toolCtx.CapsuleHandle, in.JobID)
	}
	if err != nil {
		out["error"] = err.Error()
		return out
	}
	out["state"] = view.State
	if in.JobOp == yaegikernel.JobOpStart {
		out["started"] = view.State == capsulejob.StateRunning
		if view.State == capsulejob.StateRunning {
			out["next"] = "the job runs now; do other work or end your turn: you are woken when it ends, its record arrives in choir.Records(), and choir.Poll(id) shows its receipt_ref"
		} else if view.Reason != "" {
			out["error"] = view.Reason
		}
	}
	return out
}

// jobStartRequest is the executor's start request for a staged job start,
// carrying the cell's declared memory need.
func jobStartRequest(in yaegikernel.StagedIntent, notify map[string]string) capsule.JobRequest {
	return capsule.JobRequest{ID: in.JobID, Argv: in.Argv, Cwd: in.Cwd, WallClockMS: in.WallClockMS, MemoryMiB: in.MemoryMiB, Notify: notify}
}

// jobEndRecordRequest builds the inbox command for a job's end record: a
// system record from job:<id> to the desk that started it, deterministic in
// the job, so a retried commit replays.
func jobEndRecordRequest(ownerID, computerID, inbox string, c capsule.JobCompletion) (types.CommitInboxRecordsRequest, error) {
	st := c.Status
	body := &types.CommitmentJob{
		JobID: st.ID, State: st.State, ExitCode: st.ExitCode, Argv: append([]string(nil), st.Argv...), Cwd: st.Cwd,
		Reason: st.Reason, OutputSHA256: st.OutputSHA256, OutputBytes: st.OutputBytes,
		LogRef: c.View.LogRef, Tail: c.View.Tail,
	}
	if c.Receipt != nil {
		body.ReceiptRef = c.Receipt.ReceiptRef
	}
	if !st.StartedAt.IsZero() {
		body.StartedAt = st.StartedAt.UTC().Format(time.RFC3339Nano)
	}
	if !st.FinishedAt.IsZero() {
		body.FinishedAt = st.FinishedAt.UTC().Format(time.RFC3339Nano)
	}
	rec := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: types.JobRecordID(st.ID),
		Kind: types.CommitmentKindReport, Discrepancy: types.DiscrepancyUnresolved,
		Provenance:  types.CommitmentProvenance{AgentID: types.JobSystemAgentID},
		EmitterKind: types.CommitmentEmitterSystem, ComputerID: computerID,
		Inbox: inbox, SenderInbox: types.JobSenderInbox(st.ID), Addressee: inbox,
		SubjectRefs: map[string]string{"job_id": st.ID},
		Job:         body,
	}
	if body.ReceiptRef != "" {
		rec.EvidenceRefs = []string{body.ReceiptRef}
	}
	req := types.CommitInboxRecordsRequest{OwnerID: ownerID, ComputerID: computerID, CommandID: "job-ended:" + st.ID, Records: []types.CommitmentRecord{rec}}
	digest, err := store.ComputeCommitInboxRecordsDigest(req)
	if err != nil {
		return types.CommitInboxRecordsRequest{}, err
	}
	req.CommandDigest = digest
	return req, nil
}

// handleCapsuleJobDone is the executor's completion handler: it commits the
// job's end record into the desk's inbox and wakes the run.
func (rt *Runtime) handleCapsuleJobDone(c capsule.JobCompletion) {
	if rt == nil {
		return
	}
	ownerID, computerID := c.Notify[jobNotifyOwner], c.Notify[jobNotifyComputer]
	inbox, runID := c.Notify[jobNotifyInbox], c.Notify[jobNotifyRun]
	if rt.store != nil && ownerID != "" && computerID != "" && strings.HasPrefix(inbox, agentprofile.Engineering+":") {
		req, err := jobEndRecordRequest(ownerID, computerID, inbox, c)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_, err = rt.store.CommitInboxRecords(ctx, req)
			cancel()
		}
		if err != nil {
			log.Printf("runtime: job %s ended %s; its record to %s: %v", c.Status.ID, c.Status.State, inbox, err)
		}
	}
	if runID == "" {
		runID = c.AgentRunID
	}
	rt.signalJobWake(runID)
}

// jobWake returns the run's job wake channel (one pending signal).
func (rt *Runtime) jobWake(runID string) chan struct{} {
	ch, _ := rt.jobWakes.LoadOrStore(runID, make(chan struct{}, 1))
	return ch.(chan struct{})
}

func (rt *Runtime) signalJobWake(runID string) {
	if runID == "" {
		return
	}
	select {
	case rt.jobWake(runID) <- struct{}{}:
	default:
	}
}

// engineeringJobWait is the completion guard's wait (F15-F18): when an
// engineering turn ends without a terminal fate while its capsule runs a
// job, it waits for the job's end (bounded by the run's context and the
// job's wall clock) and resumes the turn with the outcome. ok is false when
// the run has no running job.
func (rt *Runtime) engineeringJobWait(ctx context.Context, rec *types.RunRecord) (toolregistry.ToolLoopCompletionGuardResult, bool) {
	if rt == nil || rec == nil {
		return toolregistry.ToolLoopCompletionGuardResult{}, false
	}
	jobs := rt.capsuleJobs()
	if jobs == nil {
		return toolregistry.ToolLoopCompletionGuardResult{}, false
	}
	view, deadline, running := jobs.RunningJob(rec.RunID)
	if !running {
		return toolregistry.ToolLoopCompletionGuardResult{}, false
	}
	ended, reason, err := rt.waitJobEnd(ctx, jobs, rec.RunID, view, deadline)
	if err != nil || reason != jobParkEnded {
		return toolregistry.ToolLoopCompletionGuardResult{}, false
	}
	return toolregistry.ToolLoopCompletionGuardResult{Continue: true, Waited: true, Reason: jobParkEnded, Instruction: jobEndedInstruction(ended)}, true
}

// How a wait on a job ended.
const (
	jobParkEnded        = "capsule_job_ended"
	jobParkAlreadyEnded = "already_ended"
	jobParkDeadline     = "deadline"
	jobParkCancelled    = "cancelled"
)

// waitJobEnd waits for the run's running job to end: woken by its end
// signal (a stale signal from an earlier job does not release it), bounded
// by the job's wall clock plus jobWaitSlack and by ctx. It returns the
// job's latest view and how the wait ended; err is ctx's.
func (rt *Runtime) waitJobEnd(ctx context.Context, jobs capsuleJobTable, runID string, view yaegikernel.JobView, deadline time.Time) (yaegikernel.JobView, string, error) {
	timer := time.NewTimer(time.Until(deadline.Add(jobWaitSlack)))
	defer timer.Stop()
	wake := rt.jobWake(runID)
	for {
		select {
		case <-ctx.Done():
			return view, jobParkCancelled, ctx.Err()
		case <-timer.C:
			return latestJobView(jobs, runID, view), jobParkDeadline, nil
		case <-wake:
		}
		if _, _, still := jobs.RunningJob(runID); still {
			continue // a stale signal from an earlier job
		}
		return latestJobView(jobs, runID, view), jobParkEnded, nil
	}
}

// latestJobView is the run's current view of job view.ID.
func latestJobView(jobs capsuleJobTable, runID string, view yaegikernel.JobView) yaegikernel.JobView {
	for _, v := range jobs.JobViews(runID) {
		if v.ID == view.ID {
			return v
		}
	}
	return view
}

// The busy-poll fix (docs/problems/engineering-busy-polls-its-own-job-2026-10-24.md):
// a cell that looked at the run's own job while it ran parks the turn on
// the job's end. The tool loop calls no model in between (its batch wait),
// so "end your turn and be woken" is what happens, not what is asked.

// cellJobPark is the job a cell's turn parks on: the first job its Poll
// returned unfinished, unless the cell ended the assignment (a terminal
// fate) or staged a Stop (the job is ending). A cell that polled and then
// failed still waited on the job.
func cellJobPark(result capsule.GoEvalResult) string {
	if result.FateTerminal || len(result.JobsAwaited) == 0 {
		return ""
	}
	for _, in := range result.Intents {
		if in.Kind == yaegikernel.IntentJob && in.JobOp == yaegikernel.JobOpStop {
			return ""
		}
	}
	return strings.TrimSpace(result.JobsAwaited[0])
}

// engineeringJobPark is engineering's tool-loop batch wait. It parks when
// the batch's capsule_go_eval named a job_park (cause "poll") or was the
// Complete refused because the job runs (cause "complete_refused"), waits
// for the job's end, and resumes the turn with its outcome. A job that
// already ended resumes at once. The park's start (capsule_job_park) and
// end (capsule_job_park_ended) are run progress events. It holds nothing
// durable: a crash ends it with the process, and the boot closes the job as
// interrupted and the run under the restart rule.
func (rt *Runtime) engineeringJobPark(ctx context.Context, rec *types.RunRecord, emit provideriface.EventEmitFunc, calls []types.ToolCall, results []types.ToolResult) (toolregistry.ToolLoopCompletionGuardResult, bool, error) {
	none := toolregistry.ToolLoopCompletionGuardResult{}
	if rt == nil || rec == nil {
		return none, false, nil
	}
	jobs := rt.capsuleJobs()
	if jobs == nil {
		return none, false, nil
	}
	jobID, cause := "", ""
	for i := 0; i < len(calls) && i < len(results); i++ {
		if strings.TrimSpace(calls[i].Name) != "capsule_go_eval" {
			continue
		}
		if results[i].IsError {
			if strings.Contains(results[i].Output, completeWaitsForJobMarker) {
				if running, _, ok := jobs.RunningJob(rec.RunID); ok {
					jobID, cause = running.ID, "complete_refused"
				}
			}
			continue
		}
		var out struct {
			JobPark string `json:"job_park"`
		}
		if json.Unmarshal([]byte(results[i].Output), &out) == nil && strings.TrimSpace(out.JobPark) != "" {
			jobID, cause = strings.TrimSpace(out.JobPark), "poll"
		}
	}
	if jobID == "" {
		return none, false, nil
	}
	view, deadline, running := jobs.RunningJob(rec.RunID)
	if !running || view.ID != jobID {
		// The job ended between the cell and the park.
		for _, v := range jobs.JobViews(rec.RunID) {
			if v.ID == jobID && capsulejob.Terminal(v.State) {
				emitJobPark(emit, "capsule_job_park", map[string]any{"job_id": jobID, "cause": cause})
				emitJobPark(emit, "capsule_job_park_ended", map[string]any{"job_id": jobID, "reason": jobParkAlreadyEnded, "state": v.State, "waited_ms": 0})
				return toolregistry.ToolLoopCompletionGuardResult{Continue: true, Waited: true, Reason: jobParkEnded, Instruction: jobEndedInstruction(v)}, true, nil
			}
		}
		return none, false, nil
	}
	emitJobPark(emit, "capsule_job_park", map[string]any{"job_id": jobID, "cause": cause, "deadline": deadline.Add(jobWaitSlack).UTC().Format(time.RFC3339)})
	started := time.Now()
	ended, reason, err := rt.waitJobEnd(ctx, jobs, rec.RunID, view, deadline)
	emitJobPark(emit, "capsule_job_park_ended", map[string]any{"job_id": jobID, "reason": reason, "state": ended.State, "waited_ms": time.Since(started).Milliseconds()})
	if err != nil {
		return none, false, err
	}
	instruction := jobEndedInstruction(ended)
	if reason == jobParkDeadline {
		instruction = jobOutlivedInstruction(ended)
	}
	return toolregistry.ToolLoopCompletionGuardResult{Continue: true, Waited: true, Reason: reason, Instruction: instruction}, true, nil
}

func emitJobPark(emit provideriface.EventEmitFunc, phase string, body map[string]any) {
	if emit == nil {
		return
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return
	}
	emit(types.EventRunProgress, phase, payload)
}

// jobOutlivedInstruction resumes a turn whose job was still running past
// its wall clock and the slack.
func jobOutlivedInstruction(v yaegikernel.JobView) string {
	return fmt.Sprintf("Your capsule job %s (%s) was still running past its wall clock of %d s. choir.Poll(%q) shows it now; if it hangs, choir.Stop it in a cell of its own and decide what to run instead.",
		v.ID, strings.Join(v.Argv, " "), v.WallClockSec, v.ID)
}

// jobEndedInstruction is the turn that resumes a desk when its job ends.
func jobEndedInstruction(v yaegikernel.JobView) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Your capsule job %s (%s) ended: state %s", v.ID, strings.Join(v.Argv, " "), v.State)
	if v.ExitCode != nil {
		fmt.Fprintf(&b, ", exit code %d", *v.ExitCode)
	}
	fmt.Fprintf(&b, ", after %d s.", v.ElapsedSec)
	if v.Reason != "" {
		fmt.Fprintf(&b, " %s.", v.Reason)
	}
	if v.ReceiptRef != "" {
		if v.State == capsulejob.StateDone {
			fmt.Fprintf(&b, " Its receipt is %s: cite it in choir.Freeze for the build or the tests it ran.", v.ReceiptRef)
		} else {
			fmt.Fprintf(&b, " Its receipt %s records a failure; Freeze refuses it as evidence. Read the output, fix the cause and run it again.", v.ReceiptRef)
		}
	}
	b.WriteString(" Its record is in choir.Records() and choir.Poll(\"" + v.ID + "\") shows its output tail. Continue the assignment in your next cell.")
	return b.String()
}
