package yaegikernel

import (
	"fmt"
	"time"
)

// delegation, and completion stage into a per-cell in-memory tray in
// microseconds; the tray ships with the cell result and the guest daemon
// (autoputer) reduces it after the cell succeeds. Local IDs are tray
// bookkeeping, never delivery receipts.

// Intent kinds staged in the tray.
const (
	IntentMessage  = "message"
	IntentSpawn    = "spawn"
	IntentComplete = "complete"
	// IntentFreeze stages the self-development freeze (commit_transaction
	// successor): the reducer freezes the capsule diff as a verifier-ready
	// effect bundle on cell return.
	IntentFreeze = "freeze"
	// IntentVerify stages the independent verifier decision
	// (record_self_development_verification successor): the reducer records
	// it against the exact frozen bundle on cell return.
	IntentVerify = "verify"
)

// Completion results for IntentComplete.
const (
	CompleteCompleted = "completed"
	CompleteFailed    = "failed"
	CompleteBlocked   = "blocked"
	CompletePartial   = "partial"
)

// Tray quotas bound every cell: at most 16 staged intents, 16 KiB per message
// body, 256 KiB aggregate tray payload. The broker rejects cells that exceed
// them before they reach durable state.
const (
	MaxIntentsPerCell = 16
	MaxIntentBody     = 16 << 10
	MaxTrayBytes      = 256 << 10
)

// IncomingMessage is one mailbox message delivered as a cell-start snapshot.
// The snapshot is injected by autoputer at cell launch; Inbox() reads it
// side-effect-free inside the cell without network roundtrips.
type IncomingMessage struct {
	ID           string    `json:"id"`
	FromDesk     string    `json:"from_desk"`
	ToDesk       string    `json:"to_desk"`
	Kind         string    `json:"kind"`
	CreatedAt    time.Time `json:"created_at"`
	EvidenceRefs []string  `json:"evidence_refs,omitempty"`
	Body         string    `json:"body"`
}

// StagedIntent is one non-blocking in-cell request awaiting post-cell
// reduction. LocalID correlates the intent within its cell only.
type StagedIntent struct {
	LocalID       string   `json:"local_id"`
	Kind          string   `json:"kind"`
	ToDesk        string   `json:"to_desk,omitempty"`
	MsgKind       string   `json:"msg_kind,omitempty"`
	Body          string   `json:"body,omitempty"`
	Role          string   `json:"role,omitempty"`
	Objective     string   `json:"objective,omitempty"`
	Result        string   `json:"result,omitempty"`
	Verdict       string   `json:"verdict,omitempty"`
	Summary       string   `json:"summary,omitempty"`
	EvidenceRefs  []string `json:"evidence_refs,omitempty"`
	ExecutionRefs []string `json:"execution_refs,omitempty"`
	// Freeze fields (IntentFreeze): the build recipe ref, test receipts, and
	// dependency/toolchain refs the frozen bundle binds.
	BuildRecipeRef          string   `json:"build_recipe_ref,omitempty"`
	TestReceipts            []string `json:"test_receipts,omitempty"`
	DependencyToolchainRefs []string `json:"dependency_toolchain_refs,omitempty"`
	// Verify fields (IntentVerify): the verifier decision and its evidence
	// refs against the mounted frozen bundle.
	Decision     string   `json:"decision,omitempty"`
	VerifierRefs []string `json:"verifier_refs,omitempty"`
}

// Tray stages one cell's outbound intents. It is not safe for concurrent use:
// cells execute serially on one interpreter. Methods return immediately and
// never touch the network.
type Tray struct {
	intents []StagedIntent
	bytes   int
	next    int
}

// Message stages an asynchronous outbound message to a peer desk (mesh) or
// parent (fan-in). Non-blocking; returns a cell-local correlation ID.
func (t *Tray) Message(toDesk, body string) (string, error) {
	if toDesk == "" {
		return "", fmt.Errorf("tray: message requires a destination desk")
	}
	return t.stage(StagedIntent{Kind: IntentMessage, ToDesk: toDesk, Body: body})
}

// Spawn stages an asynchronous subtask delegation within role policy.
// Non-blocking; returns a cell-local child handle.
func (t *Tray) Spawn(role, objective string) (string, error) {
	if role == "" || objective == "" {
		return "", fmt.Errorf("tray: spawn requires a role and objective")
	}
	return t.stage(StagedIntent{Kind: IntentSpawn, Role: role, Objective: objective})
}

// Complete stages the assignment verdict. At most one complete intent is
// permitted per cell; the reducer binds execution receipts to it.
// executionRefs carries the exact capsule-go-eval receipt_ref strings the
// assignment's evidence binds to; a terminal completed pass requires at
// least one. result may be completed, failed, blocked, or partial.
func (t *Tray) Complete(result, verdict, summary string, evidenceRefs, executionRefs []string) error {
	switch result {
	case CompleteCompleted, CompleteFailed, CompleteBlocked, CompletePartial:
	default:
		return fmt.Errorf("tray: complete result %q not in {completed, failed, blocked, partial}", result)
	}
	for _, in := range t.intents {
		if in.Kind == IntentComplete {
			return fmt.Errorf("tray: at most one complete per cell")
		}
	}
	_, err := t.stage(StagedIntent{Kind: IntentComplete, Result: result, Verdict: verdict, Summary: summary, EvidenceRefs: evidenceRefs, ExecutionRefs: executionRefs})
	return err
}

// Freeze stages the self-development freeze request. At most one freeze per
// cell; the reducer runs the freeze on cell return using the cell's bound
// capsule handle (never model input).
func (t *Tray) Freeze(buildRecipeRef string, testReceipts, dependencyToolchainRefs []string) error {
	for _, in := range t.intents {
		if in.Kind == IntentFreeze {
			return fmt.Errorf("tray: at most one freeze per cell")
		}
	}
	_, err := t.stage(StagedIntent{Kind: IntentFreeze, BuildRecipeRef: buildRecipeRef, TestReceipts: testReceipts, DependencyToolchainRefs: dependencyToolchainRefs})
	return err
}

// Verify stages the independent verifier decision for the mounted frozen
// bundle. At most one verify per cell; the reducer records it on cell
// return. decision is pass or fail; verifierRefs are the verifier's evidence.
func (t *Tray) Verify(decision string, verifierRefs []string) error {
	switch decision {
	case "pass", "fail":
	default:
		return fmt.Errorf("tray: verify decision %q not in {pass, fail}", decision)
	}
	for _, in := range t.intents {
		if in.Kind == IntentVerify {
			return fmt.Errorf("tray: at most one verify per cell")
		}
	}
	_, err := t.stage(StagedIntent{Kind: IntentVerify, Decision: decision, VerifierRefs: verifierRefs})
	return err
}

func (t *Tray) stage(in StagedIntent) (string, error) {
	if len(t.intents) >= MaxIntentsPerCell {
		return "", fmt.Errorf("tray: cell intent quota exceeded (%d)", MaxIntentsPerCell)
	}
	size := len(in.Body) + len(in.Objective) + len(in.Summary) + len(in.Verdict) + len(in.BuildRecipeRef) + len(in.Decision)
	for _, refs := range [][]string{in.EvidenceRefs, in.ExecutionRefs, in.TestReceipts, in.DependencyToolchainRefs, in.VerifierRefs} {
		for _, ref := range refs {
			size += len(ref)
		}
	}
	if len(in.Body) > MaxIntentBody {
		return "", fmt.Errorf("tray: message body %d bytes exceeds %d", len(in.Body), MaxIntentBody)
	}
	if t.bytes+size > MaxTrayBytes {
		return "", fmt.Errorf("tray: aggregate payload exceeds %d bytes", MaxTrayBytes)
	}
	t.next++
	in.LocalID = fmt.Sprintf("tray-%d", t.next)
	t.intents = append(t.intents, in)
	t.bytes += size
	return in.LocalID, nil
}

// Drain returns the staged intents exactly once. Reduction ships the drained
// batch; undrained trays die with their cell and are never retried.
func (t *Tray) Drain() []StagedIntent {
	out := t.intents
	t.intents = nil
	return out
}

// Len reports staged intent count without draining.
func (t *Tray) Len() int {
	return len(t.intents)
}

// CellHooks binds one cell's tray and inbox snapshot to the choir scope:
// Begin runs at cell launch (snapshot in, fresh tray), End runs at cell
// completion (tray out for reduction). The session loop owns the hook
// lifetime; the scope only stages while bound.
type CellHooks struct {
	Begin func(frame SessionFrame)
	End   func() []StagedIntent
}
