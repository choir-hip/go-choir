package capsule

import (
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsulejob"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Capsule jobs, executor side (docs/design/capsule-long-exec-2026-10-15.md):
// the types and the receipt every platform builds; the lifecycle is Linux
// only (jobs.go).

// JobRequest is a job start as the reducer hands it to the executor. Notify
// rides back unchanged on the job's JobCompletion (the runtime's binding of
// the desk to wake); it is never model input.
type JobRequest struct {
	ID          string
	Argv        []string
	Cwd         string
	WallClockMS int64
	// MemoryMiB is the job's declared memory need beyond its capsule
	// (capsulejob.Spec.MemoryMiB).
	MemoryMiB int
	Notify    map[string]string
}

// JobParams are the broker's job verb parameters.
type JobParams struct {
	ID          string   `json:"id"`
	Argv        []string `json:"argv,omitempty"`
	Cwd         string   `json:"cwd,omitempty"`
	WallClockMS int64    `json:"wall_clock_ms,omitempty"`
}

// JobCompletion is what the executor hands the runtime when a job ends.
// Receipt is nil when the job ended without one (its capsule ended first).
type JobCompletion struct {
	AgentRunID string
	CapsuleID  string
	Notify     map[string]string
	View       yaegikernel.JobView
	Status     capsulejob.Status
	Receipt    *ExecutionReceipt
}

// JobAdmission is the seam for memory leases (virtio-mem,
// docs/archive/reports/memory-hotplug-controller-design-2026-10-14.md section 9):
// called before a job starts; release runs when its memory is returned
// (the job's end, or its capsule's when the boost was retained). A
// non-nil error refuses the start.
type JobAdmission func(capsuleID string, spec capsulejob.Spec) (release func(), err error)

// JobReceiptInput is everything a job's receipt binds. The status is the
// broker's (its memory, read over the capability-verified RPC); the
// worktree digests and the time are the executor's own.
type JobReceiptInput struct {
	AgentRunID          string
	Handle              string
	CapsuleID           string
	SourceTreeDigest    string
	StartWorktreeDigest string
	EndWorktreeDigest   string
	LogRef              string
	Status              capsulejob.Status
	ObservedAt          time.Time
}

// jobStderrDigest is the receipt's stderr digest: a job's stderr is merged
// into its one log, whose digest is StdoutDigest.
var jobStderrDigest = computerevent.DigestBytes(nil)

// JobCommand is the receipt's command line for a job: "job <id>: argv".
func JobCommand(id string, argv []string) string {
	return "job " + id + ": " + strings.Join(argv, " ")
}

// MintJobReceipt builds a job's execution receipt and the canonical bytes
// to persist. It is a capsule-exec receipt (the bundle validator and the
// receipt opener already accept the prefix): the command, argv, cwd,
// environment digest, exit code, output digest, timing, job state and both
// worktree digests are bound, with the run, handle, capsule and source that
// every execution receipt binds.
func MintJobReceipt(in JobReceiptInput) (ExecutionReceipt, []byte, error) {
	st := in.Status
	if !st.Terminal() {
		return ExecutionReceipt{}, nil, fmt.Errorf("job %s has not ended (%s)", st.ID, st.State)
	}
	exit := -1
	if st.ExitCode != nil {
		exit = *st.ExitCode
	}
	if st.State != capsulejob.StateDone && exit == 0 {
		// A stopped or timed-out job never reads as a clean exit.
		exit = -1
	}
	duration := int64(0)
	if !st.FinishedAt.IsZero() {
		duration = st.FinishedAt.Sub(st.StartedAt).Milliseconds()
	}
	receipt := ExecutionReceipt{
		AgentRunID: in.AgentRunID, CapabilityHandleDigest: computerevent.DigestBytes([]byte(in.Handle)),
		CapsuleID: in.CapsuleID, Command: JobCommand(st.ID, st.Argv), Cwd: st.Cwd, ExitCode: exit,
		StdoutDigest: st.OutputSHA256, StderrDigest: jobStderrDigest,
		WorktreeDigest: in.EndWorktreeDigest, SourceTreeDigest: in.SourceTreeDigest,
		OccurredAt: in.ObservedAt.UTC().Format(time.RFC3339Nano),
		JobID:      st.ID, JobState: st.State, Argv: append([]string(nil), st.Argv...), EnvDigest: st.EnvDigest,
		StartedAt: st.StartedAt.UTC().Format(time.RFC3339Nano), DurationMS: duration,
		OutputBytes: st.OutputBytes, OutputTruncated: st.Truncated,
		StartWorktreeDigest: in.StartWorktreeDigest, LogRef: in.LogRef,
	}
	canonical, err := computerevent.CanonicalJSON(receipt)
	if err != nil {
		return ExecutionReceipt{}, nil, err
	}
	receipt.ReceiptRef = "capsule-exec:sha256:" + computerevent.DigestBytes(canonical)
	stored, err := computerevent.CanonicalJSON(receipt)
	if err != nil {
		return ExecutionReceipt{}, nil, err
	}
	return receipt, stored, nil
}

// JobReceiptOutcomeRefusal says why a receipt cannot stand as evidence that
// its slot's work succeeded, or "" when it can: a non-zero exit code (a
// failed, timed-out or stopped command, or a cell that panicked), a job that
// did not end done, and, for a job in a build or test slot, a tree that
// changed while it ran.
func JobReceiptOutcomeRefusal(receipt ExecutionReceipt, buildOrTest bool) string {
	switch {
	case receipt.JobID != "" && receipt.JobState != capsulejob.StateDone:
		return fmt.Sprintf("job %s ended %s (exit code %d)", receipt.JobID, receipt.JobState, receipt.ExitCode)
	case receipt.ExitCode != 0:
		return fmt.Sprintf("its command exited %d", receipt.ExitCode)
	case buildOrTest && receipt.JobID != "" && receipt.StartWorktreeDigest != receipt.WorktreeDigest:
		return fmt.Sprintf("the workspace changed while job %s ran, so it did not build or test the tree it names", receipt.JobID)
	}
	return ""
}

// jobView projects a job's status (and receipt, once minted) for a cell.
func jobView(st capsulejob.Status, receiptRef, logRef string) yaegikernel.JobView {
	tail := st.Tail
	if len(tail) > capsulejob.TailBytes {
		tail = tail[len(tail)-capsulejob.TailBytes:]
	}
	return yaegikernel.JobView{
		ID: st.ID, State: st.State, Argv: append([]string(nil), st.Argv...), Cwd: st.Cwd, ExitCode: st.ExitCode,
		Reason: st.Reason, ElapsedSec: st.ElapsedMS / 1000, WallClockSec: st.WallClockMS / 1000,
		OutputBytes: st.OutputBytes, Tail: tail, ReceiptRef: receiptRef, LogRef: logRef,
	}
}
