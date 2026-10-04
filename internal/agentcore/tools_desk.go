package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// desk_go_eval (R3b): the host desk-cell carrier's sole tool for a non-capsule
// desk. It evaluates model-authored Go source in a killable host session worker
// — never in the daemon's address space — and reduces the cell's staged intents
// through the same ledger path capsule_go_eval uses.
// The worker is re-executed from the daemon binary (autoputer desk-session);
// SpawnDeskSessionWorker supplies the socketpair + process-group hardening.

const deskWorkerArg = "desk-session"

// deskWorkerMemoryLimitBytes is the D2 memory cap applied to every desk
// session worker via RLIMIT_AS (R3r): a model-authored cell cannot grow
// its address space unboundedly and OOM the daemon. The worker re-execs
// the autoputer binary — its linked address space needs headroom far
// above a cell's working set — so the cap binds growth, not startup
// size (8 GiB virtual; Linux staging enforced).
const deskWorkerMemoryLimitBytes uint64 = 8 << 30 // 8 GiB

type deskSessionWorkers struct {
	mu      sync.Mutex
	workers map[string]*yaegikernel.DeskSessionWorker
}

func newDeskSessionWorkers() *deskSessionWorkers {
	return &deskSessionWorkers{workers: map[string]*yaegikernel.DeskSessionWorker{}}
}

// deskSessionWorkers returns the Runtime's shared worker pool, lazily
// allocated so desk cells never spawn a worker before one is requested.
func (rt *Runtime) deskSessionWorkers() *deskSessionWorkers {
	rt.deskWorkersMu.Lock()
	defer rt.deskWorkersMu.Unlock()
	if rt.deskWorkers == nil {
		rt.deskWorkers = newDeskSessionWorkers()
	}
	return rt.deskWorkers
}

// deskWorkerFor returns (or spawns) the activation's host session worker.
// bin is the daemon's own executable; cfg carries the desk identity/bounds.
func (m *deskSessionWorkers) deskWorkerFor(activationID string, cfg yaegikernel.DeskSessionWorkerConfig) (*yaegikernel.DeskSessionWorker, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.workers == nil {
		m.workers = map[string]*yaegikernel.DeskSessionWorker{}
	}
	if w := m.workers[activationID]; w != nil && !w.Dead() {
		return w, nil
	}
	if old := m.workers[activationID]; old != nil {
		old.Close()
	}
	w, err := yaegikernel.SpawnDeskSessionWorker(cfg)
	if err != nil {
		delete(m.workers, activationID)
		return nil, err
	}
	m.workers[activationID] = w
	return w, nil
}

// release terminates and forgets the activation's worker.
func (m *deskSessionWorkers) release(activationID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if w := m.workers[activationID]; w != nil {
		w.Close()
		delete(m.workers, activationID)
	}
}

// newDeskGoEvalTool evaluates model-authored Go source for a non-capsule desk
// inside a dedicated host session worker. It mirrors newCapsuleGoEvalTool but
// carries no capsule obligation: the desk's authority is its profile/role, and
// the containment boundary is the subprocess, not a guest VM. Staged intents
// reduce through rlmReductionForDeskCall onto the canonical ledger.
func newDeskGoEvalTool(rt *Runtime, workers *deskSessionWorkers, deskRole string) toolregistry.Tool {
	type args struct {
		Source    string `json:"source"`
		TimeoutMS int    `json:"timeout_ms"`
	}
	return toolregistry.Tool{
		Name:        "desk_go_eval",
		Description: "Evaluate model-authored Go source for this desk in a killable host subprocess (restricted stdlib, per-role choir verbs).",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"source":     map[string]any{"type": "string", "description": "Raw Go source for one REPL cell; stage desk acts with choir.* verbs."},
			"timeout_ms": map[string]any{"type": "integer", "description": "Evaluation timeout in milliseconds."},
		}, []string{"source"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			execCtx := toolregistry.ExecutionContextFrom(ctx)
			var input args
			if err := json.Unmarshal(raw, &input); err != nil {
				return "", err
			}
			if strings.TrimSpace(input.Source) == "" {
				return "", fmt.Errorf("desk_go_eval: source is required")
			}
			activationID := deskWorkerActivationID(execCtx)
			if activationID == "" {
				return "", fmt.Errorf("desk_go_eval: no activation identity for desk worker")
			}
			workerCfg := yaegikernel.DeskSessionWorkerConfig{
				Bin:       deskWorkerBinary(),
				WorkerArg: deskWorkerArg,
				Session: yaegikernel.SessionWorkerConfig{
					AllowedPackages:  yaegikernel.DefaultSafeStdlibPackagesList(),
					ComputerID:       execCtx.ComputerID,
					ActivationID:     activationID,
					Epoch:            deskWorkerEpoch(execCtx),
					AllowedRoot:      deskWorkerRoot(execCtx),
					Role:             deskRole,
					MemoryLimitBytes: deskWorkerMemoryLimitBytes,
				},
				ProcessGroup: true,
				// Emit is a durable host write: the worker's ActionEmit crosses
				// the session socket and lands here. Scope is derived per-call
				// from the evaluating ctx (a worker persists across cells, so
				// spawn-time capture would bind a stale channel/agent).
				Emit: func(emitCtx context.Context, payload yaegikernel.EmitPayload) (yaegikernel.EmitResult, error) {
					return rt.deskEmitSignal(emitCtx, payload)
				},
				// Egress services the cell's host-mediated research verbs
				// (choir.* → StreamBrokerEgress). Evidence/run-memory actions
				// resolve through rt-bound tools; network/content actions go to
				// researchDeps.HostEgress under the shared egress ledger — one
				// deps surface, never an open socket.
				Egress: func(egrCtx context.Context, action yaegikernel.BrokerAction, payload json.RawMessage) (json.RawMessage, error) {
					if fn, ok := rt.egressEvidenceTool()[string(action)]; ok {
						out, err := fn(egrCtx, payload)
						return json.RawMessage(out), err
					}
					if rt.researchDeps == nil {
						return nil, fmt.Errorf("%s host unavailable", action)
					}
					return rt.researchDeps.HostEgress(egrCtx, string(action), payload)
				},
			}
			w, err := workers.deskWorkerFor(activationID, workerCfg)
			if err != nil {
				return "", fmt.Errorf("desk_go_eval: spawn worker: %w", err)
			}
			reduction := rlmReductionForDeskCall(ctx, rt)
			// R3d: a texture cell reads its bound document's head via
			// choir.ReadDoc(); inject the cell-start snapshot on the same
			// frame as the inbox so authoring is a genuine turn, not blind.
			var docSnapshot *yaegikernel.DocSnapshot
			if deskRole == agentprofile.Texture && execCtx.RunRecord != nil {
				docSnapshot = textureDocSnapshotForRun(ctx, rt, execCtx.RunRecord)
			}
			// R4: the acting desk's commitment pack rides the same frame —
			// score-free by construction (types.ActingPack), so the
			// epistemic boundary holds on the wire.
			pack := actingCommitmentPackForDesk(ctx, rt, execCtx)
			evalCtx := ctx
			if input.TimeoutMS > 0 {
				var cancel context.CancelFunc
				evalCtx, cancel = context.WithTimeout(ctx, time.Duration(input.TimeoutMS)*time.Millisecond)
				defer cancel()
			}
			// RLM prompt-as-variable: pending update_coagent records ride the
			// frame so choir.Updates() exposes them inside the cell. The chat
			// wake turn carries only ids.
			updates := pendingCellUpdates(ctx, rt, execCtx)
			emits := pendingCellEmits(ctx, rt, execCtx)
			if reduction.active {
				rt.armCellTerminalDeadline(ctx, reduction)
			}
			res, evalErr := w.EvalCell(evalCtx, input.Source, reduction.inbox, docSnapshot, pack, updates, emits)

			result := yaegikernel.SessionResult{}
			if evalErr != nil {
				if w.Dead() {
					workers.release(activationID)
				}
				result.Error = evalErr.Error()
			} else {
				result = res
			}
			if reduction.active && result.Error != "" {
				fate := cellFateFailure
				if evalCtx.Err() == context.DeadlineExceeded {
					fate = cellFateTimeout
				}
				if fateErr := reduction.abort(context.Background(), fate, result.Error); fateErr != nil {
					return "", fateErr
				}
			}
			if reduction.active && result.Error == "" {
				if rerr := reduction.commit(ctx, result.Intents); rerr != nil {
					return "", rerr
				}
				for _, in := range reduction.receipt.Intents {
					result.Receipts = append(result.Receipts, fmt.Sprintf("rlm:%s:%d", in.Kind, in.Seq))
				}
				// A texture desk cell that staged no texture_apply intent did not
				// commit a turn, so the owner-revision trigger would stay pending
				// forever (defect #5: "activation returned without disposing exact
				// trigger"). Consume it with a no_semantic_change turn — the desk
				// authored nothing, which is still a disposition of the wake.
				if deskRole == agentprofile.Texture {
					if cerr := rt.consumeIdleTextureTrigger(ctx, result.Intents); cerr != nil {
						return "", fmt.Errorf("desk_go_eval: consume texture trigger: %w", cerr)
					}
				}
			}
			out, _ := json.Marshal(map[string]any{
				"stdout":   result.Stdout,
				"stderr":   result.Stderr,
				"error":    result.Error,
				"duration": result.DurationMs,
				"reuse":    result.Reuse,
				"diag":     result.DiagKind,
				"receipts": result.Receipts,
				"intents":  len(result.Intents),
			})
			return string(out), nil
		},
	}
}

// textureDocSnapshotForRun loads the bound texture document's head for a
// texture desk cell (R3d) — the current revision id + content injected as the
// cell-start ReadDoc snapshot. The doc id resolves the same way the authoring
// commit does (run metadata doc_id, else the texture channel). Returns nil
// when the run is not doc-bound or the doc cannot be read; the cell still
// sees the doc on its prompt in that case.
func textureDocSnapshotForRun(ctx context.Context, rt *Runtime, rec *types.RunRecord) *yaegikernel.DocSnapshot {
	if rt == nil || rt.store == nil || rec == nil {
		return nil
	}
	docID := strings.TrimSpace(metadataStringValue(rec.Metadata, "doc_id"))
	if docID == "" {
		docID = strings.TrimSpace(rec.ChannelID)
	}
	if docID == "" {
		return nil
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	computerID := strings.TrimSpace(rec.ComputerID)
	var doc types.Document
	var err error
	if subject, subErr := rt.store.GetAgentByScope(ctx, ownerID, computerID, rec.AgentID); subErr == nil && subject.LifecycleVersion > 0 {
		doc, err = rt.store.GetLifecycleDocument(ctx, ownerID, computerID, docID)
	} else {
		doc, err = rt.store.GetDocument(ctx, docID, ownerID)
	}
	if err != nil || strings.TrimSpace(doc.CurrentRevisionID) == "" {
		return nil
	}
	var rev types.Revision
	if strings.TrimSpace(doc.TrajectoryID) != "" {
		rev, err = rt.store.GetLifecycleRevision(ctx, ownerID, computerID, doc.CurrentRevisionID)
	} else {
		rev, err = rt.store.GetRevision(ctx, doc.CurrentRevisionID, ownerID)
	}
	if err != nil {
		return nil
	}
	return &yaegikernel.DocSnapshot{
		DocID:      docID,
		RevisionID: rev.RevisionID,
		AuthorKind: string(rev.AuthorKind),
		Content:    rev.Content,
	}
}

// pendingCellUpdates loads the pending update_coagent records addressed to
// this desk's agent and maps them into the cell-visible PendingUpdate shape
// (RLM prompt-as-variable: the payload lives on the frame, not in chat).
func pendingCellUpdates(ctx context.Context, rt *Runtime, execCtx toolregistry.ExecutionContext) []yaegikernel.PendingUpdate {
	if rt == nil || rt.store == nil || execCtx.RunRecord == nil {
		return nil
	}
	rec := execCtx.RunRecord
	ownerID := strings.TrimSpace(rec.OwnerID)
	agentID := strings.TrimSpace(rec.AgentID)
	if ownerID == "" || agentID == "" || !runSupportsCoagentUpdateInjection(rec) {
		return nil
	}
	updates, err := rt.pendingCoagentUpdatesForRun(ctx, rec, ownerID, agentID, 100)
	if err != nil {
		return nil
	}
	if len(updates) == 0 {
		return nil
	}
	// The cell variable exposes the desk's pending backlog — every
	// deliverable update not yet disposed by a terminal write — regardless
	// of whether a chat wake turn already named it. Seen-ness exists to
	// dedupe chat injection, not to hide the payload from the cell.
	out := make([]yaegikernel.PendingUpdate, 0, len(updates))
	for _, u := range updates {
		id := strings.TrimSpace(u.UpdateID)
		if id == "" || !coagentUpdateDeliverableForRun(rec, u) {
			continue
		}
		out = append(out, yaegikernel.PendingUpdate{
			UpdateID:        id,
			FromAgentID:     strings.TrimSpace(u.AgentID),
			FromRole:        strings.TrimSpace(u.Role),
			ChannelID:       strings.TrimSpace(u.ChannelID),
			MessageSeq:      u.MessageSeq,
			WorkItemID:      strings.TrimSpace(u.WorkItemID),
			Packet:          u.Packet,
			HumanProjection: strings.TrimSpace(u.Content),
		})
	}
	return out
}

// pendingCellEmits drains emitted signals addressed to this desk's agent,
// after the desk's inbox cursor. Emits travel on the *sender's* channel with
// to_agent_id set, so the desk's own-channel ChannelRead never sees them;
// this is the boundary-drain read for choir.Emits(). The cell gets the full
// untrusted body; the chat boundary turn carries only a fixed-format notice.
// pendingCellEmits is the cell-frame adapter: drains addressed emissions for
// the run's desk agent into the bound choir.Emits() snapshot.
func pendingCellEmits(ctx context.Context, rt *Runtime, execCtx toolregistry.ExecutionContext) []yaegikernel.PendingEmit {
	if execCtx.RunRecord == nil {
		return nil
	}
	return pendingEmitsForRun(ctx, rt, execCtx.RunRecord, nil)
}

// pendingEmitsForRun drains emitted signals addressed to this desk's agent.
// Emits travel on the *sender's* channel with to_agent_id set, so the desk's
// own-channel ChannelRead never sees them; this is the boundary-drain read.
// The cell gets the full untrusted body (choir.Emits()); the chat boundary
// turn carries only a fixed-format notice (sender/kind/seq/snippet).
func pendingEmitsForRun(ctx context.Context, rt *Runtime, rec *types.RunRecord, seen map[string]bool) []yaegikernel.PendingEmit {
	if rt == nil || rt.store == nil || rec == nil {
		return nil
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	agentID := strings.TrimSpace(rec.AgentID)
	if ownerID == "" || agentID == "" || !runSupportsCoagentUpdateInjection(rec) {
		return nil
	}
	// Drain addressed emissions. Emit bodies are bound per-cell; the desk
	// consumes them by terminal write so a cursor the cell never processed
	// re-surfaces — drain from zero is safe (idempotent re-read) and lets the
	// desk decide disposition.
	msgs, err := rt.store.ListChannelMessagesTo(ctx, ownerID, agentID, 0, 200)
	if err != nil || len(msgs) == 0 {
		return nil
	}
	out := make([]yaegikernel.PendingEmit, 0, len(msgs))
	for _, m := range msgs {
		// Only ActionEmit envelopes are emissions: a message written through
		// the emit path always carries rlmEnvelopeV1 with Kind="emit". Plain
		// channel traffic (coagent updates, packets, legacy writes) is NOT an
		// emission — defaulting unenveloped content to emit re-delivers the
		// update payload as a notice snippet and inlines it into the wake
		// turn (the leak the warm-update test catches).
		kind, body := "", ""
		if rest, ok := strings.CutPrefix(m.Content, rlmEnvelopeV1); ok {
			var env rlmEnvelope
			if jerr := json.Unmarshal([]byte(rest), &env); jerr == nil {
				if env.Kind != "" {
					kind = env.Kind
					if kind == "message" && env.MsgKind != "" {
						kind = env.MsgKind
					}
				}
				body = env.Body
			}
		}
		if kind != "emit" {
			continue // only emission envelopes drain here; packets/other kinds use their own path
		}
		if seen != nil && seen[emitSeenKey(m.ChannelID, m.Seq)] {
			continue // already delivered at a prior boundary — suppress re-notification
		}
		out = append(out, yaegikernel.PendingEmit{
			ChannelID:   strings.TrimSpace(m.ChannelID),
			MessageSeq:  m.Seq,
			FromAgentID: strings.TrimSpace(m.FromAgentID),
			FromRole:    strings.TrimSpace(m.Role),
			Kind:        kind,
			Body:        body,
		})
	}
	return out
}

// deskWorkerBinary is the executable the session worker re-executes as
// `autoputer desk-session` — normally the daemon's own os.Executable. The
// override lets tests point at a real compiled autoputer instead of the test
// harness binary.
var deskWorkerBinOverride string

func deskWorkerBinary() string {
	if deskWorkerBinOverride != "" {
		return deskWorkerBinOverride
	}
	if bin, err := os.Executable(); err == nil {
		return bin
	}
	return "autoputer"
}

// deskWorkerActivationID identifies the desk activation a worker serves. Run
// identity is the stable key; the run's record supplies it when the tool
// context lacks an explicit activation field.
func deskWorkerActivationID(execCtx toolregistry.ExecutionContext) string {
	if execCtx.RunRecord != nil {
		if id := strings.TrimSpace(execCtx.RunRecord.RunID); id != "" {
			return id
		}
	}
	return strings.TrimSpace(execCtx.RunID)
}

// deskWorkerEpoch returns the handle-issuer epoch the worker fences on. For a
// non-capsule desk there is no capsule epoch; a positive constant satisfies
// the issuer's epoch>0 constraint. Desk epochs gain a canonical source when
// R3c binds desk activations to a lifecycle version.
func deskWorkerEpoch(execCtx toolregistry.ExecutionContext) uint64 {
	return 1
}

// deskWorkerRoot bounds the worker's filesystem root. Desks without file
// verbs get a scratch dir; the allowed root is the run's working dir.
func deskWorkerRoot(execCtx toolregistry.ExecutionContext) string {
	if wd := strings.TrimSpace(execCtx.WorkingDir); wd != "" {
		return wd
	}
	return os.TempDir()
}

// actingPackMaxItems bounds the commitment pack injected into a desk cell
// frame: enough to cover a working set without flooding model context.
const actingPackMaxItems = 16

// actingCommitmentPackForDesk builds the acting desk's score-free
// commitment pack (R4) from the commitment ledger: the desk's own
// committed acts plus acts addressed to it, joined with their resolution
// observations and discrepancies. The pack rides the cell frame so
// choir.Pack() reads it inside the worker without a network roundtrip.
// Nil on any ledger failure — the pack is context, never a gate.
func actingCommitmentPackForDesk(ctx context.Context, rt *Runtime, execCtx toolregistry.ExecutionContext) *types.ActingPack {
	if rt == nil || rt.store == nil {
		return nil
	}
	agentID := strings.TrimSpace(execCtx.AgentID)
	ownerID := strings.TrimSpace(execCtx.OwnerID)
	computerID := strings.TrimSpace(execCtx.ComputerID)
	if agentID == "" || ownerID == "" || computerID == "" {
		return nil
	}
	records, err := rt.store.ListCommitmentRecords(ctx, ownerID, computerID, "", 256)
	if err != nil {
		log.Printf("desk pack: commitment record list for %s: %v", agentID, err)
		return nil
	}
	pack := types.BuildActingPack(records, agentID, actingPackMaxItems)
	return &pack
}

// deskEmitSignal performs the host-side durable write for a desk cell's
// choir.Emit call. Emits bypass the cell tray: the host mails the emit
// envelope on the desk's channel and wakes the recipient before the blocked
// cell resumes. The envelope kind stays "emit" with the caller's signal kind
// preserved as msg_kind so the receiving desk sees the typed signal.
func (rt *Runtime) deskEmitSignal(ctx context.Context, payload yaegikernel.EmitPayload) (yaegikernel.EmitResult, error) {
	if rt == nil || rt.store == nil {
		return yaegikernel.EmitResult{}, fmt.Errorf("emit: store unavailable")
	}
	execCtx := toolregistry.ExecutionContextFrom(ctx)
	to := strings.TrimSpace(payload.ToDesk)
	if to == "" {
		return yaegikernel.EmitResult{}, fmt.Errorf("emit: destination desk required")
	}
	channel := channelIDForRun(execCtx.RunRecord)
	if channel == "" {
		channel = strings.TrimSpace(execCtx.ChannelID)
	}
	if channel == "" || strings.TrimSpace(execCtx.RunID) == "" {
		return yaegikernel.EmitResult{}, fmt.Errorf("emit: no bound channel for desk cell")
	}
	from := strings.TrimSpace(execCtx.AgentID)
	role := strings.TrimSpace(execCtx.Role)
	if role == "" && execCtx.RunRecord != nil {
		role = agentProfileForRun(execCtx.RunRecord)
	}
	content := encodeEnvelope(rlmEnvelope{Kind: "emit", MsgKind: strings.TrimSpace(payload.Kind), Body: payload.Body, From: from})
	// A deterministic idempotency key makes a re-emitted identical signal
	// collapse to the already-written record rather than duplicate it.
	sum := sha256.Sum256([]byte(to + "\x1f" + payload.Kind + "\x1f" + payload.Body + "\x1f" + execCtx.RunID))
	seq, err := rt.CastEnvelope(ctx, channel, to, from, role, content, "rlm-emit:"+hex.EncodeToString(sum[:8]))
	if err != nil {
		return yaegikernel.EmitResult{}, fmt.Errorf("emit: %w", err)
	}
	// Advisory piggyback: the newest emission seq addressed to the *calling*
	// desk, not the recipient — a mid-cell emit returns a watermark the cell
	// can compare against its consumed high-water. Non-fatal: a drain failure
	// degrades to no advisory rather than refusing the emit.
	advisory := rt.newestEmissionSeqTo(ctx, execCtx)
	return yaegikernel.EmitResult{Seq: seq, AdvisorySeq: advisory}, nil
}

// newestEmissionSeqTo returns the highest channel seq of emissions addressed
// to the calling desk — the advisory watermark riding an emit's s.call
// response. Read-only and non-fatal: the caller desk is resolved the same
// way the reduction resolves its cursor identity (run record's bound agent,
// else exec context agent). Returns 0 when the desk can't be resolved or the
// drain fails.
func (rt *Runtime) newestEmissionSeqTo(ctx context.Context, execCtx toolregistry.ExecutionContext) uint64 {
	if rt == nil || rt.store == nil {
		return 0
	}
	ownerID := ""
	agentID := strings.TrimSpace(execCtx.AgentID)
	if execCtx.RunRecord != nil {
		ownerID = strings.TrimSpace(execCtx.RunRecord.OwnerID)
		if id := strings.TrimSpace(execCtx.RunRecord.AgentID); id != "" {
			agentID = id
		}
	}
	if agentID == "" {
		return 0
	}
	msgs, err := rt.store.ListChannelMessagesTo(ctx, ownerID, agentID, 0, 1)
	if err != nil || len(msgs) == 0 {
		return 0
	}
	var max uint64
	for _, m := range msgs {
		if uint64(m.Seq) > max {
			max = uint64(m.Seq)
		}
	}
	return max
}

// consumeIdleTextureTrigger disposes the owner-revision wake when a texture
// desk cell completed without staging a texture_apply intent. Without this the
// trigger never consumes: texture_turn_committed needs an artifactRefs[1] ==
// the trigger head, which only a turn commit writes. A cell that authored no
// revision/decision still must answer the wake — the desk observed the owner
// revision and chose to change nothing, which is the no_semantic_change turn.
//
// The decision_kind must describe what the cell actually did, because the
// desk's authored acts (Ask/Resolve/Note/Report) are real work — the cell only
// declined a *doc* write. Recording "no_worker_needed" for a cell that staged
// an Ask launders a real act into "no act" (the s0m idle-mask defect): the
// audit row then claims the desk did nothing and the run reads as a completed
// no-op, destroying the retry signal. A cell that staged a semantic act but no
// apply disposes the wake as delegation_skipped — the work went elsewhere, the
// doc needed no edit. Only a cell that staged nothing at all is a true
// no_semantic_change.
func (rt *Runtime) consumeIdleTextureTrigger(ctx context.Context, intents []yaegikernel.StagedIntent) error {
	stagedAct := false
	for _, in := range intents {
		if in.Kind == yaegikernel.IntentTextureApply {
			return nil // the cell committed a real turn; the trigger consumed
		}
		// Any semantic act (ask/resolve/note/report/reply/message/…) counts as
		// an act — the cell was not idle even though it authored no revision.
		if strings.TrimSpace(in.Kind) != "" {
			stagedAct = true
		}
	}
	execCtx := toolregistry.ExecutionContextFrom(ctx)
	rec := execCtx.RunRecord
	if rt == nil || rt.textureCellAuthorizer == nil || rec == nil {
		return nil // not a texture run or no authorizer bound — nothing to do
	}
	// Only texture desk cells owe the owner-revision trigger a turn. Every
	// other desk (engineering, management, research) reduces intents against
	// the channel ledger and never touches the texture head — consuming here
	// would be a non-texture cell reaching into the doc it does not own.
	if !runHasProfile(rec, agentprofile.Texture) {
		return nil
	}
	docID := strings.TrimSpace(metadataStringValue(rec.Metadata, "doc_id"))
	if docID == "" {
		docID = strings.TrimSpace(rec.ChannelID)
	}
	if docID == "" {
		docID = strings.TrimSpace(execCtx.ChannelID)
	}
	if docID == "" {
		return nil
	}
	doc, err := rt.store.GetLifecycleDocument(ctx, rec.OwnerID, rec.ComputerID, docID)
	if err != nil {
		// A missing doc is not a cell failure — the desk ran but its doc moved
		// out from under it; the trigger either consumed or the head is gone.
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return err
	}
	if doc.CurrentRevisionID == "" {
		return nil
	}
	// decision_kind names what the cell actually did so the audit row is honest:
	// a staged act means the desk routed work off the doc (delegation_skipped);
	// no staged act means it genuinely did nothing (no_worker_needed).
	decisionKind := "no_worker_needed"
	reason := "desk cell completed with no authoring act; consuming the owner revision"
	if stagedAct {
		decisionKind = "delegation_skipped"
		reason = "desk cell committed an act off the texture doc (no revision needed); consuming the owner revision"
	}
	body, err := json.Marshal(map[string]any{
		"op":               "decide",
		"doc_id":           docID,
		"base_revision_id": doc.CurrentRevisionID,
		"decision_kind":    decisionKind,
		"reason":           reason,
	})
	if err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(rec.RunID + "\x1f" + docID + "\x1f" + doc.CurrentRevisionID))
	_, err = rt.textureCellAuthorizer.CommitCellTextureAuthor(ctx, rec, string(body), "rlm-texture-idle:"+hex.EncodeToString(sum[:8]))
	if err != nil {
		// A non-pending mutation or mismatched doc means the trigger was already
		// consumed (the cell committed a turn through another path, or the run
		// settled). That is success for our purpose — never fail a healthy cell
		// over an already-disposed wake.
		if strings.Contains(err.Error(), "not pending") || strings.Contains(err.Error(), "does not match authenticated") {
			return nil
		}
		return err
	}
	return nil
}
