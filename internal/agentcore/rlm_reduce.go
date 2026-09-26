package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
	"log"
	"strings"
	"time"
)

// commits a successful cell's staged tray into durable state and wakes
// recipients. Doctrine: the database remembers (Dolt channel log + run
// memory), Go delivers (channel event wakes consumed by recipient turns).
// Failed, timed-out, or poisoned cells reduce nothing and advance no cursor.

// rlmEnvelopeV1 marks reducer-written channel payloads. Assembly decodes the
// envelope; legacy non-enveloped traffic still reads as raw body.
const rlmEnvelopeV1 = "rlm/v1 "

// rlmInboxCursorKind is the run-memory kind carrying the durable unread
// cursor per activation channel. Latest entry wins; absence means zero.
const rlmInboxCursorKind = types.RunMemoryEntryKind("rlm_inbox_cursor")

// rlmMailbox is the durable surface reduction needs: the Dolt-backed channel
// log for envelopes. *Runtime implements it directly.
type rlmMailbox interface {
	ChannelCast(ctx context.Context, channelID, toAgentID, toRunID, from, role, content string) (uint64, error)
	CastEnvelope(ctx context.Context, channelID, toAgentID, from, role, content, idempotencyKey string) (uint64, error)
	ChannelRead(channelID string, cursor uint64) ([]ChannelMessage, uint64, error)
}

// rlmCursorStore persists the inbox cursor as run-memory entries. *store.Store
// implements it directly.
type rlmCursorStore interface {
	AppendRunMemoryEntry(ctx context.Context, entry types.RunMemoryEntry) (types.RunMemoryEntry, error)
	ListRunMemoryEntries(ctx context.Context, ownerID, runID string) ([]types.RunMemoryEntry, error)
}

// ReductionScope binds one reduction to its validated sender and mailbox.
type ReductionScope struct {
	FromAgentID string // validated sender desk identity
	FromRole    string // validated sender role (spawn policy)
	ChannelID   string // durable mailbox channel
	RunID       string // activation run carrying the inbox cursor
	OwnerID     string // store owner for run memory
	ComputerID  string // physical owner scope for ledger/OG objects
	ReturnTo    string // supervisor desk: spawn requests and completion reports
	Cursor      uint64 // durable unread cursor entering the cell
	CellID      string // stable cell identity for intent idempotency keys
}

// ReducedIntent pairs a cell-local ID with its durable sequence.
type ReducedIntent struct {
	LocalID string
	Seq     uint64
	Kind    string
}

// ReductionReceipt is the two-phase ack: Committed is true only when every
// intent persisted and the cursor advanced. Failed cells return Committed
// false with the entering cursor unchanged.
type ReductionReceipt struct {
	Intents   []ReducedIntent
	Cursor    uint64
	Committed bool
}

// spawnRoleAllowed enforces role-bounded fan-out: research desks cannot mint
// engineering authority. Management may spawn any role; Engineering may spawn
// researchers and peers; researchers may only fan out to researchers.
func spawnRoleAllowed(spawnerRole, childRole string) bool {
	spawner := strings.ToLower(strings.TrimSpace(spawnerRole))
	child := strings.ToLower(strings.TrimSpace(childRole))
	// Frozen V2 spawn matrix (mapping §2): management allows any child;
	// engineering allows engineering and research; research allows research;
	// default false. No V1 token is accepted in any position.
	switch spawner {
	case "management":
		return true
	case "engineering":
		return child == "engineering" || child == "research"
	case "research":
		return child == "research"
	default:
		return false
	}
}

// validateCellIntents re-checks worker-produced intents at the trust
// boundary. The worker is our binary but the model authors the cells, so the
// reducer enforces quotas, the single-complete rule, and spawn policy.
func validateCellIntents(scope ReductionScope, intents []yaegikernel.StagedIntent) error {
	if len(intents) > yaegikernel.MaxIntentsPerCell {
		return fmt.Errorf("reduce: %d intents exceed cell quota %d", len(intents), yaegikernel.MaxIntentsPerCell)
	}
	complete := 0
	for i, in := range intents {
		switch in.Kind {
		case yaegikernel.IntentMessage:
			if in.ToDesk == "" {
				return fmt.Errorf("reduce: message %s missing destination", in.LocalID)
			}
			if len(in.Body) > yaegikernel.MaxIntentBody {
				return fmt.Errorf("reduce: message %s exceeds body quota", in.LocalID)
			}
		case yaegikernel.IntentOutcome:
			if in.ToDesk == "" {
				return fmt.Errorf("reduce: outcome %s missing destination", in.LocalID)
			}
			if len(in.Body) > yaegikernel.MaxIntentBody {
				return fmt.Errorf("reduce: outcome %s exceeds body quota", in.LocalID)
			}
		case yaegikernel.IntentSpawn:
			if !spawnRoleAllowed(scope.FromRole, in.Role) {
				return fmt.Errorf("reduce: role %q may not spawn %q", scope.FromRole, in.Role)
			}
			if in.Objective == "" {
				return fmt.Errorf("reduce: spawn %s missing objective", in.LocalID)
			}
		case yaegikernel.IntentComplete:
			complete++
			// Complete commits terminal fate; any intent after it could fail
			// and leave the cell reporting failure after fate already landed.
			if i != len(intents)-1 {
				return fmt.Errorf("reduce: complete %s must be the final intent in its cell", in.LocalID)
			}
			switch in.Result {
			case yaegikernel.CompleteCompleted, yaegikernel.CompleteFailed, yaegikernel.CompleteBlocked, yaegikernel.CompletePartial:
			default:
				return fmt.Errorf("reduce: complete result %q invalid", in.Result)
			}
		case yaegikernel.IntentFreeze:
			if strings.TrimSpace(in.BuildRecipeRef) == "" || len(in.TestReceipts) == 0 || len(in.DependencyToolchainRefs) == 0 {
				return fmt.Errorf("reduce: freeze %s requires build recipe, test receipts, and dependency/toolchain refs", in.LocalID)
			}
		case yaegikernel.IntentVerify:
			if in.Decision != "pass" && in.Decision != "fail" {
				return fmt.Errorf("reduce: verify %s decision %q invalid", in.LocalID, in.Decision)
			}
			if len(in.VerifierRefs) == 0 {
				return fmt.Errorf("reduce: verify %s requires verifier refs", in.LocalID)
			}
			if strings.TrimSpace(in.BundleDigest) == "" {
				return fmt.Errorf("reduce: verify %s requires the inspected bundle digest", in.LocalID)
			}
		default:
			if err := validateSemanticActIntent(in); err != nil {
				return err
			}
		}
	}
	if complete > 1 {
		return fmt.Errorf("reduce: at most one complete per cell")
	}
	return nil
}

// validateSemanticActIntent accepts the mission-R2 commitment-ledger act
// kinds (cast/ask/note/reply/cancel/escalate/precommit/report/resolve) and
// rejects anything else. Each act needs the field it names; an unrecognized
// kind is still rejected.
func validateSemanticActIntent(in yaegikernel.StagedIntent) error {
	// needTo requires a non-empty destination desk that resolves to a known
	// canonical profile — the R3b reject: an unknown desk is not a cast
	// target, fail closed before the ledger mints anything.
	needTo := func() error {
		to := strings.TrimSpace(in.ToDesk)
		if to == "" {
			return fmt.Errorf("reduce: %s %s missing destination desk", in.Kind, in.LocalID)
		}
		if _, err := agentprofile.Canonical(to); err != nil {
			return fmt.Errorf("reduce: %s %s targets unknown desk %q", in.Kind, in.LocalID, in.ToDesk)
		}
		return nil
	}
	switch in.Kind {
	case yaegikernel.IntentCast:
		if err := needTo(); err != nil {
			return err
		}
		if in.Objective == "" {
			return fmt.Errorf("reduce: cast %s missing objective", in.LocalID)
		}
	case yaegikernel.IntentAsk:
		if err := needTo(); err != nil {
			return err
		}
		if in.Question == "" {
			return fmt.Errorf("reduce: ask %s missing question", in.LocalID)
		}
	case yaegikernel.IntentNote, yaegikernel.IntentEscalate:
		if err := needTo(); err != nil {
			return err
		}
		if in.Body == "" {
			return fmt.Errorf("reduce: %s %s missing body", in.Kind, in.LocalID)
		}
	case yaegikernel.IntentReply:
		if err := needTo(); err != nil {
			return err
		}
		if in.TargetRef == "" {
			return fmt.Errorf("reduce: reply %s missing the ask's target ref", in.LocalID)
		}
	case yaegikernel.IntentCancel:
		if in.TargetRef == "" {
			return fmt.Errorf("reduce: %s %s missing the act ref it closes", in.Kind, in.LocalID)
		}
	case yaegikernel.IntentResolve:
		if in.TargetRef == "" {
			return fmt.Errorf("reduce: %s %s missing the act ref it closes", in.Kind, in.LocalID)
		}
		if in.OutcomeVal == "" {
			return fmt.Errorf("reduce: resolve %s missing the outcome verdict", in.LocalID)
		}
	case yaegikernel.IntentPrecommit:
		if in.Statement == "" {
			return fmt.Errorf("reduce: precommit %s missing the frozen prediction statement", in.LocalID)
		}
	case yaegikernel.IntentReport:
		if err := needTo(); err != nil {
			return err
		}
		if in.Claim == "" {
			return fmt.Errorf("reduce: report %s missing the claim", in.LocalID)
		}
	default:
		return fmt.Errorf("reduce: unknown intent kind %q", in.Kind)
	}
	return nil
}

type rlmEnvelope struct {
	Kind         string   `json:"kind"`
	Body         string   `json:"body,omitempty"`
	MsgKind      string   `json:"msg_kind,omitempty"`
	Role         string   `json:"role,omitempty"`
	Objective    string   `json:"objective,omitempty"`
	Result       string   `json:"result,omitempty"`
	Verdict      string   `json:"verdict,omitempty"`
	Summary      string   `json:"summary,omitempty"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
	// Packet carries a report's coagent source-packet body on the wire so the
	// receiving desk sees the full typed packet, not just the claim line.
	Packet string `json:"packet,omitempty"`
	From   string `json:"from"`
}

func encodeEnvelope(env rlmEnvelope) string {
	raw, err := json.Marshal(env)
	if err != nil {
		return rlmEnvelopeV1 + `{"kind":"encode_error"}`
	}
	return rlmEnvelopeV1 + string(raw)
}

func intentIdempotencyKey(scope ReductionScope, localID, to, content string) string {
	cell := strings.TrimSpace(scope.CellID)
	if cell == "" {
		cell = fmt.Sprintf("%s:%d", scope.RunID, scope.Cursor)
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(to) + "\x1f" + content))
	return "rlm:" + cell + ":" + strings.TrimSpace(localID) + ":" + hex.EncodeToString(sum[:6])
}

// ReduceCellIntents commits one cell's staged tray. cellSucceeded false (or a
// poisoned-cell result) persists nothing and returns the entering cursor: the
// durable cursor advances only upon successful reduction, so failed cells
// never acknowledge unread mail.
func ReduceCellIntents(ctx context.Context, mb rlmMailbox, scope ReductionScope, intents []yaegikernel.StagedIntent, cellSucceeded bool) (ReductionReceipt, error) {
	if !cellSucceeded {
		return ReductionReceipt{Cursor: scope.Cursor}, nil
	}
	if err := validateCellIntents(scope, intents); err != nil {
		return ReductionReceipt{Cursor: scope.Cursor}, err
	}
	receipt := ReductionReceipt{Cursor: scope.Cursor}
	for _, in := range intents {
		seq, err := castStagedIntent(ctx, mb, scope, in)
		if err != nil {
			return ReductionReceipt{Cursor: scope.Cursor}, fmt.Errorf("reduce: persist %s: %w", in.LocalID, err)
		}
		receipt.Intents = append(receipt.Intents, ReducedIntent{LocalID: in.LocalID, Seq: seq, Kind: in.Kind})
	}
	receipt.Committed = true
	return receipt, nil
}

// stagedIntentEnvelope builds the durable envelope for an intent. Keeping the
// exact envelope derivation available to recovery makes the mailbox
// idempotency key a stable witness of whether an interrupted act was mailed.
func stagedIntentEnvelope(scope ReductionScope, in yaegikernel.StagedIntent) (to, content string, mailed bool, err error) {
	switch in.Kind {
	case yaegikernel.IntentMessage:
		to = in.ToDesk
		content = encodeEnvelope(rlmEnvelope{Kind: "message", MsgKind: in.MsgKind, Body: in.Body, From: scope.FromAgentID})
	case yaegikernel.IntentOutcome:
		to = in.ToDesk
		content = encodeEnvelope(rlmEnvelope{Kind: "message", MsgKind: "outcome", Body: in.Body, From: scope.FromAgentID})
	case yaegikernel.IntentSpawn:
		to = scope.ReturnTo
		content = encodeEnvelope(rlmEnvelope{Kind: "spawn_request", Role: in.Role, Objective: in.Objective, From: scope.FromAgentID})
	case yaegikernel.IntentComplete:
		to = scope.ReturnTo
		content = encodeEnvelope(rlmEnvelope{Kind: "complete", Result: in.Result, Verdict: in.Verdict, Summary: in.Summary, EvidenceRefs: in.EvidenceRefs, From: scope.FromAgentID})
	// Semantic-act verbs (mission R2). Addressed acts mail an envelope that
	// carries the act kind so the receiving desk sees the typed act, not a
	// bare message. Ledger-bound acts (precommit/report/resolve) additionally
	// write the commitment record on the commit path (commitActIntent).
	case yaegikernel.IntentCast:
		to = in.ToDesk
		content = encodeEnvelope(rlmEnvelope{Kind: "cast", Objective: in.Objective, Body: in.Statement, From: scope.FromAgentID})
	case yaegikernel.IntentAsk:
		to = in.ToDesk
		content = encodeEnvelope(rlmEnvelope{Kind: "ask", Body: in.Question, From: scope.FromAgentID})
	case yaegikernel.IntentNote:
		to = in.ToDesk
		content = encodeEnvelope(rlmEnvelope{Kind: "note", Body: in.Body, From: scope.FromAgentID})
	case yaegikernel.IntentReply:
		to = in.ToDesk
		content = encodeEnvelope(rlmEnvelope{Kind: "reply", Body: in.Answer, From: scope.FromAgentID})
	case yaegikernel.IntentEscalate:
		to = in.ToDesk
		content = encodeEnvelope(rlmEnvelope{Kind: "escalate", Body: in.Body, From: scope.FromAgentID})
	case yaegikernel.IntentReport:
		to = in.ToDesk
		content = encodeEnvelope(rlmEnvelope{Kind: "report", Body: in.Claim, EvidenceRefs: in.EvidenceRefs, Packet: in.Packet, From: scope.FromAgentID})
	case yaegikernel.IntentResolve, yaegikernel.IntentCancel, yaegikernel.IntentPrecommit:
		// Resolved purely on the ledger path: the act lands on the target
		// commitment record, not a desk mailbox. Handled by commitActIntent.
		return "", "", false, nil
	default:
		return "", "", false, fmt.Errorf("intent kind %q has no envelope path", in.Kind)
	}
	return to, content, true, nil
}

// castStagedIntent mails one envelope-eligible intent to its durable channel
// target. Freeze and verify intents never mail: their effects land through
// the reduction's own commit path, not the channel log.
func castStagedIntent(ctx context.Context, mb rlmMailbox, scope ReductionScope, in yaegikernel.StagedIntent) (uint64, error) {
	to, content, mailed, err := stagedIntentEnvelope(scope, in)
	if err != nil || !mailed {
		return 0, err
	}
	return mb.CastEnvelope(ctx, scope.ChannelID, to, scope.FromAgentID, scope.FromRole, content, intentIdempotencyKey(scope, in.LocalID, to, content))
}

// AssembleCellInbox reads the durable mailbox since the cursor and maps it to
// a cell-start snapshot. It advances nothing: the returned high-water becomes
// durable only when the cell's reduction commits it. Envelope payloads decode
// to kind/body; legacy traffic reads as raw body.
func AssembleCellInbox(ctx context.Context, mb rlmMailbox, channelID string, cursor uint64) ([]yaegikernel.IncomingMessage, uint64, error) {
	messages, highWater, err := mb.ChannelRead(channelID, cursor)
	if err != nil {
		return nil, cursor, err
	}
	var inbox []yaegikernel.IncomingMessage
	for _, m := range messages {
		kind, body := "channel", m.Content
		if rest, ok := strings.CutPrefix(m.Content, rlmEnvelopeV1); ok {
			var env rlmEnvelope
			if err := json.Unmarshal([]byte(rest), &env); err == nil {
				kind = env.Kind
				if kind == "message" && env.MsgKind != "" {
					kind = env.MsgKind
				}
				body = env.Body
				if kind == "complete" {
					body = env.Summary
				}
				if kind == "spawn_request" {
					body = env.Objective
				}
			}
		}
		inbox = append(inbox, yaegikernel.IncomingMessage{
			ID:       fmt.Sprintf("chan-%d", m.Seq),
			FromDesk: m.FromAgentID,
			ToDesk:   m.ToAgentID,
			Kind:     kind,
			Body:     body,
		})
	}
	if inbox == nil {
		inbox = []yaegikernel.IncomingMessage{}
	}
	return inbox, highWater, nil
}

// LoadInboxCursor returns the durable unread cursor for the run's channel.
// Absence means zero: a fresh activation reads from the log start.
func LoadInboxCursor(ctx context.Context, st rlmCursorStore, ownerID, runID, channelID string) (uint64, error) {
	entries, err := st.ListRunMemoryEntries(ctx, ownerID, runID)
	if err != nil {
		return 0, err
	}
	var cursor uint64
	for _, e := range entries {
		if e.Kind != rlmInboxCursorKind {
			continue
		}
		channel, _ := e.Details["channel_id"].(string)
		if channel != channelID {
			continue
		}
		switch v := e.Details["cursor"].(type) {
		case float64:
			if uint64(v) > cursor {
				cursor = uint64(v)
			}
		case int64:
			if v > 0 && uint64(v) > cursor {
				cursor = uint64(v)
			}
		}
	}
	return cursor, nil
}

// CommitInboxCursor advances the durable unread cursor. Call only with a
// committed reduction receipt: failed cells never reach this path, so unread
func CommitInboxCursor(ctx context.Context, st rlmCursorStore, ownerID, runID, channelID string, cursor uint64) error {
	_, err := st.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{
		RunID:   runID,
		OwnerID: ownerID,
		Kind:    rlmInboxCursorKind,
		Summary: fmt.Sprintf("rlm inbox cursor %d on %s", cursor, channelID),
		Details: map[string]any{"channel_id": channelID, "cursor": cursor},
	})
	return err
}

// path stays byte-identical then, and reduction is a no-op. rec and toolCtx
// are retained so a staged Complete intent can author the assignment fate
// (P3-settlement: the reducer is the single fate author).
type rlmCallReduction struct {
	active bool
	mb     rlmMailbox
	st     rlmCursorStore
	// ledger is the concrete store for commitment-record writes (mission R2);
	// it is *store.Store (rlmCursorStore is the narrow cursor-only view).
	ledger    *store.Store
	scope     ReductionScope
	rec       *types.RunRecord
	toolCtx   *CapsuleToolCtx
	inbox     []yaegikernel.IncomingMessage
	highWater uint64
	receipt   ReductionReceipt
	// fateTerminal is set when a Complete intent committed a terminal
	// assignment fate; the eval tool surfaces it so the run loop can end.
	fateTerminal bool
	// freezeResult/verifyResult carry the staged freeze/verify outcomes back
	// to the cell result so the model sees the same receipt the retired JSON
	// tools returned.
	freezeResult map[string]any
	verifyResult map[string]any
}

// rlmReductionForCall assembles the cell-start inbox snapshot from the durable
// mailbox cursor. It never advances the cursor: commitment happens in commit,
// only for successful cells.
func rlmReductionForCall(ctx context.Context, rt *Runtime, toolCtx *CapsuleToolCtx) *rlmCallReduction {
	inert := &rlmCallReduction{}
	if rt == nil || toolCtx == nil || !capsule.HostSelectsRLM() {
		return inert
	}
	execCtx := toolregistry.ExecutionContextFrom(ctx)
	channel := channelIDForRun(execCtx.RunRecord)
	if channel == "" {
		channel = execCtx.ChannelID
	}
	if channel == "" || execCtx.RunID == "" {
		return inert
	}
	cursor, err := LoadInboxCursor(ctx, rt.store, execCtx.OwnerID, execCtx.RunID, channel)
	if err != nil {
		return inert
	}
	inbox, highWater, err := AssembleCellInbox(ctx, rt, channel, cursor)
	if err != nil {
		return inert
	}
	requester := ""
	if execCtx.RunRecord != nil {
		requester = metadataStringValue(execCtx.RunRecord.Metadata, "requested_by_agent_id")
	}
	return &rlmCallReduction{
		active:  true,
		mb:      rt,
		st:      rt.store,
		rec:     execCtx.RunRecord,
		toolCtx: toolCtx,
		scope: ReductionScope{
			FromAgentID: execCtx.AgentID,
			FromRole:    string(toolCtx.Role),
			ChannelID:   channel,
			RunID:       execCtx.RunID,
			OwnerID:     execCtx.OwnerID,
			ComputerID:  execCtx.ComputerID,
			ReturnTo:    requester,
			Cursor:      cursor,
			CellID:      fmt.Sprintf("%s:%d", execCtx.RunID, cursor),
		},
		ledger:    rt.store,
		inbox:     inbox,
		highWater: highWater,
	}
}

// rlmReductionForDeskCall is the host desk-cell analogue of
// rlmReductionForCall: a non-capsule desk (management/texture/research) runs
// model-authored cells in a host session worker via desk_go_eval; there is no
// CapsuleToolCtx — FromRole comes from the run's profile, toolCtx is nil
// (capsule-only fate paths are unreachable for desk intents). Same channel
// mailbox, same cursor, same commitTray path — the ledger sees no difference.
func rlmReductionForDeskCall(ctx context.Context, rt *Runtime) *rlmCallReduction {
	inert := &rlmCallReduction{}
	if rt == nil || !capsule.HostSelectsRLM() {
		return inert
	}
	execCtx := toolregistry.ExecutionContextFrom(ctx)
	channel := channelIDForRun(execCtx.RunRecord)
	if channel == "" {
		channel = execCtx.ChannelID
	}
	if channel == "" || execCtx.RunID == "" {
		return inert
	}
	cursor, err := LoadInboxCursor(ctx, rt.store, execCtx.OwnerID, execCtx.RunID, channel)
	if err != nil {
		return inert
	}
	inbox, highWater, err := AssembleCellInbox(ctx, rt, channel, cursor)
	if err != nil {
		return inert
	}
	requester := ""
	if execCtx.RunRecord != nil {
		requester = metadataStringValue(execCtx.RunRecord.Metadata, "requested_by_agent_id")
	}
	role := strings.TrimSpace(execCtx.Role)
	if role == "" && execCtx.RunRecord != nil {
		role = agentProfileForRun(execCtx.RunRecord)
	}
	return &rlmCallReduction{
		active:  true,
		mb:      rt,
		st:      rt.store,
		rec:     execCtx.RunRecord,
		toolCtx: nil, // non-capsule desk — no capsule tool context
		scope: ReductionScope{
			FromAgentID: execCtx.AgentID,
			FromRole:    role,
			ChannelID:   channel,
			RunID:       execCtx.RunID,
			OwnerID:     execCtx.OwnerID,
			ComputerID:  execCtx.ComputerID,
			ReturnTo:    requester,
			Cursor:      cursor,
			CellID:      fmt.Sprintf("%s:%d", execCtx.RunID, cursor),
		},
		ledger:    rt.store,
		inbox:     inbox,
		highWater: highWater,
	}
}

// commit reduces a successful cell's staged tray and advances the durable
// inbox cursor to the consumed snapshot high-water only. Outbound intent
// sequences live on the same channel log and must not fence unread inbound
// mail that arrived after the snapshot. Failed cells never reach this path.
// A staged Complete intent authors the assignment fate through the same saga
// the retired JSON tool used; the fate commit runs detached so teardown
// cannot interrupt it.
func (r *rlmCallReduction) commit(ctx context.Context, intents []yaegikernel.StagedIntent) error {
	if r == nil || !r.active {
		return nil
	}
	if err := validateCellIntents(r.scope, intents); err != nil {
		return err
	}
	highWater := r.highWater
	if highWater < r.scope.Cursor {
		highWater = r.scope.Cursor
	}
	if recovered, err := r.recoverPartialActCommit(ctx, intents, highWater); err != nil {
		return err
	} else if recovered {
		return nil
	}
	return r.commitTray(ctx, intents, highWater)
}

// recoverPartialActCommit is the named crash-recovery path for the reducer's
// cross-store act commit. The object graph, channel log, and run-memory cursor
// do not expose one shared transaction. A pre-existing deterministic
// commitment record therefore witnesses a possible crash after ledger mint.
// If the inbox acknowledgement is behind its snapshot or the addressed
// envelope is absent, replay the complete tray. Both ledger minting and
// envelope mail use deterministic idempotency identities, so this converges
// without double-minting a record or duplicating mail.
func (r *rlmCallReduction) recoverPartialActCommit(ctx context.Context, intents []yaegikernel.StagedIntent, highWater uint64) (bool, error) {
	if r.ledger == nil {
		return false, nil
	}
	cursor, err := LoadInboxCursor(ctx, r.st, r.scope.OwnerID, r.scope.RunID, r.scope.ChannelID)
	if err != nil {
		return false, fmt.Errorf("recover partial act commit: load inbox cursor: %w", err)
	}
	for _, in := range intents {
		if !isSemanticActKind(in.Kind) {
			continue
		}
		exists, err := r.ledger.CommitmentRecordExists(ctx, r.scope.OwnerID, r.scope.ComputerID, commitmentRecordForIntent(r.scope, in).RecordID)
		if err != nil {
			return false, fmt.Errorf("recover partial act commit: inspect %s: %w", in.LocalID, err)
		}
		if !exists {
			continue
		}
		if cursor < highWater {
			return true, r.commitTray(ctx, intents, highWater)
		}
		to, content, mailed, err := stagedIntentEnvelope(r.scope, in)
		if err != nil {
			return false, err
		}
		if !mailed {
			continue
		}
		messages, _, err := r.mb.ChannelRead(r.scope.ChannelID, 0)
		if err != nil {
			return false, fmt.Errorf("recover partial act commit: inspect envelope %s: %w", in.LocalID, err)
		}
		key := intentIdempotencyKey(r.scope, in.LocalID, to, content)
		for _, message := range messages {
			if message.IdempotencyKey == key {
				goto nextIntent
			}
		}
		return true, r.commitTray(ctx, intents, highWater)
	nextIntent:
	}
	return false, nil
}

func (r *rlmCallReduction) commitTray(ctx context.Context, intents []yaegikernel.StagedIntent, highWater uint64) error {
	receipt := ReductionReceipt{Cursor: r.scope.Cursor}
	for _, in := range intents {
		var seq uint64
		var err error
		switch in.Kind {
		case yaegikernel.IntentComplete:
			// The fate commit runs detached so teardown cannot interrupt it;
			// the completion envelope still mails to the requester after the
			// fate lands. The timeout covers a slow frozen-capsule revocation
			// (cgroup teardown plus typed receipt persistence on an
			// fsync-bound store); 30s stranded assignment-9ec36ecb
			// mid-revocation, so the window is minutes, not seconds.
			fateCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Minute)
			var terminal bool
			terminal, err = r.commitCompleteIntent(fateCtx, in)
			cancel()
			if err == nil {
				if terminal {
					r.fateTerminal = true
				}
				seq, err = castStagedIntent(ctx, r.mb, r.scope, in)
			}
		case yaegikernel.IntentFreeze:
			// Like Complete, freezing must outlive cell teardown: the cgroup
			// transition cannot be cancelled by the caller once reduction
			// starts.
			fateCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Minute)
			var out map[string]any
			out, err = r.commitFreezeIntent(fateCtx, in)
			cancel()
			if err == nil {
				r.freezeResult = out
			}
		case yaegikernel.IntentVerify:
			var out map[string]any
			out, err = r.commitVerifyIntent(ctx, in)
			if err == nil {
				r.verifyResult = out
			}
		case yaegikernel.IntentMessage:
			// Every staged message on an assigned desk carries the
			// update_coagent authority contract. The outcome envelope path is
			// reachable only through IntentOutcome, which ChoirScope.Outcome
			// stages — a model-authored MsgKind cannot claim it.
			if r.isAssignedDesk() {
				seq, err = r.commitMessageIntent(ctx, in)
			} else {
				seq, err = castStagedIntent(ctx, r.mb, r.scope, in)
			}
		case yaegikernel.IntentOutcome:
			seq, err = castStagedIntent(ctx, r.mb, r.scope, in)
		default:
			// Semantic acts (mission R2): ledger-bound kinds append the
			// commitment record; every act may also mail its envelope.
			seq, err = r.commitActIntent(ctx, in)
		}
		if err != nil {
			return fmt.Errorf("reduce: persist %s: %w", in.LocalID, err)
		}
		receipt.Intents = append(receipt.Intents, ReducedIntent{LocalID: in.LocalID, Seq: seq, Kind: in.Kind})
	}
	if err := CommitInboxCursor(ctx, r.st, r.scope.OwnerID, r.scope.RunID, r.scope.ChannelID, highWater); err != nil {
		return err
	}
	r.receipt = receipt
	r.receipt.Cursor = highWater
	r.receipt.Committed = true
	return nil
}

// commitActIntent commits one semantic act (mission R2). Epistemic acts that
// open or resolve a commitment append a choir.commitment_record object on the
// object graph (append-only; resolution is a linked record, never a rewrite,
// so the ledger stays an auditable event log). Addressed acts additionally
// mail their typed envelope to the target desk so it sees the act.
// Returns the channel seq of any mailed envelope (0 for ledger-only acts).
func (r *rlmCallReduction) commitActIntent(ctx context.Context, in yaegikernel.StagedIntent) (uint64, error) {
	if !isSemanticActKind(in.Kind) {
		return castStagedIntent(ctx, r.mb, r.scope, in)
	}
	// Mint the commitment record. The record id binds the act to its cell for
	// replay idempotency: a re-reduced cell re-derives the same id and the
	// not-exists condition re-mints nothing.
	rec := commitmentRecordForIntent(r.scope, in)
	var controlID string
	var err error
	if r.ledger != nil {
		controlID, err = r.ledger.AppendCommitmentRecord(ctx, r.scope.OwnerID, r.scope.ComputerID, rec)
		if err != nil {
			return 0, err
		}
	}
	// A cast is admission, not just a message: durably OPEN the engineering
	// assignment under the delegated-cast authority (the caster's own live
	// run/work), with the commitment record minted above as the parent control.
	// The spawn/bind/activate saga is deliberately NOT run inside the cell
	// reducer (consensus precondition) — it resumes from the deferred
	// delegated_assignment_spawn_deadline wake, or synchronously when kernel
	// mode is off (test runtimes without the durable wake outbox).
	if in.Kind == yaegikernel.IntentCast {
		if rt := r.rt(); rt != nil && r.rec != nil && controlID != "" {
			opened, openErr := rt.openDelegatedCastAssignment(ctx, DelegatedCastRequest{
				Objective:           in.Objective,
				Kind:                types.EngineeringAssignmentImplementation,
				CommitmentControlID: controlID,
				CasterRun:           *r.rec,
				CasterAgentID:       r.scope.FromAgentID,
				TargetDocID:         in.ToDesk,
				ScopeDigestSeed:     r.scope.CellID + ":" + in.LocalID,
			})
			if openErr != nil {
				return 0, fmt.Errorf("reduce: delegated cast admission: %w", openErr)
			}
			if rt.kernelMode && rt.scheduleActor != nil {
				rt.armDelegatedCastSpawn(opened.Assignment, in.Objective, "")
			} else if _, resumeErr := rt.resumeDelegatedCastAssignment(ctx, opened.Assignment, DelegatedCastRequest{
				Objective: in.Objective, Kind: opened.Assignment.Binding.Kind,
			}); resumeErr != nil {
				return 0, fmt.Errorf("reduce: delegated cast spawn (non-kernel): %w", resumeErr)
			}
		}
	}
	// An escalate carrying an action payload is a privileged-execution request
	// (the execution_request packet kind on the carrier): validate the guarded
	// action schema — type/objective + explicit per-action safety — before the
	// envelope mails. The safety contract is the same shape the retired
	// update_coagent execution_request validation enforced.
	if in.Kind == yaegikernel.IntentEscalate && strings.TrimSpace(in.Actions) != "" {
		var actions []types.CoagentPacketAction
		if err := json.Unmarshal([]byte(in.Actions), &actions); err != nil {
			return 0, fmt.Errorf("reduce: escalate_actions payload is not a valid actions array: %w", err)
		}
		if len(actions) == 0 {
			return 0, fmt.Errorf("reduce: escalate_actions requires at least one action")
		}
		for i, action := range actions {
			if err := validateCoagentPacketAction(action, true); err != nil {
				return 0, fmt.Errorf("reduce: escalate_actions actions[%d]: %w", i, err)
			}
		}
	}
	// A packet-bodied report carries the coagent packet schema as its body
	// (mission R2 — the update_coagent packet contract surviving on the
	// carrier). Validate it with the same payload validator the retired tool
	// enforced, so kind/claims/sources/actions/questions keep their contract.
	if in.Kind == yaegikernel.IntentReport && strings.TrimSpace(in.Packet) != "" {
		var packet types.CoagentSourcePacketPayload
		if err := json.Unmarshal([]byte(in.Packet), &packet); err != nil {
			return 0, fmt.Errorf("reduce: report packet is not a valid packet body: %w", err)
		}
		if err := validateCoagentSourcePacketPayload(packet); err != nil {
			return 0, fmt.Errorf("reduce: report packet invalid: %w", err)
		}
	}
	// Addressed acts mail their envelope so the target desk observes the act.
	var seq uint64
	switch in.Kind {
	case yaegikernel.IntentCast, yaegikernel.IntentAsk, yaegikernel.IntentNote,
		yaegikernel.IntentReply, yaegikernel.IntentEscalate, yaegikernel.IntentReport:
		seq, err = castStagedIntent(ctx, r.mb, r.scope, in)
	}
	return seq, err
}

func isSemanticActKind(kind string) bool {
	switch kind {
	case yaegikernel.IntentCast, yaegikernel.IntentAsk, yaegikernel.IntentNote,
		yaegikernel.IntentReply, yaegikernel.IntentCancel, yaegikernel.IntentEscalate,
		yaegikernel.IntentPrecommit, yaegikernel.IntentReport, yaegikernel.IntentResolve:
		return true
	}
	return false
}

// commitmentRecordForIntent derives the CommitmentRecord body for a staged
// act. The record id is deterministic (cell + local id) so a replayed cell
// re-derives the same record rather than minting a duplicate.
func commitmentRecordForIntent(scope ReductionScope, in yaegikernel.StagedIntent) types.CommitmentRecord {
	rec := types.CommitmentRecord{
		SchemaID:    types.CommitmentRecordSchemaV1,
		RecordID:    fmt.Sprintf("%s:%s:%s", scope.CellID, in.Kind, in.LocalID),
		Discrepancy: types.DiscrepancyUnresolved,
		Provenance: types.CommitmentProvenance{
			AgentID:    scope.FromAgentID,
			ContextRef: scope.ChannelID,
		},
		// Addressee is the ledger-side target-desk binding the packet envelope
		// would otherwise carry; EvidenceRefs preserve the act's typed
		// evidence/execution references so an evidence reader can scope and
		// materialize sources from the record alone.
		Addressee:    commitmentIntentAddressee(in),
		EvidenceRefs: commitmentIntentEvidenceRefs(in),
	}
	switch in.Kind {
	case yaegikernel.IntentPrecommit:
		rec.Prediction = types.CommitmentPrediction{Hypothesis: in.Statement}
	case yaegikernel.IntentReport:
		// A packet-bodied report preserves the full coagent packet as the
		// claim body; a thin report uses the claim text.
		if strings.TrimSpace(in.Packet) != "" {
			rec.Prediction = types.CommitmentPrediction{Hypothesis: in.Packet}
			// A packet-bodied report also lifts each source's target URI into
			// the typed EvidenceRefs so the ledger evidence resolver can
			// materialize entities without decoding the opaque packet body.
			var packet types.CoagentSourcePacketPayload
			if err := json.Unmarshal([]byte(in.Packet), &packet); err == nil {
				for _, src := range packet.Sources {
					if uri := strings.TrimSpace(src.Target.URI); uri != "" {
						rec.EvidenceRefs = append(rec.EvidenceRefs, uri)
					}
				}
			}
		} else {
			rec.Prediction = types.CommitmentPrediction{Hypothesis: in.Claim}
		}
	case yaegikernel.IntentResolve:
		// Resolution writes the outcome as a linked record on the ledger —
		// never a rewrite of the target commitment (the ledger stays an
		// auditable append-only event log). The record carries the verdict
		// class, the resolver's observation, and the resolve timestamp so the
		// score-accrual layer can read resolution→discrepancy without an
		// index.
		discrepancy := resolveOutcomeDiscrepancy(in.OutcomeVal)
		now := time.Now().UTC().Format(time.RFC3339Nano)
		rec.Discrepancy = discrepancy
		rec.Observation = types.CommitmentObservation{
			Excerpt:    in.OutcomeVal,
			SourceRef:  in.TargetRef,
			ObservedAt: now,
		}
		rec.Scores = []types.CommitmentScore{{
			ScorerModelID: scope.FromAgentID,
			Answers:       map[string]string{"outcome": in.OutcomeVal, "discrepancy": string(discrepancy)},
			ScoredAt:      now,
		}}
		rec.Provenance.ResolvedAt = now
		rec.RelatedIDs = []string{in.TargetRef}
	}
	if in.TargetRef != "" {
		rec.ParentID = in.TargetRef
	}
	return rec
}

// commitmentIntentAddressee returns the desk/actor the act is addressed to —
// the ledger-side analogue of the packet envelope's target_agent_id. ToDesk
// wins (a cast/report addressed to a desk); ResolverID is the fallback for a
// report that names the actor expected to resolve it.
func commitmentIntentAddressee(in yaegikernel.StagedIntent) string {
	if s := strings.TrimSpace(in.ToDesk); s != "" {
		return s
	}
	return strings.TrimSpace(in.ResolverID)
}

// commitmentIntentEvidenceRefs collects the act's typed evidence and execution
// references verbatim. Packet-bodied reports additionally lift each source's
// target URI in the IntentReport branch.
func commitmentIntentEvidenceRefs(in yaegikernel.StagedIntent) []string {
	refs := make([]string, 0, len(in.EvidenceRefs)+len(in.ExecutionRefs))
	refs = append(refs, in.EvidenceRefs...)
	refs = append(refs, in.ExecutionRefs...)
	return refs
}

// resolveOutcomeDiscrepancy maps a resolver's free-form outcome verdict onto
// the commitment ledger's discrepancy classes. Known verdicts normalize to
// their class; an unrecognized free-form outcome is kept verbatim in the
// record's observation and defaults to unresolved (the outcome never became
// verifiable as a typed class) so resolution never fails on phrasing.
func resolveOutcomeDiscrepancy(outcome string) types.DiscrepancyClass {
	switch strings.ToLower(strings.TrimSpace(outcome)) {
	case "confirmed", "correct", "held", "true", "pass", "passed", "success", "succeeded":
		return types.DiscrepancyConfirmed
	case "qualified", "partial", "partially", "mostly", "mixed":
		return types.DiscrepancyQualified
	case "contradicted", "wrong", "false", "fail", "failed", "refuted":
		return types.DiscrepancyContradicted
	default:
		return types.DiscrepancyUnresolved
	}
}

// isAssignedDesk reports whether this reduction serves an exact bound
// assignment run; its messages carry the update_coagent authority contract.
func (r *rlmCallReduction) isAssignedDesk() bool {
	return r != nil && r.rec != nil && metadataStringValue(r.rec.Metadata, "assignment_id") != ""
}

// commitFreezeIntent reduces a staged Freeze intent through the same freeze
// body the commit_transaction tool runs; the capsule handle is the bound
// handle, never model input. The mutation-role gate is the same predicate
// the JSON tool enforces: an exact bound writable Engineering assignment.
func (r *rlmCallReduction) commitFreezeIntent(ctx context.Context, in yaegikernel.StagedIntent) (map[string]any, error) {
	if r.rec == nil || r.toolCtx == nil || r.toolCtx.Executor == nil {
		return nil, fmt.Errorf("reduce: freeze intent without assignment authority")
	}
	if _, err := requireCapsuleMutationRole(ctx); err != nil {
		return nil, err
	}
	if r.toolCtx.OperationStore == nil {
		return nil, fmt.Errorf("reduce: freeze intent without self-development operation authority")
	}
	trajectoryID := trajectoryIDForRun(r.rec)
	if trajectoryID == "" {
		return nil, fmt.Errorf("reduce: freeze intent without trajectory binding")
	}
	if _, err := r.toolCtx.OperationStore.GetByTrajectory(ctx, r.toolCtx.ComputerID, trajectoryID); err != nil {
		return nil, fmt.Errorf("reduce: resolve self-development operation: %w", err)
	}
	return freezeCapsuleEffectBundle(ctx, r.toolCtx, r.rec, r.toolCtx.CapsuleHandle,
		in.BuildRecipeRef, in.TestReceipts, in.DependencyToolchainRefs)
}

// commitVerifyIntent reduces a staged Verify intent through the same
// verification body the record_self_development_verification tool runs. The
// operation binding resolves from the run's trajectory — the mounted bundle
// the cell inspected is the operation's frozen bundle by construction — and
// the intent's bundle digest must equal the operation's durable digest, so a
// decision can never land on a bundle other than the one the cell examined.
func (r *rlmCallReduction) commitVerifyIntent(ctx context.Context, in yaegikernel.StagedIntent) (map[string]any, error) {
	if r.rec == nil || r.toolCtx == nil || r.toolCtx.OperationStore == nil {
		return nil, fmt.Errorf("reduce: verify intent without verification authority")
	}
	trajectoryID := trajectoryIDForRun(r.rec)
	if trajectoryID == "" {
		return nil, fmt.Errorf("reduce: verify intent without trajectory binding")
	}
	operation, err := r.toolCtx.OperationStore.GetByTrajectory(ctx, r.toolCtx.ComputerID, trajectoryID)
	if err != nil {
		return nil, fmt.Errorf("reduce: resolve self-development operation: %w", err)
	}
	if in.BundleDigest != operation.BundleDigest {
		return nil, fmt.Errorf("reduce: verify %s bundle digest does not match the operation's frozen bundle", in.LocalID)
	}
	return recordSelfDevelopmentVerification(ctx, r.toolCtx, r.rec, operation.OperationID, operation.BundleDigest, in.Decision, in.VerifierRefs)
}

// commitMessageIntent reduces one staged Message intent on an assigned desk
// through the update_coagent authority path: the intent body is the
// CoagentSourcePacketPayload JSON, the desk is the explicit target agent, and
// the same durable update + wake sequence the retired tool ran executes here.
// It returns the channel sequence of the emitted message event.
func (r *rlmCallReduction) commitMessageIntent(ctx context.Context, in yaegikernel.StagedIntent) (uint64, error) {
	rt := r.rt()
	if rt == nil || rt.store == nil {
		return 0, fmt.Errorf("reduce: message intent without update authority")
	}
	var payload types.CoagentSourcePacketPayload
	decoder := json.NewDecoder(strings.NewReader(in.Body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return 0, fmt.Errorf("reduce: message %s body is not a coagent source packet: %w", in.LocalID, err)
	}
	if decoder.More() {
		return 0, fmt.Errorf("reduce: message %s body carries trailing data", in.LocalID)
	}
	packet := normalizeCoagentSourcePacketPayload(payload)
	if err := validateCoagentSourcePacketPayload(packet); err != nil {
		return 0, err
	}
	if in.MsgKind != "" && in.MsgKind != packet.Kind {
		return 0, fmt.Errorf("reduce: message %s kind %q does not match packet kind %q", in.LocalID, in.MsgKind, packet.Kind)
	}
	execution := toolregistry.ExecutionContextFrom(ctx)
	execution.ToolCallID = intentIdempotencyKey(r.scope, in.LocalID, in.ToDesk, in.Body)
	toolCallCtx := toolregistry.WithExecutionContext(ctx, execution)
	authority, err := resolveCoagentUpdateAuthorityWithStore(toolCallCtx, rt, rt.store, strings.TrimSpace(in.ToDesk), "")
	if err != nil {
		return 0, err
	}
	update := types.CoagentSourcePacket{
		OwnerID: authority.callerRun.OwnerID, ComputerID: authority.callerRun.ComputerID,
		AgentID: authority.callerRun.AgentID, TargetAgentID: authority.target.AgentID,
		ChannelID: authority.target.ChannelID, TrajectoryID: authority.trajectoryID,
		Role: authority.callerProfile, SourceRunID: authority.callerRun.RunID,
		Packet: packet, CreatedAt: time.Now().UTC(),
	}
	if authority.callerProfile == agentprofile.Engineering && authority.targetProfile == agentprofile.Management {
		update.Direction = types.LifecyclePacketDirectionProducerReport
	}
	update.UpdateID = deriveWorkerUpdateID(update)
	update.Content = buildWorkerUpdateMessage(update)
	message := &types.ChannelMessage{
		ChannelID: update.ChannelID, From: update.SourceRunID,
		FromAgentID: update.AgentID, FromRunID: update.SourceRunID,
		ToAgentID: update.TargetAgentID, TrajectoryID: update.TrajectoryID,
		Role: update.Role, Content: update.Content, Timestamp: update.CreatedAt,
	}
	var stored types.CoagentSourcePacket
	var created bool
	if authority.lifecycle {
		payloadDigest, digestErr := store.ComputeLifecycleUpdatePayloadDigest(update.Packet, update.Content)
		if digestErr != nil {
			return 0, digestErr
		}
		queue := types.QueueLifecycleUpdateRequest{
			OwnerID: update.OwnerID, ComputerID: update.ComputerID,
			CommandID: "lifecycle-queue:" + update.UpdateID, TrajectoryID: update.TrajectoryID,
			TargetAgentID: update.TargetAgentID, ProducerAgentID: update.AgentID,
			ProducerUpdateID: update.ProducerUpdateID, UpdateID: update.UpdateID,
			ChannelID: update.ChannelID, Role: update.Role, SourceRunID: update.SourceRunID,
			Packet: update.Packet, Content: update.Content, PayloadDigest: payloadDigest,
			WorkItemID: authority.workItemID,
		}
		queue.CommandDigest, _ = store.ComputeQueueLifecycleUpdateDigest(queue)
		queued, queueErr := rt.store.QueueLifecycleUpdate(ctx, queue)
		if queueErr != nil {
			return 0, fmt.Errorf("queue durable lifecycle update: %w", queueErr)
		}
		if queued.Update == nil {
			return 0, fmt.Errorf("queue durable lifecycle update: reducer returned no update projection")
		}
		stored, created = *queued.Update, !queued.Replay
		if stored.Disposition == types.UpdatePending && created {
			rt.emitChannelMessageEvent(ctx, *message, update.OwnerID)
			rt.wakeUpdatedCoagent(ctx, stored)
		}
	} else {
		var err error
		stored, created, err = rt.store.DispatchWorkerUpdate(ctx, update, message)
		if err != nil {
			return 0, err
		}
		if stored.Disposition == "" && created {
			rt.emitChannelMessageEvent(ctx, *message, update.OwnerID)
			rt.wakeUpdatedCoagent(ctx, stored)
		}
	}
	if stored.Disposition == "" && !created {
		if err := validateExistingWorkerUpdate(stored, update); err != nil {
			return 0, err
		}
	}
	return uint64(stored.MessageSeq), nil
}

// commitCompleteIntent authors the assignment fate for one staged Complete
// intent. It returns whether the committed fate is terminal. The report is
// built exactly as the retired record_assignment_result tool built it:
// execution refs resolve to bound commands/outputs, a terminal completed
// pass requires at least one, and the summary is mandatory. The cell intent
// identity substitutes for the provider tool_call_id in the partial-report
// identity (a content-derived key so a replayed cell replays the same
// report rather than minting a second one).
func (r *rlmCallReduction) commitCompleteIntent(ctx context.Context, in yaegikernel.StagedIntent) (bool, error) {
	if r.rec == nil || r.toolCtx == nil || r.toolCtx.Executor == nil {
		return false, fmt.Errorf("reduce: complete intent without assignment authority")
	}
	summary := strings.TrimSpace(in.Summary)
	if summary == "" {
		return false, fmt.Errorf("reduce: complete intent summary is required")
	}
	result := types.EngineeringAssignmentResultKind(strings.TrimSpace(in.Result))
	verdict := types.EngineeringAssignmentVerdict(strings.TrimSpace(in.Verdict))
	evidenceRefs := sortedUniqueStrings(in.EvidenceRefs)
	executionRefs := trimNonEmptyStrings(in.ExecutionRefs)
	// An implementation assignment cannot issue a verification verdict; the
	// store requires "none". Coerce rather than reject: the verdict is
	// meaningless for implementation, and a reduce error here poisons the
	// capsule mid-run with no in-cell recovery path. The tray already
	// rejects non-enum verdicts at staging, so only typed-but-wrong values
	// (pass/fail/abstain) or empty reach this point. This must run before
	// the execution-ref check below: a completed+pass implementation report
	// with no refs would otherwise hit that check first and strand.
	if kind := metadataStringValue(r.rec.Metadata, "assignment_kind"); kind == string(types.EngineeringAssignmentImplementation) &&
		verdict != types.EngineeringVerdictNone {
		verdict = types.EngineeringVerdictNone
	}
	if result == types.EngineeringResultCompleted && verdict == types.EngineeringVerdictPass && len(executionRefs) == 0 {
		return false, fmt.Errorf("reduce: terminal completed pass requires at least one valid execution_ref")
	}
	receipts, err := r.toolCtx.Executor.ResolveExecutionReceipts(executionRefs)
	if err != nil {
		return false, err
	}
	report := types.EngineeringAssignmentReport{Result: result, Verdict: verdict, Summary: summary,
		EvidenceRefs: evidenceRefs, Commands: recordedCommandsFromReceipts(receipts), Outputs: recordedOutputsFromReceipts(receipts)}
	// Content-derived identity: a replayed cell (same intent content) replays
	// the same report instead of minting a second one.
	intentIdentity := "rlm-complete:" + r.scope.RunID + ":" + objectgraph.SHA256([]byte(strings.Join([]string{
		in.Result, in.Verdict, summary, strings.Join(evidenceRefs, "\x1f"), strings.Join(executionRefs, "\x1f"),
	}, "\x00")))
	rt := r.rt()
	if rt == nil {
		return false, fmt.Errorf("reduce: complete intent without runtime")
	}
	cmdResult, err := rt.recordAssignedEngineeringReport(ctx, r.rec, intentIdentity, report)
	if err != nil {
		return false, err
	}
	if !cmdResult.Replay && cmdResult.Update != nil {
		rt.wakeUpdatedCoagent(ctx, *cmdResult.Update)
	}
	if result != types.EngineeringResultPartial && cmdResult.Assignment.Disposition == types.EngineeringAssignmentCompleted &&
		cmdResult.Assignment.Binding.Kind == types.EngineeringAssignmentImplementation {
		// Verification chaining is host-side: a completed implementation on a
		// document-bound trajectory opens the verification assignment when the
		// bound self-development operation is frozen.
		if _, recErr := rt.ReconcileEngineeringDeskForTrajectory(ctx, cmdResult.Assignment.Binding.OwnerID,
			cmdResult.Assignment.Binding.TrajectoryID); recErr != nil {
			log.Printf("runtime: engineering desk verification reconcile after %s: %v", cmdResult.Assignment.AssignmentID, recErr)
		}
	}
	return result != types.EngineeringResultPartial, nil
}

func (r *rlmCallReduction) rt() *Runtime {
	if mb, ok := r.mb.(*Runtime); ok {
		return mb
	}
	return nil
}
