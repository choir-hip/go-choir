package yaegikernel

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// delegation, and completion stage into a per-cell in-memory tray in
// microseconds; the tray ships with the cell result and the guest daemon
// (autoputer) reduces it after the cell succeeds. Local IDs are tray
// bookkeeping, never delivery receipts.

// Intent kinds staged in the tray.
const (
	IntentMessage  = "message"
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
	IntentCancel = "cancel"
	// IntentCancelAssignment revokes one engineering assignment capsule with
	// executor acknowledgement (SMG): the verb carrier for the deleted
	// cancel_co_super_assignment typed tool. Management desk only.
	IntentCancelAssignment = "cancel_assignment"
	IntentEscalate         = "escalate"
	// IntentPrecommit freezes a machine-scoreable prediction on the
	// commitment ledger.
	IntentPrecommit = "precommit"
	// IntentReport asserts a typed claim with evidence and names its
	// resolver; it resolves when that resolver's acceptance lands.
	IntentReport = "report"
	// IntentResolve records an evidence-bearing resolver verdict.
	IntentResolve = "resolve"
	// IntentDisagreement records a scorer verdict that differs from the
	// resolver verdict as a distinct supervision act.
	IntentDisagreement = "disagreement"
	// IntentTextureTurn is a Texture cell's turn (Cut 2 2a): the one intent
	// choir.Write, Done, Wait and Reject build together. Its Body is a
	// TextureTurn. The reducer commits it through the atomic
	// ApplyTextureTurn transaction; it is not a semantic act and mints no
	// mailbox envelope — the commit lands on the lifecycle/document head.
	IntentTextureTurn = "texture_turn"
	// IntentDelegate is management's batch delegation (management
	// delegation S2a, R2-14): its Body is a DelegateSpec. The reducer opens
	// one management task per group, each with a Cast into a new
	// management-task:<id> inbox, in the cell's one records transaction.
	IntentDelegate = "delegate"
	// IntentCancelTask cancels one management task (R2-20); TargetRef is
	// the task id, Body the reason.
	IntentCancelTask = "cancel_task"
	// IntentTaskDone ends a management task (R2-8); Body is its note. The
	// host then mints the task's final Report from its durable
	// dispositions.
	IntentTaskDone = "task_done"
)

// DelegateSpec is the body of a Delegate intent: the task kind, the
// objective each task is given, and the groups of record ids, one task per
// group.
type DelegateSpec struct {
	Kind      string     `json:"kind"`
	Objective string     `json:"objective"`
	Groups    [][]string `json:"groups"`
}

// Management task kinds and the coalescing cap (design §3.2, O7).
const (
	TaskKindIntake = "intake"
	TaskKindAnswer = "answer"
	// MaxTaskCaptures is the most captures one intake task is given.
	MaxTaskCaptures = 12
)

// TextureTurn is the body of a Texture cell's turn. A cell sets Write,
// Done, Wait and Rejects through its verbs; the runtime alone sets Idle,
// Handles, InboxRecords and InboxCursor.
type TextureTurn struct {
	// BaseRevisionID is the revision the cell's frame showed; a Write
	// applies to it and the commit refuses if the head moved.
	BaseRevisionID string `json:"base_revision_id,omitempty"`
	// Write revises the document.
	Write *TextureWrite `json:"write,omitempty"`
	// Done says the requested work is complete; its value is why.
	Done string `json:"done,omitempty"`
	// Wait records waiting for evidence without a revision; its value is
	// why.
	Wait string `json:"wait,omitempty"`
	// Rejects decline records or leftover update packets, each with a reason.
	Rejects []TextureReject `json:"rejects,omitempty"`
	// Idle is the runtime's turn for an activation that ended without one.
	Idle string `json:"idle,omitempty"`
	// Handles are the frame's record sources, set by the runtime from the
	// records the cell read.
	Handles map[string]texturewrite.Source `json:"handles,omitempty"`
	// InboxRecords and InboxCursor are the dispatch fold (Cut 1 L3a):
	// Texture's records to research and management and the cursor advance
	// over the range the cell read.
	InboxRecords []types.CommitmentRecord  `json:"inbox_records,omitempty"`
	InboxCursor  *types.InboxCursorAdvance `json:"inbox_cursor,omitempty"`
}

// TextureWrite is one Write: the edit script and why.
type TextureWrite struct {
	Edits string `json:"edits"`
	Why   string `json:"why"`
	// PSupported is the Write's self-score: Texture's probability that the
	// judge finds each new citation's passage supports its sentence
	// (docs/reports/icrl-calibration-experiment-2026-10-14.md). Nil when
	// the Write states none.
	PSupported *float64 `json:"p_supported,omitempty"`
}

// TextureReject declines one record (by record id) or one leftover update
// packet (by update id).
type TextureReject struct {
	RecordID string `json:"record_id,omitempty"`
	UpdateID string `json:"update_id,omitempty"`
	Why      string `json:"why"`
}

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

// InboxRecord is one record addressed to this desk's inbox, shown in the
// cell frame as the range (cursor, head] the cell reads (Cut 1, records are
// the only work state). The cell reads them through choir.Records(); the
// cursor advances only when the cell's acts commit, in the same
// transaction.
type InboxRecord struct {
	Seq      int64  `json:"seq"`
	RecordID string `json:"record_id"`
	// ID is RecordID under the name models reach for (cluster D,
	// docs/reports/cross-model-cell-errors-2026-10-13.md).
	ID             string                  `json:"-"`
	Kind           string                  `json:"kind"`
	From           string                  `json:"from"`
	ConversationID string                  `json:"conversation_id,omitempty"`
	Round          int                     `json:"round,omitempty"`
	Final          bool                    `json:"final,omitempty"`
	Objective      string                  `json:"objective,omitempty"`
	Body           string                  `json:"body,omitempty"`
	Cast           *types.CommitmentCast   `json:"cast,omitempty"`
	Report         *types.CommitmentReport `json:"report,omitempty"`
	// Support is a support list's pairs (to management) or management's
	// failing verdicts (to a Texture), Cut 2 2a (v).
	Support *types.CommitmentSupport `json:"support,omitempty"`
	// Captures is a capture arrival's source and new captures (to
	// management), Cut 2 2b.
	Captures *types.CommitmentCaptures `json:"captures,omitempty"`
	// Labels is Jev's label on each capture of a captures record, by
	// capture id, when intake labels are on (L2a A2 / R2-6). It is joined
	// at read time, never stored on the record.
	Labels map[string]CaptureLabelView `json:"labels,omitempty"`
	// TaskOutcome is a management task's final Report (management
	// delegation S2a): what each leased capture became and the residuals,
	// the captures it left, which management may delegate again by this
	// record's id.
	TaskOutcome *types.CommitmentTaskOutcome `json:"task_outcome,omitempty"`
	// Job is a capsule job's end (jobs.go): its state, exit code, receipt
	// and output tail, from job:<id>.
	Job *types.CommitmentJob `json:"job,omitempty"`
	// ParentID, RelatedIDs and EvidenceRefs are the record's links (C1e
	// D9): a Cast's parent request, the records a Reply answers, and the
	// evidence a Report names (an engineering Report names its assignment,
	// so management's Verify can name it).
	ParentID     string   `json:"parent_id,omitempty"`
	RelatedIDs   []string `json:"related_ids,omitempty"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
	// ParentConversationID and ParentRound name, on an engineering
	// Report, the Texture request its assignment serves (from the
	// assignment's binding), so management can answer that Texture.
	ParentConversationID string    `json:"parent_conversation_id,omitempty"`
	ParentRound          int       `json:"parent_round,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	// Request is, on management's Cast that opened an engineering
	// assignment, the Texture request the assignment serves: the parent
	// Texture Cast's sender, objective and body, verbatim (bounded), read from
	// the tape and checked against the assignment binding's RequestDigest.
	// RequestError says why it is absent when the check refused it.
	Request      *InboxRequest `json:"request,omitempty"`
	RequestError string        `json:"request_error,omitempty"`
}

// InboxRequest is the Texture request an engineering assignment serves, as
// engineering reads it. Objective and Body are the Texture Cast's own text
// (cut at a size bound, with Truncated and a marker line when cut); Digest
// is over the full text and is what the assignment binding commits to.
type InboxRequest struct {
	RecordID  string `json:"record_id"`
	From      string `json:"from"`
	Objective string `json:"objective,omitempty"`
	Body      string `json:"body,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
	Digest    string `json:"digest"`
}

// CaptureLabelView is Jev's label on one capture as management sees it:
// advice, not a decision. Version names the label version shown, so a
// later relabel or override never rewrites what management read.
type CaptureLabelView struct {
	Version int `json:"version,omitempty"`
	// Says is the label in words: "Jev: same event as texture:<doc>
	// '<headline>' (p 0.97)", "Jev: new; same topic as texture:<doc>
	// '<headline>'", or "unlabelled".
	Says string `json:"says"`
	// Event is story, sibling or new; Story is the story's inbox
	// (texture:<doc>) and SameEventAs the sibling capture's id.
	Event         string  `json:"event,omitempty"`
	Story         string  `json:"story,omitempty"`
	Headline      string  `json:"headline,omitempty"`
	SameEventAs   string  `json:"same_event_as,omitempty"`
	P             float64 `json:"p,omitempty"`
	TopicStory    string  `json:"topic_story,omitempty"`
	TopicHeadline string  `json:"topic_headline,omitempty"`
}

// String prints a record as its JSON, so a cell that prints records sees
// their bodies (cast, report, support, captures) rather than pointer
// addresses (B13).
func (r InboxRecord) String() string {
	b, err := json.Marshal(r)
	if err != nil {
		return fmt.Sprintf("{record_id:%s kind:%s from:%s}", r.RecordID, r.Kind, r.From)
	}
	return string(b)
}

// PendingUpdate is one pending update_coagent record bound into the cell at
// cell-start, readable via choir.Updates(). The RLM contract puts the update
// payload in a REPL variable instead of inline in the model's context window;
// the wake turn carries only update IDs. A terminal write disposes each update.
type PendingUpdate struct {
	UpdateID string `json:"update_id"`
	// ID is UpdateID under the name models reach for.
	ID              string             `json:"-"`
	FromAgentID     string             `json:"from_agent_id"`
	FromRole        string             `json:"from_role,omitempty"`
	ChannelID       string             `json:"channel_id,omitempty"`
	MessageSeq      int64              `json:"message_seq,omitempty"`
	WorkItemID      string             `json:"work_item_id,omitempty"`
	Packet          types.SourcePacket `json:"packet"`
	HumanProjection string             `json:"human_projection,omitempty"`
}

// PendingEmit is one emitted signal body bound into a desk cell as a REPL
// variable. Emits travel on the sender's channel with to_agent_id set, so a
// recipient desk drains addressed emissions through this bound snapshot —
// the chat boundary turn carries only the fixed-format notice (sender/kind/
// seq/snippet), never the body. Side-effect-free inside the cell; the model
// reads the untrusted body here, keeping prompt-injection bounded to data.
type PendingEmit struct {
	ChannelID   string `json:"channel_id,omitempty"`
	MessageSeq  int64  `json:"message_seq"`
	FromAgentID string `json:"from_agent_id"`
	FromRole    string `json:"from_role,omitempty"`
	Kind        string `json:"kind"`
	Body        string `json:"body"`
}

// StagedIntent is one non-blocking in-cell request awaiting post-cell
// reduction. LocalID correlates the intent within its cell only.
type StagedIntent struct {
	LocalID       string   `json:"local_id"`
	Kind          string   `json:"kind"`
	ToDesk        string   `json:"to_desk,omitempty"`
	MsgKind       string   `json:"msg_kind,omitempty"`
	Body          string   `json:"body,omitempty"`
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
	// Semantic-act fields (mission R2 commitment-ledger intents). TargetRef
	// binds a Resolve/Reply/Cancel to the act it closes. Claim is a Report's
	// typed claim text; ResolverID names the desk that accepts a Report.
	// Precommit, Resolve, and Disagreement are JSON-encoded typed commitment
	// bodies, preserving objectgraph's additive unknown-field tolerance.
	TargetRef    string `json:"target_ref,omitempty"`
	Claim        string `json:"claim,omitempty"`
	ResolverID   string `json:"resolver_id,omitempty"`
	Question     string `json:"question,omitempty"`
	Answer       string `json:"answer,omitempty"`
	Statement    string `json:"statement,omitempty"`
	Precommit    string `json:"precommit,omitempty"`
	Resolve      string `json:"resolve,omitempty"`
	Disagreement string `json:"disagreement,omitempty"`
	// Packet carries a Report's full coagent source-packet body (the
	// update_coagent packet schema surviving on the carrier) — JSON-encoded
	// types.SourcePacket. Set by ReportPacket; empty for a thin
	// claim-only Report.
	Packet string `json:"packet,omitempty"`
	// Disposition fields (management's choir.Feed and choir.Decline, L2a
	// R2-5): a Note that disposes of captures carries which (feed or
	// decline), the capture ids and management's reason.
	Disposition string   `json:"disposition,omitempty"`
	Captures    []string `json:"captures,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	// Job fields (IntentJob, jobs.go): the operation, the job id the cell
	// minted, and a start's argv, cwd, wall clock and declared memory need.
	JobOp       string   `json:"job_op,omitempty"`
	JobID       string   `json:"job_id,omitempty"`
	Argv        []string `json:"argv,omitempty"`
	Cwd         string   `json:"cwd,omitempty"`
	WallClockMS int64    `json:"wall_clock_ms,omitempty"`
	MemoryMiB   int      `json:"memory_mib,omitempty"`
}

// Tray stages one cell's outbound intents. It is not safe for concurrent use:
// cells execute serially on one interpreter. Methods return immediately and
// never touch the network.
type Tray struct {
	intents []StagedIntent
	bytes   int
	next    int
	// writeRefused holds the refusal of this cell's Write, if one was
	// refused; Done refuses after it (choir.go).
	writeRefused string
}

// Message stages an asynchronous outbound message to a peer desk (mesh) or
// parent (fan-in). Non-blocking; returns a cell-local correlation ID.
func (t *Tray) Message(toDesk, body string) (string, error) {
	if toDesk == "" {
		return "", fmt.Errorf("tray: message requires a destination desk")
	}
	return t.stage(StagedIntent{Kind: IntentMessage, ToDesk: toDesk, Body: body})
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
	if t.hasJobIntent() {
		return fmt.Errorf("tray: complete cannot share a cell with a job start or stop")
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
	if t.hasJobIntent() {
		return fmt.Errorf("tray: freeze cannot share a cell with a job start or stop")
	}
	_, err := t.stage(StagedIntent{Kind: IntentFreeze, BuildRecipeRef: buildRecipeRef, TestReceipts: testReceipts, DependencyToolchainRefs: dependencyToolchainRefs})
	return err
}

// Verify stages a verdict on an assignment's frozen candidate. At most one
// verify per cell; the reducer records it on cell return. assignmentID
// rides TargetRef; decision is pass or fail; verifierRefs are the evidence.
// bundleDigest is the exact digest the verdict is about; the reducer asserts
// it equals the operation's durable bundle digest, so a decision can never
// land on a bundle other than the one named.
func (t *Tray) Verify(assignmentID, decision string, verifierRefs []string, bundleDigest string) error {
	assignmentID = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(assignmentID), "assignment:"))
	if assignmentID == "" {
		return fmt.Errorf("tray: verify names the assignment whose candidate it judges")
	}
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
	_, err := t.stage(StagedIntent{Kind: IntentVerify, TargetRef: assignmentID, Decision: decision, VerifierRefs: verifierRefs, BundleDigest: bundleDigest})
	return err
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

// Disposition values of a disposing Note (L2a R2-5).
const (
	DispositionFeed    = types.DispositionFeed
	DispositionDecline = types.DispositionDecline
)

// Dispose stages a Note that disposes of captures: fed into the story at
// toDesk (texture:<doc>) or declined to their source (source:<id>). Its
// body names the reason and the captures, as a hand-written Note would.
func (t *Tray) Dispose(toDesk, disposition string, captureIDs []string, reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if toDesk == "" || len(captureIDs) == 0 || reason == "" {
		return "", fmt.Errorf("tray: %s requires a destination, capture ids and a reason", disposition)
	}
	verb := "Fed"
	if disposition == DispositionDecline {
		verb = "Declined"
	}
	body := reason + "\n\n" + verb + " captures: " + strings.Join(captureIDs, ", ")
	return t.stage(StagedIntent{Kind: IntentNote, ToDesk: toDesk, Body: body, Disposition: disposition, Captures: append([]string(nil), captureIDs...), Reason: reason})
}

// Delegate stages a batch delegation (management delegation S2a): one
// management task per group of spec.Groups.
func (t *Tray) Delegate(spec DelegateSpec) (string, error) {
	if len(spec.Groups) == 0 {
		return "", fmt.Errorf("tray: delegate requires at least one group of records")
	}
	raw, err := json.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("tray: encode delegate: %w", err)
	}
	return t.stage(StagedIntent{Kind: IntentDelegate, Objective: spec.Objective, Body: string(raw)})
}

// CancelTask stages the cancellation of one management task.
func (t *Tray) CancelTask(taskID, reason string) (string, error) {
	if strings.TrimSpace(taskID) == "" {
		return "", fmt.Errorf("tray: cancel_task requires the task id")
	}
	return t.stage(StagedIntent{Kind: IntentCancelTask, TargetRef: strings.TrimSpace(taskID), Body: strings.TrimSpace(reason)})
}

// TaskDone stages the end of a management task, once per cell.
func (t *Tray) TaskDone(note string) (string, error) {
	for _, in := range t.intents {
		if in.Kind == IntentTaskDone {
			return "", fmt.Errorf("tray: Done once per cell")
		}
	}
	return t.stage(StagedIntent{Kind: IntentTaskDone, Body: strings.TrimSpace(note)})
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

// CancelAssignment revokes an engineering assignment capsule with executor
// acknowledgement (SMG — the verb carrier for the deleted
// cancel_co_super_assignment tool). Management desk only.
func (t *Tray) CancelAssignment(assignmentID, reason string) (string, error) {
	if strings.TrimSpace(assignmentID) == "" {
		return "", fmt.Errorf("tray: cancel_assignment requires the assignment id")
	}
	return t.stage(StagedIntent{Kind: IntentCancelAssignment, TargetRef: strings.TrimSpace(assignmentID), Body: reason})
}

// Escalate surfaces an issue to management or the owner.
func (t *Tray) Escalate(toDesk, issue string) (string, error) {
	if toDesk == "" || issue == "" {
		return "", fmt.Errorf("tray: escalate requires a target and an issue")
	}
	return t.stage(StagedIntent{Kind: IntentEscalate, ToDesk: toDesk, Body: issue})
}

// Precommit freezes a typed prediction on the commitment ledger. precommitJSON
// encodes types.CommitmentPrecommit; validation occurs before reduction so an
// invalid body never reaches durable state.
func (t *Tray) Precommit(precommitJSON string) (string, error) {
	if strings.TrimSpace(precommitJSON) == "" {
		return "", fmt.Errorf("tray: precommit requires a typed precommit body")
	}
	return t.stage(StagedIntent{Kind: IntentPrecommit, Precommit: precommitJSON})
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
// (mission R2). packetJSON is a JSON-encoded types.SourcePacket;
// the reducer validates it (schema_version, kind, claims/sources/actions/
// questions) before the act commits, preserving the packet's safety contract.
func (t *Tray) ReportPacket(toDesk, packetJSON, resolverID string) (string, error) {
	if toDesk == "" || strings.TrimSpace(packetJSON) == "" {
		return "", fmt.Errorf("tray: report_packet requires a desk and a packet body")
	}
	return t.stage(StagedIntent{Kind: IntentReport, ToDesk: toDesk, Packet: packetJSON, ResolverID: resolverID})
}

// Resolve closes a commitment with a typed evidence-bearing resolver verdict.
func (t *Tray) Resolve(targetRef, resolveJSON string) (string, error) {
	if targetRef == "" || strings.TrimSpace(resolveJSON) == "" {
		return "", fmt.Errorf("tray: resolve requires the act ref and typed resolve body")
	}
	return t.stage(StagedIntent{Kind: IntentResolve, TargetRef: targetRef, Resolve: resolveJSON})
}

// Disagreement records scorer and resolver verdicts separately from Resolve.
func (t *Tray) Disagreement(disagreementJSON string) (string, error) {
	if strings.TrimSpace(disagreementJSON) == "" {
		return "", fmt.Errorf("tray: disagreement requires a typed disagreement body")
	}
	return t.stage(StagedIntent{Kind: IntentDisagreement, Disagreement: disagreementJSON})
}

// TextureTurn builds the cell's one Texture turn: the first call stages it,
// later calls revise it in place. update refuses a combination the turn
// cannot hold, leaving the turn as it was. Doc bodies may exceed the message
// cap, so this kind is bound by the aggregate tray quota, not MaxIntentBody.
func (t *Tray) TextureTurn(baseRevisionID string, update func(*TextureTurn) error) (string, error) {
	at := -1
	for i, in := range t.intents {
		if in.Kind == IntentTextureTurn {
			at = i
			break
		}
	}
	turn := TextureTurn{BaseRevisionID: baseRevisionID}
	if at >= 0 {
		if err := json.Unmarshal([]byte(t.intents[at].Body), &turn); err != nil {
			return "", fmt.Errorf("tray: texture turn body: %w", err)
		}
	}
	if err := update(&turn); err != nil {
		return "", err
	}
	raw, err := json.Marshal(turn)
	if err != nil {
		return "", fmt.Errorf("tray: encode texture turn: %w", err)
	}
	if at < 0 {
		return t.stage(StagedIntent{Kind: IntentTextureTurn, Body: string(raw)})
	}
	grown := len(raw) - len(t.intents[at].Body)
	if t.bytes+grown > MaxTrayBytes {
		return "", fmt.Errorf("tray: aggregate payload exceeds %d bytes", MaxTrayBytes)
	}
	t.bytes += grown
	t.intents[at].Body = string(raw)
	return t.intents[at].LocalID, nil
}

func (t *Tray) stage(in StagedIntent) (string, error) {
	if len(t.intents) >= MaxIntentsPerCell {
		return "", fmt.Errorf("tray: cell intent quota exceeded (%d)", MaxIntentsPerCell)
	}
	size := len(in.Body) + len(in.Objective) + len(in.Summary) + len(in.Verdict) + len(in.BuildRecipeRef) + len(in.Decision) +
		len(in.TargetRef) + len(in.Claim) + len(in.ResolverID) + len(in.Question) + len(in.Answer) + len(in.Statement) +
		len(in.Precommit) + len(in.Resolve) + len(in.Disagreement)
	size += len(in.Reason) + len(in.JobOp) + len(in.JobID) + len(in.Cwd)
	for _, refs := range [][]string{in.EvidenceRefs, in.ExecutionRefs, in.TestReceipts, in.DependencyToolchainRefs, in.VerifierRefs, in.Captures, in.Argv} {
		for _, ref := range refs {
			size += len(ref)
		}
	}
	if in.Kind != IntentTextureTurn && len(in.Body) > MaxIntentBody {
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
	// Refusals returns the errors choir verbs returned during the cell, as
	// "choir.Verb refused: ..." lines.
	Refusals func() []string
	// SearchCounts returns one count line per async search with arrivals
	// the desk has not been told about ("s3: 3/4 answered, +2 new").
	SearchCounts func() []string
	// RecordsShown returns the highest record seq a choir.Records() call in
	// the cell returned (0: none).
	RecordsShown func() int64
	// JobsAwaited returns the jobs a choir.Poll in the cell returned
	// unfinished, in first-poll order (nil: none).
	JobsAwaited func() []string
	// KeepOutput takes a successful cell's printed output and returns what
	// the result shows: a print over the bound is stored whole for
	// choir.CellOut and shown cut (LR lever 6, cell_output.go). Nil means no
	// cut, since no store could serve a handle (the capsule path).
	KeepOutput func(stdout string) string
}
