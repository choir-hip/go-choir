package yaegikernel

import (
	"fmt"
	"strings"
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
	// IntentOutcome is the cell's own result notice to its activation desk.
	// It is a distinct kind, not a MsgKind on IntentMessage, so a staged
	// message cannot claim the outcome envelope path and skip the assigned
	// desk's update authority.
	IntentOutcome = "outcome"
	// IntentFreeze stages the self-development freeze (commit_transaction
	// successor): the reducer freezes the capsule diff as a verifier-ready
	// effect bundle on cell return.
	IntentFreeze = "freeze"
	// IntentVerify stages the independent verifier decision
	// (record_self_development_verification successor): the reducer records
	// it against the exact frozen bundle on cell return.
	IntentVerify = "verify"
)

// Semantic-act intent kinds (commitment-ledger carrier, mission R2). These
// stage the same way as the existing kinds: they land in the tray in
// microseconds and the reducer authors the act on cell return. Cast/Ask/Note/
// Reply/Cancel/Escalate are operational acts (tracked, not scored);
// Precommit/Report/Resolve are epistemic acts (claims that resolve and score
// on the commitment ledger).
const (
	// IntentCast is delegated admission: open a downstream assignment under
	// the delegated-cast admission authority (not an owner revision).
	IntentCast = "cast"
	// IntentAsk is a directed query; it resolves on the target's Reply.
	IntentAsk = "ask"
	// IntentNote is raw unscored transport (the successor to Message's
	// unstructured use).
	IntentNote = "note"
	// IntentReply answers a staged Ask.
	IntentReply = "reply"
	// IntentCancel retracts a commitment; IntentEscalate surfaces an issue to
	// management or the owner. Both are operational, unscored.
	IntentCancel   = "cancel"
	IntentEscalate = "escalate"
	// IntentPrecommit freezes a typed prediction on the commitment ledger.
	IntentPrecommit = "precommit"
	// IntentReport asserts a typed claim with evidence and names its
	// resolver; it resolves when that resolver's acceptance lands.
	IntentReport = "report"
	// IntentResolve is the named resolver's act closing a Report/Ask.
	IntentResolve = "resolve"
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
	// refs against the mounted frozen bundle. BundleDigest binds the decision
	// to the exact bundle the cell inspected; the reducer asserts it equals
	// the operation's durable bundle digest.
	Decision     string   `json:"decision,omitempty"`
	VerifierRefs []string `json:"verifier_refs,omitempty"`
	BundleDigest string   `json:"bundle_digest,omitempty"`
	// Semantic-act fields (mission R2 commitment-ledger intents).
	// TargetRef binds a Resolve/Reply/Cancel to the act it closes (the
	// staged intent's canonical commitment or message id). Claim is a
	// Report's typed claim text. ResolverID names the desk/actor that must
	// accept a Report or resolve a Precommit. Question is an Ask's prompt;
	// Answer is a Reply's/Resolve's body. Outcome is a Resolve verdict.
	// Statement is a Precommit's frozen prediction body (JSON-encoded
	// CommitmentRecord prediction). Deadline is a resolve-by bound.
	TargetRef  string `json:"target_ref,omitempty"`
	Claim      string `json:"claim,omitempty"`
	ResolverID string `json:"resolver_id,omitempty"`
	Question   string `json:"question,omitempty"`
	Answer     string `json:"answer,omitempty"`
	OutcomeVal string `json:"outcome_val,omitempty"`
	Statement  string `json:"statement,omitempty"`
	Deadline   string `json:"deadline,omitempty"`
	// Actions carries an Escalate's guarded action schema (the execution_request
	// packet kind on the carrier): a desk requests management execute typed
	// actions under explicit safety annotations. JSON-encoded
	// []types.CoagentPacketAction; nil for a plain issue escalation.
	Actions string `json:"actions,omitempty"`
	// Packet carries a Report's full coagent source-packet body (the
	// update_coagent packet schema surviving on the carrier) — JSON-encoded
	// types.CoagentSourcePacketPayload. Set by ReportPacket; empty for a thin
	// claim-only Report.
	Packet string `json:"packet,omitempty"`
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
	// The verdict is a typed enum, not free text: implementation assignments
	// carry "none"; only the verifier slot issues pass/fail/abstain. A
	// non-enum verdict (e.g. summary prose passed positionally) is rejected
	// here so the model sees the error in-cell and can retry — the store's
	// ValidateAgainst would otherwise strand the terminal saga after
	// freeze+revoke with no recovery path.
	switch verdict {
	case "", "none", "pass", "fail", "abstain":
	default:
		return fmt.Errorf("tray: complete verdict %q not in {none, pass, fail, abstain}; implementation assignments use \"none\"", verdict)
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
// bundleDigest is the exact content digest the cell inspected; the reducer
// asserts it equals the operation's durable bundle digest, so a decision can
// never land on a bundle other than the one the cell examined.
func (t *Tray) Verify(decision string, verifierRefs []string, bundleDigest string) error {
	switch decision {
	case "pass", "fail":
	default:
		return fmt.Errorf("tray: verify decision %q not in {pass, fail}", decision)
	}
	if strings.TrimSpace(bundleDigest) == "" {
		return fmt.Errorf("tray: verify requires the inspected bundle digest")
	}
	for _, in := range t.intents {
		if in.Kind == IntentVerify {
			return fmt.Errorf("tray: at most one verify per cell")
		}
	}
	_, err := t.stage(StagedIntent{Kind: IntentVerify, Decision: decision, VerifierRefs: verifierRefs, BundleDigest: bundleDigest})
	return err
}

// Outcome stages the cell's own result notice to its activation desk. It is a
// distinct intent kind, not a MsgKind on Message, so a staged message cannot
// claim the outcome envelope path and skip the assigned desk's update
// authority.
func (t *Tray) Outcome(toDesk, body string) (string, error) {
	if toDesk == "" {
		return "", fmt.Errorf("tray: outcome requires a destination desk")
	}
	return t.stage(StagedIntent{Kind: IntentOutcome, ToDesk: toDesk, Body: body})
}

// --- Semantic-act tray methods (mission R2). All stage a single intent and
// return the cell-local correlation id; the reducer authors the act and its
// commitment-ledger record on cell return.

// Cast stages delegated admission: open a downstream assignment for
// (desk, objective) under the delegated-cast admission authority. Spec is an
// optional structured payload (JSON) the assignment binds.
func (t *Tray) Cast(desk, objective, spec string) (string, error) {
	if desk == "" || objective == "" {
		return "", fmt.Errorf("tray: cast requires a desk and objective")
	}
	return t.stage(StagedIntent{Kind: IntentCast, ToDesk: desk, Objective: objective, Statement: spec})
}

// Ask stages a directed query to a desk; it resolves on the target's Reply.
func (t *Tray) Ask(toDesk, question string) (string, error) {
	if toDesk == "" || question == "" {
		return "", fmt.Errorf("tray: ask requires a desk and a question")
	}
	return t.stage(StagedIntent{Kind: IntentAsk, ToDesk: toDesk, Question: question})
}

// Note stages raw unscored transport to a desk (successor to unstructured
// Message). Returns the cell-local correlation id.
func (t *Tray) Note(toDesk, body string) (string, error) {
	if toDesk == "" {
		return "", fmt.Errorf("tray: note requires a destination desk")
	}
	return t.stage(StagedIntent{Kind: IntentNote, ToDesk: toDesk, Body: body})
}

// Reply answers a staged Ask; targetRef is the Ask's correlation id.
func (t *Tray) Reply(toDesk, targetRef, answer string) (string, error) {
	if toDesk == "" || targetRef == "" {
		return "", fmt.Errorf("tray: reply requires a desk and the ask's target ref")
	}
	return t.stage(StagedIntent{Kind: IntentReply, ToDesk: toDesk, TargetRef: targetRef, Answer: answer})
}

// Cancel retracts a commitment by its correlation/record ref.
func (t *Tray) Cancel(targetRef string) (string, error) {
	if targetRef == "" {
		return "", fmt.Errorf("tray: cancel requires the commitment ref")
	}
	return t.stage(StagedIntent{Kind: IntentCancel, TargetRef: targetRef})
}

// Escalate surfaces an issue to management or the owner.
func (t *Tray) Escalate(toDesk, issue string) (string, error) {
	if toDesk == "" || issue == "" {
		return "", fmt.Errorf("tray: escalate requires a target and an issue")
	}
	return t.stage(StagedIntent{Kind: IntentEscalate, ToDesk: toDesk, Body: issue})
}

// EscalateActions stages a privileged-execution escalation: the desk asks
// management to run a set of guarded actions (the execution_request packet
// kind on the carrier). actionsJSON is a JSON-encoded
// []types.CoagentPacketAction; each action must carry explicit safety
// annotations (mutation_class, network, file_mutation). The reducer validates
// the schema before mailing.
func (t *Tray) EscalateActions(toDesk, issue, actionsJSON string) (string, error) {
	if toDesk == "" || issue == "" {
		return "", fmt.Errorf("tray: escalate_actions requires a target and an issue")
	}
	if strings.TrimSpace(actionsJSON) == "" {
		return "", fmt.Errorf("tray: escalate_actions requires a non-empty actions array")
	}
	return t.stage(StagedIntent{Kind: IntentEscalate, ToDesk: toDesk, Body: issue, Actions: actionsJSON})
}

// Precommit freezes a typed prediction on the commitment ledger. statement is
// the JSON-encoded CommitmentRecord prediction body; resolverID names who
// resolves it; deadline bounds resolution.
func (t *Tray) Precommit(statement, resolverID, deadline string) (string, error) {
	if statement == "" {
		return "", fmt.Errorf("tray: precommit requires a frozen prediction statement")
	}
	return t.stage(StagedIntent{Kind: IntentPrecommit, Statement: statement, ResolverID: resolverID, Deadline: deadline})
}

// Report asserts a typed claim with evidence refs and names its resolver.
func (t *Tray) Report(toDesk, claim string, evidenceRefs []string, resolverID string) (string, error) {
	if toDesk == "" || claim == "" {
		return "", fmt.Errorf("tray: report requires a desk and a claim")
	}
	return t.stage(StagedIntent{Kind: IntentReport, ToDesk: toDesk, Claim: claim, EvidenceRefs: evidenceRefs, ResolverID: resolverID})
}

// ReportPacket stages a report whose body is the full coagent source-packet
// schema — the update_coagent packet contract surviving as Report's body
// (mission R2). packetJSON is a JSON-encoded types.CoagentSourcePacketPayload;
// the reducer validates it (schema_version, kind, claims/sources/actions/
// questions) before the act commits, preserving the packet's safety contract.
func (t *Tray) ReportPacket(toDesk, packetJSON, resolverID string) (string, error) {
	if toDesk == "" || strings.TrimSpace(packetJSON) == "" {
		return "", fmt.Errorf("tray: report_packet requires a desk and a packet body")
	}
	return t.stage(StagedIntent{Kind: IntentReport, ToDesk: toDesk, Packet: packetJSON, ResolverID: resolverID})
}

// Resolve is the named resolver's act closing a Report/Ask/Precommit;
// targetRef is the act being resolved and outcome is the verdict.
func (t *Tray) Resolve(targetRef, outcome string) (string, error) {
	if targetRef == "" || outcome == "" {
		return "", fmt.Errorf("tray: resolve requires the act ref and an outcome")
	}
	return t.stage(StagedIntent{Kind: IntentResolve, TargetRef: targetRef, OutcomeVal: outcome})
}

func (t *Tray) stage(in StagedIntent) (string, error) {
	if len(t.intents) >= MaxIntentsPerCell {
		return "", fmt.Errorf("tray: cell intent quota exceeded (%d)", MaxIntentsPerCell)
	}
	size := len(in.Body) + len(in.Objective) + len(in.Summary) + len(in.Verdict) + len(in.BuildRecipeRef) + len(in.Decision) +
		len(in.TargetRef) + len(in.Claim) + len(in.ResolverID) + len(in.Question) + len(in.Answer) + len(in.OutcomeVal) +
		len(in.Statement) + len(in.Deadline)
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
