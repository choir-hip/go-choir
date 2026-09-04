package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
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
	ReturnTo    string // supervisor desk: spawn requests and completion reports
	Cursor      uint64 // durable unread cursor entering the cell
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
// engineering authority. Super may spawn any role; CoSuper may spawn
// researchers and peers; researchers may only fan out to researchers.
func spawnRoleAllowed(spawnerRole, childRole string) bool {
	spawner := strings.ToLower(strings.TrimSpace(spawnerRole))
	child := strings.ToLower(strings.TrimSpace(childRole))
	switch spawner {
	case "super":
		return true
	case "co-super", "cosuper", "engineering":
		return child == "researcher" || child == "co-super" || child == "cosuper" || child == "engineering"
	case "researcher", "research":
		return child == "researcher" || child == "research"
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
	for _, in := range intents {
		switch in.Kind {
		case yaegikernel.IntentMessage:
			if in.ToDesk == "" {
				return fmt.Errorf("reduce: message %s missing destination", in.LocalID)
			}
			if len(in.Body) > yaegikernel.MaxIntentBody {
				return fmt.Errorf("reduce: message %s exceeds body quota", in.LocalID)
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
			switch in.Result {
			case yaegikernel.CompleteCompleted, yaegikernel.CompleteFailed, yaegikernel.CompleteBlocked:
			default:
				return fmt.Errorf("reduce: complete result %q invalid", in.Result)
			}
		default:
			return fmt.Errorf("reduce: unknown intent kind %q", in.Kind)
		}
	}
	if complete > 1 {
		return fmt.Errorf("reduce: at most one complete per cell")
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
	From         string   `json:"from"`
}

func encodeEnvelope(env rlmEnvelope) string {
	raw, err := json.Marshal(env)
	if err != nil {
		return rlmEnvelopeV1 + `{"kind":"encode_error"}`
	}
	return rlmEnvelopeV1 + string(raw)
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
		var to, content string
		switch in.Kind {
		case yaegikernel.IntentMessage:
			to = in.ToDesk
			content = encodeEnvelope(rlmEnvelope{Kind: "message", MsgKind: in.MsgKind, Body: in.Body, From: scope.FromAgentID})
		case yaegikernel.IntentSpawn:
			to = scope.ReturnTo
			content = encodeEnvelope(rlmEnvelope{Kind: "spawn_request", Role: in.Role, Objective: in.Objective, From: scope.FromAgentID})
		case yaegikernel.IntentComplete:
			to = scope.ReturnTo
			content = encodeEnvelope(rlmEnvelope{Kind: "complete", Result: in.Result, Verdict: in.Verdict, Summary: in.Summary, EvidenceRefs: in.EvidenceRefs, From: scope.FromAgentID})
		}
		seq, err := mb.ChannelCast(ctx, scope.ChannelID, to, "", scope.FromAgentID, scope.FromRole, content)
		if err != nil {
			return ReductionReceipt{Cursor: scope.Cursor}, fmt.Errorf("reduce: persist %s: %w", in.LocalID, err)
		}
		receipt.Intents = append(receipt.Intents, ReducedIntent{LocalID: in.LocalID, Seq: seq, Kind: in.Kind})
	}
	receipt.Committed = true
	return receipt, nil
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

// rlmCallReduction carries one go_eval call's inbox assembly through to its
// post-cell commit. Inactive outside RLM mode or without a runtime: the tools
// path stays byte-identical then, and reduction is a no-op.
type rlmCallReduction struct {
	active    bool
	mb        rlmMailbox
	st        rlmCursorStore
	scope     ReductionScope
	inbox     []yaegikernel.IncomingMessage
	highWater uint64
	receipt   ReductionReceipt
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
		active: true,
		mb:     rt,
		st:     rt.store,
		scope: ReductionScope{
			FromAgentID: execCtx.AgentID,
			FromRole:    string(toolCtx.Role),
			ChannelID:   channel,
			RunID:       execCtx.RunID,
			OwnerID:     execCtx.OwnerID,
			ReturnTo:    requester,
			Cursor:      cursor,
		},
		inbox:     inbox,
		highWater: highWater,
	}
}

// commit reduces a successful cell's staged tray and advances the durable
// inbox cursor past the consumed snapshot and the newly persisted intents.
// Failed cells never reach this path: their tray is dropped and the cursor
// holds, so unread mail is never acknowledged for work that did not happen.
func (r *rlmCallReduction) commit(ctx context.Context, intents []yaegikernel.StagedIntent) error {
	if r == nil || !r.active {
		return nil
	}
	receipt, err := ReduceCellIntents(ctx, r.mb, r.scope, intents, true)
	if err != nil {
		return err
	}
	highWater := r.highWater
	for _, in := range receipt.Intents {
		if in.Seq > highWater {
			highWater = in.Seq
		}
	}
	if highWater < r.scope.Cursor {
		highWater = r.scope.Cursor
	}
	if err := CommitInboxCursor(ctx, r.st, r.scope.OwnerID, r.scope.RunID, r.scope.ChannelID, highWater); err != nil {
		return err
	}
	r.receipt = receipt
	r.receipt.Cursor = highWater
	r.receipt.Committed = true
	return nil
}