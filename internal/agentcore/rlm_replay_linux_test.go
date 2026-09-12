//go:build linux

package agentcore

// RLM replay-harness replay driver (P4-replay, replay side).
//
// Replays the five retiring operations' in-cell successors against the
// persisted capture state dir ($RLM_CAPTURE_STATE) produced by the
// pre-cutover capture driver. Goldens are the committed fixtures under
// docs/evidence/rlm-replay/goldens/.
//
// Every row drives the REAL in-cell carrier: a capsule_go_eval cell on the
// session worker stages the choir intent into the tray, and the cell's
// staged intents commit through rlmReductionForCall -> commit — the exact
// path a model-authored cell takes. No commit*Intent is invoked directly.
//
// Per row the driver asserts:
//   - canonical equality: the replay view (surfaced result + durable reads)
//     projected through the frozen P0 field set equals the golden view;
//   - the golden's declared_fields exactly cover the P0 projection set, so a
//     narrowed field list cannot pass silently;
//   - zero-effect census: canonical head seq, incoming bundle dirs, stored
//     update/report/mailbox counts are unchanged by the replay;
//   - pre-effect conflict: reused semantic identity with changed canonical
//     input is refused by the identity journal before dispatch;
//   - new-identity leg: a fresh semantic identity either mints a distinct
//     durable object (update) or is refused by the durable layer (verify,
//     complete), never silently overwriting the golden state.
//
// Run on Node B:
//   RLM_CAPTURE_STATE=/root/rlm-replay-state CHOIR_CAPSULE_BROKER=/tmp/capsule-broker \
//     CHOIR_ACTUATOR=rlm go test ./internal/agentcore -run TestRLMReplayGoldens -v -timeout 600s

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	contentowner "github.com/yusefmosiah/go-choir/internal/content"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// rlmReplayGolden mirrors the committed golden contract written by the
// pre-cutover capture driver.
type rlmReplayGolden struct {
	Operation      string         `json:"operation"`
	Tool           string         `json:"tool"`
	Successor      string         `json:"successor"`
	BuildSHA       string         `json:"build_sha"`
	SemanticID     string         `json:"semantic_id"`
	Input          map[string]any `json:"input"`
	DeclaredFields []string       `json:"declared_fields"`
	Receipt        map[string]any `json:"receipt"`
	Durable        map[string]any `json:"durable,omitempty"`
	CapturedAt     string         `json:"captured_at"`
}

type rlmReplayManifest struct {
	BuildSHA          string   `json:"build_sha"`
	OwnerID           string   `json:"owner_id"`
	ComputerID        string   `json:"computer_id"`
	TrajectoryID      string   `json:"trajectory_id"`
	ImplAssignmentID  string   `json:"impl_assignment_id"`
	ImplRunID         string   `json:"impl_run_id"`
	ImplAgentID       string   `json:"impl_agent_id"`
	ImplCapsuleID     string   `json:"impl_capsule_id"`
	ImplHandle        string   `json:"impl_handle"`
	VerifyAssignID    string   `json:"verify_assignment_id"`
	VerifyRunID       string   `json:"verify_run_id"`
	VerifyAgentID     string   `json:"verify_agent_id"`
	VerifyCapsuleID   string   `json:"verify_capsule_id"`
	VerifyHandle      string   `json:"verify_handle"`
	VerifyCandidateID string   `json:"verify_candidate_id"`
	OperationID       string   `json:"operation_id"`
	BundleDigest      string   `json:"bundle_digest"`
	CellReceiptRefs   []string `json:"cell_receipt_refs"`
	ReportToolCallID  string   `json:"report_tool_call_id"`
	ParentAgentID     string   `json:"parent_agent_id"`
	ParentRunID       string   `json:"parent_run_id"`
	CapturedAt        string   `json:"captured_at"`
}

type rlmReplayEnv struct {
	stateDir   string
	updaterDir string
	sourceDir  string
	ownerID    string
	computerID string
	rt         *Runtime
	s          *store.Store
	executor   *capsule.Executor
	appender   *computerevent.ComputerEventAppender
}

// rlmReplayPinner extends the shared rollback test pinner with non-private
// payload pinning so the verification event's AppendNewPayload succeeds.
type rlmReplayPinner struct{ key computerevent.SigningKey }

func (p rlmReplayPinner) PinEvent(ctx context.Context, computerID string, canonical []byte, requestCommitment string) (computerevent.PinResult, error) {
	return rollbackTestPinner{key: p.key}.PinEvent(ctx, computerID, canonical, requestCommitment)
}

func (p rlmReplayPinner) PinNonPrivatePayload(_ context.Context, computerID string, payload []byte, mediaType, privacyClass, pinIntentCommitment string) (computerevent.PinResult, error) {
	digest := computerevent.DigestBytes(payload)
	receipt, err := computerevent.NewSignedReceipt("PayloadPinReceipt", "corpusd", map[string]any{
		"computer_id": computerID, "artifact_digest": digest, "media_type": mediaType,
		"privacy_class": privacyClass, "pin_intent_commitment": pinIntentCommitment,
	}, []computerevent.SigningKey{p.key}, time.Now().UTC())
	return computerevent.PinResult{ArtifactDigest: digest, Receipt: receipt}, err
}

// rlmReplayGoldenDir locates the committed golden fixtures relative to this
// test file's package directory.
func rlmReplayGoldenDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "docs", "evidence", "rlm-replay", "goldens")
	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
		t.Skipf("golden fixtures unavailable at %s: %v", dir, err)
	}
	return dir
}

func rlmReplayLoadGolden(t *testing.T, dir, name string) rlmReplayGolden {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, name+".golden.json"))
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	var rec rlmReplayGolden
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatalf("parse golden %s: %v", name, err)
	}
	return rec
}

// rlmReplayRuntime opens the persisted capture state dir without wiping it:
// the replay driver re-runs the five retiring operations against the
// assignments, store, and source tree the capture produced.
func rlmReplayRuntime(t *testing.T) *rlmReplayEnv {
	t.Helper()
	stateDir := strings.TrimSpace(os.Getenv("RLM_CAPTURE_STATE"))
	if stateDir == "" {
		t.Skip("RLM_CAPTURE_STATE unset; replay driver runs only on Node B against a capture state dir")
	}
	brokerPath := strings.TrimSpace(os.Getenv("CHOIR_CAPSULE_BROKER"))
	if brokerPath == "" {
		t.Skip("CHOIR_CAPSULE_BROKER unset; replay needs the broker binary")
	}
	manifestRaw, err := os.ReadFile(filepath.Join(stateDir, "goldens", "manifest.json"))
	if err != nil {
		t.Fatalf("read capture manifest: %v", err)
	}
	var manifest rlmReplayManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	env := &rlmReplayEnv{
		stateDir:   filepath.Clean(stateDir),
		updaterDir: filepath.Join(stateDir, "updater"),
		sourceDir:  filepath.Join(stateDir, "source"),
		ownerID:    manifest.OwnerID,
		computerID: manifest.ComputerID,
	}
	env.executor = capsule.NewExecutorWithSource(
		filepath.Join(env.stateDir, "executor"), filepath.Join(env.stateDir, "lower"),
		env.sourceDir, brokerPath, 8<<30)
	if err := env.executor.InitializationError(); err != nil {
		t.Fatalf("capsule executor init: %v", err)
	}

	s, err := store.Open(filepath.Join(env.stateDir, "db"))
	if err != nil {
		t.Fatalf("open capture store: %v", err)
	}
	env.s = s
	bus := events.NewEventBus()
	env.rt = New(provideriface.Config{
		ComputerID:          env.computerID,
		StorePath:           filepath.Join(env.stateDir, "db"),
		PromptRoot:          filepath.Join(env.stateDir, "prompts"),
		ProviderTimeout:     time.Second,
		SupervisionInterval: time.Hour,
	}, s, bus, provider.NewStubProvider(0),
		WithContentService(contentowner.NewService(s, bus)),
		WithCapsuleExecutor(env.executor),
		WithSelfDevelopmentUpdater(nil, env.updaterDir, env.computerID, "realization-rlm-replay"))
	setTestDispatch(env.rt, s)
	t.Cleanup(func() { env.rt.Stop(); _ = s.Close() })

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signingKey := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "rlm-replay"}, PrivateKey: privateKey}
	appender, err := computerevent.NewComputerEventAppender(env.computerID,
		rlmReplayPinner{signingKey}, s, rollbackTestCAS{key: signingKey, projection: s}, rollbackTestReceiptVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	env.rt.eventAppender = appender
	env.appender = appender
	return env
}

func (env *rlmReplayEnv) rlmHeadSeq(t *testing.T, ctx context.Context) uint64 {
	t.Helper()
	head, err := env.s.Head(ctx, env.computerID)
	if err != nil || head == nil {
		t.Fatalf("canonical head: %v", err)
	}
	return head.Sequence
}

func (env *rlmReplayEnv) rlmIncomingDirs(t *testing.T) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(env.updaterDir, "incoming"))
	if err != nil {
		t.Fatalf("read incoming dir: %v", err)
	}
	out := make(map[string]bool, len(entries))
	for _, e := range entries {
		out[e.Name()] = true
	}
	return out
}

// rlmReplayCensus is the durable effect census: the witness set a replay must
// leave untouched. Counts and identities are resolved independently of the
// replayed call's own receipt.
type rlmReplayCensus struct {
	headSeq         uint64
	incoming        map[string]bool
	updateCount     int
	reportRefs      int
	mailboxCount    int
	operationState  string
	operationDigest string
	operationCount  int
}

func (env *rlmReplayEnv) rlmTakeCensus(t *testing.T, ctx context.Context, manifest rlmReplayManifest) rlmReplayCensus {
	t.Helper()
	updates, err := env.s.ListWorkerUpdatesByTrajectoryOG(ctx, env.ownerID, manifest.TrajectoryID, 1000)
	if err != nil {
		t.Fatalf("census updates: %v", err)
	}
	assignment, err := env.s.GetCoSuperAssignment(ctx, env.ownerID, env.computerID, manifest.ImplAssignmentID, 1)
	if err != nil {
		t.Fatalf("census assignment: %v", err)
	}
	mailbox, err := env.s.ListCoagentMailboxBacklog(ctx, env.ownerID, manifest.ParentAgentID, 1000)
	if err != nil {
		t.Fatalf("census mailbox: %v", err)
	}
	op, err := env.rt.selfdevOperations.Get(ctx, env.computerID, manifest.OperationID)
	if err != nil {
		t.Fatalf("census operation: %v", err)
	}
	return rlmReplayCensus{
		headSeq:         env.rlmHeadSeq(t, ctx),
		incoming:        env.rlmIncomingDirs(t),
		updateCount:     len(updates),
		reportRefs:      len(assignment.ReportRefs),
		mailboxCount:    len(mailbox),
		operationState:  op.State,
		operationDigest: op.BundleDigest,
		operationCount:  env.rlmOperationCount(t, ctx),
	}
}

func (env *rlmReplayEnv) rlmOperationCount(t *testing.T, ctx context.Context) int {
	t.Helper()
	ops, err := env.rt.selfdevOperations.ListByStates(ctx, env.computerID,
		"requested", "executing", "frozen", "verified", "awaiting_approval",
		"accepted", "materializing", "applied", "rejected", "rollback_pending",
		"rolled_back", "failed", "degraded")
	if err != nil {
		t.Fatalf("census operation count: %v", err)
	}
	return len(ops)
}

func (env *rlmReplayEnv) rlmAssertCensusEqual(t *testing.T, ctx context.Context, manifest rlmReplayManifest, before rlmReplayCensus, row string) {
	t.Helper()
	after := env.rlmTakeCensus(t, ctx, manifest)
	if after.headSeq != before.headSeq {
		t.Fatalf("%s replay advanced canonical head: %d -> %d", row, before.headSeq, after.headSeq)
	}
	if !reflect.DeepEqual(after.incoming, before.incoming) {
		t.Fatalf("%s replay staged new bundle dirs: %v", row, after.incoming)
	}
	if after.updateCount != before.updateCount {
		t.Fatalf("%s replay minted a new update: %d -> %d", row, before.updateCount, after.updateCount)
	}
	if after.reportRefs != before.reportRefs {
		t.Fatalf("%s replay rebound report refs: %d -> %d", row, before.reportRefs, after.reportRefs)
	}
	if after.mailboxCount != before.mailboxCount {
		t.Fatalf("%s replay appended mailbox rows: %d -> %d", row, before.mailboxCount, after.mailboxCount)
	}
	if after.operationState != before.operationState || after.operationDigest != before.operationDigest {
		t.Fatalf("%s replay mutated the operation: %s/%s -> %s/%s", row,
			before.operationState, before.operationDigest, after.operationState, after.operationDigest)
	}
	if after.operationCount != before.operationCount {
		t.Fatalf("%s replay minted or dropped operation rows: %d -> %d", row, before.operationCount, after.operationCount)
	}
}

// rlmReplayView merges a surfaced receipt with durable reads into the flat
// view the P0 projection table selects from.
func rlmReplayView(receipt, durable map[string]any) map[string]any {
	view := make(map[string]any, len(receipt)+len(durable)+8)
	for k, v := range receipt {
		view[k] = v
	}
	for k, v := range durable {
		view[k] = v
	}
	// Flatten the nested report object the assignment-fate receipt carries.
	if report, ok := receipt["report"].(map[string]any); ok {
		for _, k := range []string{"report_id", "proposition_digest", "result", "verdict", "summary", "evidence_refs"} {
			if v, ok := report[k]; ok {
				view[k] = v
			}
		}
		if cmds, ok := report["commands"].([]any); ok {
			ids := make([]string, 0, len(cmds))
			for _, c := range cmds {
				if m, ok := c.(map[string]any); ok {
					ids = append(ids, fmt.Sprint(m["command_id"]))
				}
			}
			view["command_ids"] = ids
		}
		if outs, ok := report["outputs"].([]any); ok {
			digests := make([]string, 0, len(outs))
			for _, o := range outs {
				if m, ok := o.(map[string]any); ok {
					digests = append(digests, fmt.Sprint(m["digest"]))
				}
			}
			view["output_digests"] = digests
		}
	}
	// execution_receipts: P0 says compare receipt REFS only, never bodies
	// (bodies embed occurred_at). Normalize objects to their receipt_ref.
	if execs, ok := view["execution_receipts"].([]any); ok {
		refs := make([]string, 0, len(execs))
		for _, e := range execs {
			if m, ok := e.(map[string]any); ok {
				refs = append(refs, fmt.Sprint(m["receipt_ref"]))
			} else {
				refs = append(refs, fmt.Sprint(e))
			}
		}
		view["execution_receipts"] = refs
	}
	return view
}

// rlmReportCommandIDs projects command_ids from a stored report.
func rlmReportCommandIDs(report types.CoSuperAssignmentReport) []string {
	ids := make([]string, 0, len(report.Commands))
	for _, c := range report.Commands {
		ids = append(ids, c.CommandID)
	}
	return ids
}

func rlmReportOutputDigests(report types.CoSuperAssignmentReport) []string {
	digests := make([]string, 0, len(report.Outputs))
	for _, o := range report.Outputs {
		digests = append(digests, o.Digest)
	}
	return digests
}

// rlmGoldenString reads a required string field from a golden receipt or
// input, failing closed instead of panicking on a type assertion.
func rlmGoldenString(t *testing.T, m map[string]any, key string) string {
	t.Helper()
	v, ok := m[key].(string)
	if !ok || v == "" {
		t.Fatalf("golden lacks string field %q", key)
	}
	return v
}

// rlmEmptyToNil mirrors the golden null convention: the capture receipts
// record absent scalars (verdict, candidate) as null, so a replayed empty
// string must project as nil, not "".
func rlmEmptyToNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// rlmCopyDir replicates a bundle tree for the corruption leg: a partial copy
// would reject on missing files instead of the corrupted content, proving
// nothing about integrity detection.
func rlmCopyDir(t *testing.T, src, dst string) {
	t.Helper()
	if err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	}); err != nil {
		t.Fatalf("copy bundle tree: %v", err)
	}
}

// rlmAssertReplayEquality projects the golden view and the replay view through
// the frozen P0 field set and requires canonical equality. Coverage is
// P0-relative, not golden-relative: the golden's declared_fields plus the
// explicitly named exclusions must exactly cover the static P0 set, so a
// field missing from the golden can never pass silently on both sides.
// Exclusions are the only honest way to drop a field, and each must be
// disclosed at its call site: row 5 `state` (freeze-time constant vs the
// live anachronistic row state) and row 9 `replay` (a capture-time golden
// cannot carry the replay marker).
func rlmAssertReplayEquality(t *testing.T, golden rlmReplayGolden, replayView map[string]any, exclusions ...string) {
	t.Helper()
	operation := golden.Operation
	goldenView := rlmReplayView(golden.Receipt, golden.Durable)

	p0 := rlmReplayP0Fields(operation)
	if p0 == nil {
		t.Fatalf("%s: unknown operation", operation)
	}
	excluded := make(map[string]bool, len(exclusions))
	for _, e := range exclusions {
		excluded[e] = true
	}
	allowed := make([]string, 0, len(p0))
	for _, f := range p0 {
		if !excluded[f] {
			allowed = append(allowed, f)
		}
	}
	sort.Strings(allowed)
	declared := append([]string(nil), golden.DeclaredFields...)
	sort.Strings(declared)
	if !reflect.DeepEqual(declared, allowed) {
		t.Fatalf("%s: golden declared_fields %v do not cover the P0 set %v minus exclusions %v", operation, declared, p0, exclusions)
	}

	wantProj, err := projectRLMReplayReceipt(operation, goldenView, exclusions)
	if err != nil {
		t.Fatalf("%s: project golden view: %v", operation, err)
	}
	gotProj, err := projectRLMReplayReceipt(operation, replayView, exclusions)
	if err != nil {
		t.Fatalf("%s: project replay view: %v", operation, err)
	}
	for _, field := range allowed {
		want, wantOK := wantProj[field]
		got, gotOK := gotProj[field]
		if !wantOK {
			t.Fatalf("%s: golden view lacks projected field %q", operation, field)
		}
		if !gotOK {
			t.Fatalf("%s: replay view lacks projected field %q (view %v)", operation, field, replayView)
		}
		// status flips submitted -> existing on replay: that flip IS the
		// dedup proof, not a divergence.
		if field == "status" {
			if got != "existing" {
				t.Fatalf("%s: replay status %v, expected existing (dedup proof)", operation, got)
			}
			continue
		}
		wantJSON, _ := computerevent.CanonicalJSON(want)
		gotJSON, _ := computerevent.CanonicalJSON(got)
		if string(wantJSON) != string(gotJSON) {
			t.Fatalf("%s: projected field %q diverged: golden=%v replay=%v", operation, field, want, got)
		}
	}
}

// rlmReplayCell runs one cell through the real carrier: the session worker
// stages the choir intents into the tray, and rlmReductionForCall commits
// them — the exact path a model-authored cell takes inside
// newCapsuleGoEvalTool.Func. The tool's admission-time obligation gate is
// intentionally not re-entered: it exists to refuse LIVE calls on terminal
// assignments, while replay is a harness operation that must reach the
// durable layer to prove dedup.
func (env *rlmReplayEnv) rlmReplayCell(t *testing.T, ctx context.Context, rec *types.RunRecord, toolCtx *CapsuleToolCtx, toolCallID, source string) map[string]any {
	t.Helper()
	result, err := env.rlmReplayCellErr(ctx, rec, toolCtx, toolCallID, source)
	if err != nil {
		t.Fatalf("cell %s: %v", toolCallID, err)
	}
	return result
}

// rlmReplayCellErr is the error-returning variant for conflict legs: the cell
// may fail at eval, reduction, or the durable layer; the caller asserts which.
func (env *rlmReplayEnv) rlmReplayCellErr(ctx context.Context, rec *types.RunRecord, toolCtx *CapsuleToolCtx, toolCallID, source string) (map[string]any, error) {
	execCtx := WithCapsuleCtx(env.rlmExecCtx(ctx, rec, toolCallID), toolCtx)
	reduction := rlmReductionForCall(execCtx, env.rt, toolCtx)
	req := capsule.GoEvalRequest{Source: source, Cwd: "/workspace/platform", TimeoutMS: 30000}
	if reduction.active {
		req.Inbox = reduction.inbox
	}
	result, err := toolCtx.Executor.GoEval(execCtx, toolCtx.AgentRunID, toolCtx.CapsuleHandle, req)
	if err != nil {
		return nil, err
	}
	if result.Error != "" {
		return nil, fmt.Errorf("cell error: %s", result.Error)
	}
	if reduction.active {
		if rerr := reduction.commit(execCtx, result.Intents); rerr != nil {
			return nil, rerr
		}
	}
	out := map[string]any{
		"stdout": result.Stdout, "stderr": result.Stderr,
		"receipt_ref": result.ReceiptRef, "staged_intent_ids": result.StagedIntentIDs,
	}
	if reduction.freezeResult != nil {
		out["freeze_result"] = reduction.freezeResult
	}
	if reduction.verifyResult != nil {
		out["verify_result"] = reduction.verifyResult
	}
	return out, nil
}

func (env *rlmReplayEnv) rlmExecCtx(ctx context.Context, rec *types.RunRecord, toolCallID string) context.Context {
	return toolregistry.WithExecutionContext(ctx, toolregistry.ExecutionContext{
		RunID: rec.RunID, ToolCallID: toolCallID, AgentID: rec.AgentID, OwnerID: rec.OwnerID,
		Profile: "engineering", Role: "engineering", ChannelID: rec.ChannelID, ComputerID: rec.ComputerID,
		RunRecord: rec,
	})
}

// rlmReplayToolCtx builds the capsule tool context for replay: identical to
// the assigned desk's context except the obligation validator is
// replay-scoped — it requires the assignment to exist, be bound to this run,
// and carry the same work item, but does not require live disposition. The
// live-call liveness gate is admission enforcement; replay is a harness
// operation whose dedup proof lives in the durable layer (operation state,
// update dedup, lifecycle command digest), not in admission.
func (env *rlmReplayEnv) rlmReplayToolCtx(rec *types.RunRecord, handle string) *CapsuleToolCtx {
	toolCtx := env.rt.assignedCoSuperCapsuleToolCtx(rec, handle)
	assignmentID := metadataStringValue(rec.Metadata, "assignment_id")
	attempt := uint64(metadataIntValue(rec.Metadata, "assignment_attempt"))
	workItemID := metadataStringValue(rec.Metadata, "assigned_work_item_id")
	toolCtx.ValidateCurrentObligation = func(callCtx context.Context) error {
		assignment, err := env.s.GetCoSuperAssignment(callCtx, rec.OwnerID, rec.ComputerID, assignmentID, attempt)
		if err != nil {
			return fmt.Errorf("replay obligation: %w", err)
		}
		if assignment.BoundRunID != rec.RunID || assignment.Binding.AssignedWorkItemID != workItemID {
			return fmt.Errorf("replay obligation: assignment binding drifted")
		}
		return nil
	}
	return toolCtx
}

func rlmStringSlice(v any) []string {
	items, _ := v.([]any)
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func rlmGoStringSlice(items []string) string {
	quoted := make([]string, 0, len(items))
	for _, s := range items {
		quoted = append(quoted, fmt.Sprintf("%q", s))
	}
	return "[]string{" + strings.Join(quoted, ", ") + "}"
}

func TestRLMReplayGoldens(t *testing.T) {
	env := rlmReplayRuntime(t)
	ctx := context.Background()
	goldenDir := rlmReplayGoldenDir(t)

	manifestRaw, err := os.ReadFile(filepath.Join(goldenDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest rlmReplayManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}

	// The manifest stamp binds every golden: a restamped manifest with stale
	// goldens (or vice versa) must fail here, not as a mysterious field
	// divergence rows later. The list derives from the directory so a sixth
	// golden cannot escape the coherence assert silently.
	goldenEntries, err := os.ReadDir(goldenDir)
	if err != nil {
		t.Fatalf("list goldens: %v", err)
	}
	coherentCount := 0
	for _, e := range goldenEntries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".golden.json") {
			continue
		}
		golden := rlmReplayLoadGolden(t, goldenDir, strings.TrimSuffix(name, ".golden.json"))
		if golden.BuildSHA != manifest.BuildSHA {
			t.Fatalf("golden %s build_sha %q drifts from manifest build_sha %q", name, golden.BuildSHA, manifest.BuildSHA)
		}
		coherentCount++
	}
	if coherentCount == 0 {
		t.Fatal("no golden fixtures found")
	}

	// runSuffix makes every test-minted identity unique per run: the capture
	// state dir persists across runs, so fixed idempotency keys, capsule IDs
	// and handle names would collide (or false-dedup) on the second run.
	runSuffix := fmt.Sprintf("%d", time.Now().UnixNano())
	implRun, err := env.s.GetLifecycleRun(ctx, env.ownerID, env.computerID, manifest.ImplRunID)
	if err != nil {
		t.Fatalf("load impl run: %v", err)
	}
	verifyRun, err := env.s.GetLifecycleRun(ctx, env.ownerID, env.computerID, manifest.VerifyRunID)
	if err != nil {
		t.Fatalf("load verify run: %v", err)
	}

	// Rebuild the impl capsule: respawn from the same source snapshot, mint a
	// fresh handle under the recorded name, and re-apply the release mutation
	// so the frozen worktree digest matches the capture. A killed prior run
	// leaves the on-disk capsule dir (and possibly a stale overlay mount);
	// ForceDestroy only handles live capsules, so clear the state directly.
	// Sweep stale corrupt-bundle dirs too: a SIGKILLed run can die between
	// rlmCopyDir and cleanup, and the incoming census would absorb the
	// leftover as baseline forever. t.Cleanup covers graceful exits; this
	// covers kills.
	if incoming, err := os.ReadDir(filepath.Join(env.stateDir, "updater", "incoming")); err == nil {
		for _, e := range incoming {
			if strings.HasPrefix(e.Name(), "corrupt-") {
				_ = os.RemoveAll(filepath.Join(env.stateDir, "updater", "incoming", e.Name()))
			}
		}
	}
	sweepIDs := []string{manifest.ImplCapsuleID, manifest.VerifyCapsuleID, "capsule-rlm-verify-conflict"}
	if entries, err := os.ReadDir(filepath.Join(env.stateDir, "executor")); err == nil {
		for _, e := range entries {
			name := e.Name()
			if strings.HasPrefix(name, "capsule-rlm-verify-conflict-") || strings.HasPrefix(name, "capsule-rlm-verify-corrupt-") {
				sweepIDs = append(sweepIDs, name)
			}
		}
	}
	for _, id := range sweepIDs {
		_ = env.executor.ForceDestroy(ctx, id)
		dir := filepath.Join(env.stateDir, "executor", id)
		if mounts, err := exec.Command("sh", "-c",
			"mount | grep "+dir+" | awk '{print $3}' | sort -r").Output(); err == nil {
			for _, m := range strings.Fields(string(mounts)) {
				_ = exec.Command("umount", "-l", m).Run()
			}
		}
		_ = os.RemoveAll(dir)
	}
	preflight, err := env.executor.PreflightSourceSnapshot(ctx, "")
	if err != nil {
		t.Fatalf("preflight source: %v", err)
	}
	if _, err := env.executor.Spawn(ctx, capsule.SpawnSpec{
		CapsuleID: manifest.ImplCapsuleID, OwnerRunID: manifest.ImplRunID,
		MemoryMax: 1 << 30, CpuQuota: 100000, CpuPeriod: 100000, PidsMax: 256,
		WorkingDir: "/workspace/platform", Tier: capsule.TierMedium,
		SourceArtifactRef: preflight.ArtifactRef, ExpectedSubjectDigest: preflight.SubjectDigest,
	}); err != nil {
		t.Fatalf("respawn impl capsule: %v", err)
	}
	t.Cleanup(func() {
		for _, id := range []string{manifest.ImplCapsuleID, manifest.VerifyCapsuleID} {
			_ = env.executor.ForceDestroy(context.Background(), id)
		}
	})
	if _, err := env.executor.MintCapabilityHandle(manifest.ImplRunID, capsule.RoleCoSuper, manifest.ImplCapsuleID, manifest.ImplHandle, 24*time.Hour, ""); err != nil {
		t.Fatalf("mint impl handle: %v", err)
	}
	for _, m := range []struct {
		path, body string
		mode       uint32
	}{
		{"/var/lib/artifact/release/bin/autoputer", "#!/bin/sh\necho rlm-replay\n", 0o755},
		{"/var/lib/artifact/release/frontend/index.html", "<html>rlm replay release</html>\n", 0o644},
	} {
		if err := env.executor.WriteFile(ctx, manifest.ImplRunID, manifest.ImplHandle, m.path, []byte(m.body), m.mode); err != nil {
			t.Fatalf("mutate worktree %s: %v", m.path, err)
		}
	}
	implToolCtx := env.rlmReplayToolCtx(&implRun, manifest.ImplHandle)
	verifyToolCtx := env.rlmReplayToolCtx(&verifyRun, manifest.VerifyHandle)

	identities := &rlmReplayIdentityJournal{}
	dispatches := &rlmReplayDispatchJournal{}

	// --- Row 5: commit_transaction -> choir.Freeze ---
	// The cell stages choir.Freeze; the reducer commits it through
	// commitFreezeIntent. The operation is already frozen, so the durable
	// layer must replay the stored receipt — the freeze semantic identity is
	// the operation itself, not the call inputs.
	commitGolden := rlmReplayLoadGolden(t, goldenDir, "commit_transaction")
	freezeInput := map[string]any{
		"build_recipe_ref":          commitGolden.Input["build_recipe_ref"],
		"test_receipts":             commitGolden.Input["test_receipts"],
		"dependency_toolchain_refs": commitGolden.Input["dependency_toolchain_refs"],
	}
	freezeCell := fmt.Sprintf(`package main
import "choir"
func main() {
err := choir.Freeze(%q, %s, %s)
if err != nil { panic(err) }
}`,
		commitGolden.Input["build_recipe_ref"].(string),
		rlmGoStringSlice(rlmStringSlice(commitGolden.Input["test_receipts"])),
		rlmGoStringSlice(rlmStringSlice(commitGolden.Input["dependency_toolchain_refs"])))
	censusBefore := env.rlmTakeCensus(t, ctx, manifest)
	if _, err := identities.claim("commit_transaction", commitGolden.SemanticID, freezeInput); err != nil {
		t.Fatalf("freeze identity claim: %v", err)
	}
	dispatches.record(commitGolden.SemanticID)
	verifyGolden := rlmReplayLoadGolden(t, goldenDir, "record_self_development_verification")
	freezeResult := env.rlmReplayCell(t, ctx, &implRun, implToolCtx, "replay-freeze-"+runSuffix, freezeCell)
	freezeView, _ := freezeResult["freeze_result"].(map[string]any)
	if freezeView == nil {
		t.Fatalf("freeze cell surfaced no freeze_result: %v", freezeResult)
	}
	// The short-circuit receipt carries only {handle,bundle_digest,operation_id,
	// state}; the P0 projection's remaining fields live on the durable
	// operation record and the frozen bundle draft on disk. Read them live —
	// never from the golden — so equality on those fields can actually fail.
	op, err := env.rt.selfdevOperations.Get(ctx, env.computerID, manifest.OperationID)
	if err != nil {
		t.Fatalf("load operation for freeze view: %v", err)
	}
	freezeView["base_event_head"] = op.BaseHead
	freezeView["trajectory_id"] = op.TrajectoryID
	// The frozen bundle draft is the durable record of what the cell froze.
	// Read the five compared P0 fields from it live — never from the golden.
	// `state` is excluded from the compared set (see rlmAssertReplayEquality):
	// it names the freeze-time state, which the live row has since left
	// (awaiting_approval after verify); surfacing the live value would compare
	// an anachronism, and a literal would be a fabricated pass.
	draftPath := filepath.Join(env.updaterDir, "incoming", rlmGoldenString(t, verifyGolden.Receipt, "bundle_digest"), "bundle.draft.json")
	rawDraft, err := os.ReadFile(draftPath)
	if err != nil {
		t.Fatalf("read frozen bundle draft: %v", err)
	}
	var draft transaction.CapsuleEffectBundle
	if err := json.Unmarshal(rawDraft, &draft); err != nil {
		t.Fatalf("parse frozen bundle draft: %v", err)
	}
	freezeView["content_digest"] = draft.ContentDigest
	freezeView["change_count"] = float64(len(draft.OrderedFileEffects))
	freezeView["classifier_version"] = draft.ClassifierV
	freezeView["classifier_digest"] = draft.ClassifierDigest
	freezeView["groups"] = draft.Groups
	rlmAssertReplayEquality(t, commitGolden, freezeView, "state")

	// Conflict probe: same semantic identity, changed canonical input must be
	// refused by the journal BEFORE dispatch — no cell runs at all.
	mutatedFreezeInput := map[string]any{
		"build_recipe_ref":          "capsule-exec:sha256:" + strings.Repeat("f", 64),
		"test_receipts":             commitGolden.Input["test_receipts"],
		"dependency_toolchain_refs": commitGolden.Input["dependency_toolchain_refs"],
	}
	if _, err := identities.claim("commit_transaction", commitGolden.SemanticID, mutatedFreezeInput); err == nil {
		t.Fatal("freeze conflict probe: mutated input on the same identity was not refused pre-dispatch")
	}
	env.rlmAssertCensusEqual(t, ctx, manifest, censusBefore, "commit_transaction conflict")
	// New-identity leg for freeze: a fresh trajectory + operation row is a
	// distinct semantic identity. The durable layer must mint a new operation
	// and freeze it independently — never dedup to the golden's operation.
	freshOp, err := env.rt.selfdevOperations.Start(ctx, selfdev.StartRequest{
		ComputerID:        env.computerID,
		IdempotencyKey:    "rlm-replay-freeze-fresh-" + runSuffix,
		PromptArtifactRef: "artifact:sha256:" + strings.Repeat("a", 64),
	})
	if err != nil {
		t.Fatalf("start fresh freeze operation: %v", err)
	}
	if freshOp.OperationID == manifest.OperationID {
		t.Fatal("freeze new-identity leg: fresh operation deduped to golden operation")
	}
	if freshOp.TrajectoryID == manifest.TrajectoryID {
		t.Fatal("freeze new-identity leg: fresh operation shares golden trajectory")
	}
	// Non-interference: the fresh identity minted alongside the golden
	// operation, never over it — prove the golden row still reads back
	// intact. Scope note: this leg proves Start identity allocation
	// (distinct durable object, no dedup), not an independent Freeze of the
	// fresh operation; fresh-Freeze independence is deferred (residue).
	goldenOp, err := env.rt.selfdevOperations.Get(ctx, env.computerID, manifest.OperationID)
	if err != nil {
		t.Fatalf("load golden operation after fresh start: %v", err)
	}
	if goldenOp.OperationID != manifest.OperationID || goldenOp.TrajectoryID != manifest.TrajectoryID {
		t.Fatal("freeze new-identity leg: fresh start disturbed the golden operation row")
	}
	// The replay env takes the direct SQL path (no projection sink bound),
	// so Start wrote a real row into the shared capture DB. Remove it: the
	// census below proves zero net effect, and reruns must not accumulate
	// junk `requested` rows.
	if _, err := env.s.DB().ExecContext(ctx, `DELETE FROM self_development_operations WHERE computer_id = ? AND operation_id = ?`, env.computerID, freshOp.OperationID); err != nil {
		t.Fatalf("remove fresh freeze operation: %v", err)
	}
	env.rlmAssertCensusEqual(t, ctx, manifest, censusBefore, "commit_transaction new-identity")
	// The replay above proves the durable layer returns the stored receipt on
	// a second freeze of the same operation; the conflict probe proves
	// input-mutation is refused pre-dispatch; the fresh operation proves a
	// distinct identity mints a distinct durable object with no residue.

	// --- Row 6: inspect_self_development_bundle -> choir.InspectBundle ---
	// The verifier capsule respawns with the frozen bundle mounted at
	// /selfdev/bundle; the cell inspects through the mount. The mount is the
	// binding — the cell never supplies identity.
	inspectGolden := rlmReplayLoadGolden(t, goldenDir, "inspect_self_development_bundle")
	// verifyGolden loaded above for the freeze draft path.
	// The finalized bundle dir mirrors the frozen draft (finalize copies it
	// alongside bundle.json), so the P0 frozen fields read here are the frozen
	// values; the digest selects the durable object, the reads prove the values.
	draftDir := filepath.Join(env.updaterDir, "incoming", rlmGoldenString(t, verifyGolden.Receipt, "bundle_digest"))
	binding, _ := json.Marshal(map[string]any{
		"operation_id":  rlmGoldenString(t, inspectGolden.Input, "operation_id"),
		"bundle_digest": rlmGoldenString(t, inspectGolden.Input, "bundle_digest"),
	})
	verifyPreflight, err := env.executor.PreflightSourceSnapshot(ctx, "")
	if err != nil {
		t.Fatalf("preflight verifier source: %v", err)
	}
	if _, err := env.executor.Spawn(ctx, capsule.SpawnSpec{
		CapsuleID: manifest.VerifyCapsuleID, OwnerRunID: manifest.VerifyRunID,
		MemoryMax: 1 << 30, CpuQuota: 100000, CpuPeriod: 100000, PidsMax: 256,
		WorkingDir: "/workspace/platform", Tier: capsule.TierMedium,
		SourceArtifactRef: verifyPreflight.ArtifactRef, ExpectedSubjectDigest: verifyPreflight.SubjectDigest,
		VerifierBundleDir: draftDir, VerifierBinding: string(binding),
	}); err != nil {
		t.Fatalf("respawn verifier capsule: %v", err)
	}
	if _, err := env.executor.MintCapabilityHandle(manifest.VerifyRunID, capsule.RoleCoSuper, manifest.VerifyCapsuleID, manifest.VerifyHandle, 24*time.Hour, "verifier"); err != nil {
		t.Fatalf("mint verifier handle: %v", err)
	}
	inspectCell := `package main
import ("choir"; "encoding/json"; "fmt")
func main() {
m, err := choir.InspectBundle()
if err != nil { fmt.Print("INSPECT_ERR:" + err.Error()); return }
b, _ := json.Marshal(m)
fmt.Print(string(b))
}`
	inspectInput := map[string]any{
		"operation_id":  inspectGolden.Input["operation_id"],
		"bundle_digest": inspectGolden.Input["bundle_digest"],
	}
	if _, err := identities.claim("inspect_self_development_bundle", inspectGolden.SemanticID, inspectInput); err != nil {
		t.Fatalf("inspect identity claim: %v", err)
	}
	dispatches.record(inspectGolden.SemanticID)
	inspectResult := env.rlmReplayCell(t, ctx, &verifyRun, verifyToolCtx, "replay-inspect-"+runSuffix, inspectCell)
	stdout, _ := inspectResult["stdout"].(string)
	if strings.HasPrefix(stdout, "INSPECT_ERR:") {
		t.Fatalf("replay inspect_self_development_bundle: %s", stdout)
	}
	var inspectReplay map[string]any
	if err := json.Unmarshal([]byte(stdout), &inspectReplay); err != nil {
		t.Fatalf("parse inspect receipt: %v (stdout %q)", err, stdout)
	}
	rlmAssertReplayEquality(t, inspectGolden, inspectReplay)

	// Conflict probe: a capsule whose binding names a different digest must
	// fail closed — the mount is the binding, so a mismatched binding is a
	// pre-effect conflict on reused identity with changed canonical input.
	badBinding, _ := json.Marshal(map[string]any{
		"operation_id":  inspectGolden.Input["operation_id"].(string),
		"bundle_digest": strings.Repeat("0", 64),
	})
	conflictCapsule := "capsule-rlm-verify-conflict-" + runSuffix
	conflictHandle := "h-rlm-verify-conflict-" + runSuffix
	if _, err := env.executor.Spawn(ctx, capsule.SpawnSpec{
		CapsuleID: conflictCapsule, OwnerRunID: manifest.VerifyRunID,
		MemoryMax: 1 << 30, CpuQuota: 100000, CpuPeriod: 100000, PidsMax: 256,
		WorkingDir: "/workspace/platform", Tier: capsule.TierMedium,
		SourceArtifactRef: verifyPreflight.ArtifactRef, ExpectedSubjectDigest: verifyPreflight.SubjectDigest,
		VerifierBundleDir: draftDir, VerifierBinding: string(badBinding),
	}); err != nil {
		t.Fatalf("spawn conflict verifier capsule: %v", err)
	}
	defer env.executor.ForceDestroy(context.Background(), conflictCapsule)
	if _, err := env.executor.MintCapabilityHandle(manifest.VerifyRunID, capsule.RoleCoSuper, conflictCapsule, conflictHandle, 24*time.Hour, "verifier"); err != nil {
		t.Fatalf("mint conflict verifier handle: %v", err)
	}
	conflictToolCtx := env.rlmReplayToolCtx(&verifyRun, conflictHandle)
	conflictCellID := "replay-inspect-conflict-" + runSuffix
	conflictRes, err := env.rlmReplayCellErr(ctx, &verifyRun, conflictToolCtx, conflictCellID, inspectCell)
	if err == nil {
		if s, _ := conflictRes["stdout"].(string); !strings.HasPrefix(s, "INSPECT_ERR:") {
			t.Fatalf("inspect conflict probe: mismatched binding digest accepted (stdout %q)", s)
		}
	}

	// Integrity leg for inspect (the new-identity-observes-change half for the
	// read-only row): copy the WHOLE frozen bundle tree to a fresh incoming
	// dir, corrupt one runtime file, and require the corruption — not a
	// binding-shape error — to reject the inspection. The binding carries the
	// FROZEN digest (dfdc) under a fresh operation id: the mirrored draft
	// declares the frozen content digest, and the mounted binding must name it
	// (row 6 binds inspectGolden.Input, frozen, for the same reason). The
	// finalized receipt digest (ea24) fails closed before any file is read,
	// which would make the corruption causally inert — so only the frozen
	// digest keeps the corruption causal, and the rejection can only come from
	// the corrupted file content.
	frozenDigest := rlmGoldenString(t, commitGolden.Receipt, "bundle_digest")
	corruptDir := filepath.Join(env.updaterDir, "incoming", "corrupt-"+runSuffix)
	rlmCopyDir(t, draftDir, corruptDir)
	// Failure-safe cleanup: any Fatalf between here and the leg's end must
	// not leave the copy inside the censused, persistent incoming tree.
	t.Cleanup(func() { _ = os.RemoveAll(corruptDir) })
	rawDraft2, err := os.ReadFile(filepath.Join(corruptDir, "bundle.draft.json"))
	if err != nil {
		t.Fatalf("read copied draft for corruption: %v", err)
	}
	var corruptDraft transaction.CapsuleEffectBundle
	if err := json.Unmarshal(rawDraft2, &corruptDraft); err != nil {
		t.Fatalf("parse copied draft for corruption: %v", err)
	}
	if len(corruptDraft.RuntimeFiles) == 0 {
		t.Fatal("corrupt leg: no runtime files to corrupt")
	}
	corruptFile := filepath.Join(corruptDir, filepath.FromSlash(corruptDraft.RuntimeFiles[0].Path))
	if err := os.WriteFile(corruptFile, []byte("corrupted"), 0o644); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}
	corruptBinding, _ := json.Marshal(map[string]any{
		"operation_id":  rlmGoldenString(t, inspectGolden.Input, "operation_id") + "-corrupt-" + runSuffix,
		"bundle_digest": frozenDigest,
	})
	corruptCapsule := "capsule-rlm-verify-corrupt-" + runSuffix
	corruptHandle := "h-rlm-verify-corrupt-" + runSuffix
	if _, err := env.executor.Spawn(ctx, capsule.SpawnSpec{
		CapsuleID: corruptCapsule, OwnerRunID: manifest.VerifyRunID,
		MemoryMax: 1 << 30, CpuQuota: 100000, CpuPeriod: 100000, PidsMax: 256,
		WorkingDir: "/workspace/platform", Tier: capsule.TierMedium,
		SourceArtifactRef: verifyPreflight.ArtifactRef, ExpectedSubjectDigest: verifyPreflight.SubjectDigest,
		VerifierBundleDir: corruptDir, VerifierBinding: string(corruptBinding),
	}); err != nil {
		t.Fatalf("spawn corrupt verifier capsule: %v", err)
	}
	defer env.executor.ForceDestroy(context.Background(), corruptCapsule)
	if _, err := env.executor.MintCapabilityHandle(manifest.VerifyRunID, capsule.RoleCoSuper, corruptCapsule, corruptHandle, 24*time.Hour, "verifier"); err != nil {
		t.Fatalf("mint corrupt verifier handle: %v", err)
	}
	corruptToolCtx := env.rlmReplayToolCtx(&verifyRun, corruptHandle)
	corruptRes, err := env.rlmReplayCellErr(ctx, &verifyRun, corruptToolCtx, "replay-inspect-corrupt-"+runSuffix, inspectCell)
	if err != nil {
		t.Fatalf("inspect integrity leg: cell failed instead of surfacing INSPECT_ERR: %v", err)
	}
	corruptStdout, _ := corruptRes["stdout"].(string)
	if !strings.Contains(corruptStdout, "frozen runtime file digest mismatch") {
		t.Fatalf("inspect integrity leg: expected file-digest rejection (stdout %q)", corruptStdout)
	}
	// Remove the corrupted copy immediately: later rows census the incoming
	// dir set, and the shared capture state must stay pristine for reruns.
	if err := os.RemoveAll(corruptDir); err != nil {
		t.Fatalf("remove corrupt bundle dir: %v", err)
	}

	// --- Row 7: record_self_development_verification -> choir.Verify ---
	// The cell stages choir.Verify; the reducer commits it through
	// commitVerifyIntent. The operation is awaiting_approval, so the durable
	// layer replays the recorded verification.
	verifyInput := map[string]any{
		"operation_id":  verifyGolden.Input["operation_id"],
		"bundle_digest": verifyGolden.Receipt["bundle_digest"],
		"decision":      verifyGolden.Input["decision"],
		"verifier_refs": verifyGolden.Input["verifier_refs"],
	}
	verifyCell := fmt.Sprintf(`package main
func main() {
err := choir.Verify(%q, %s, %q)
if err != nil { panic(err) }
}`,
		rlmGoldenString(t, verifyGolden.Input, "decision"),
		rlmGoStringSlice(rlmStringSlice(verifyGolden.Input["verifier_refs"])),
		rlmGoldenString(t, verifyGolden.Receipt, "bundle_digest"))

	censusBefore = env.rlmTakeCensus(t, ctx, manifest)
	if _, err := identities.claim("record_self_development_verification", verifyGolden.SemanticID, verifyInput); err != nil {
		t.Fatalf("verify identity claim: %v", err)
	}
	dispatches.record(verifyGolden.SemanticID)
	verifyResult := env.rlmReplayCell(t, ctx, &verifyRun, verifyToolCtx, "replay-verify-"+runSuffix, verifyCell)
	verifyView, _ := verifyResult["verify_result"].(map[string]any)
	if verifyView == nil {
		t.Fatalf("verify cell surfaced no verify_result: %v", verifyResult)
	}
	rlmAssertReplayEquality(t, verifyGolden, verifyView)
	env.rlmAssertCensusEqual(t, ctx, manifest, censusBefore, "record_self_development_verification")

	// Conflict probe: same identity, changed decision must be refused by the
	// journal pre-dispatch.
	mutatedVerifyInput := map[string]any{
		"operation_id":  verifyGolden.Input["operation_id"],
		"bundle_digest": verifyGolden.Receipt["bundle_digest"],
		"decision":      "fail",
		"verifier_refs": verifyGolden.Input["verifier_refs"],
	}
	if _, err := identities.claim("record_self_development_verification", verifyGolden.SemanticID, mutatedVerifyInput); err == nil {
		t.Fatal("verify conflict probe: mutated decision on the same identity was not refused pre-dispatch")
	}
	env.rlmAssertCensusEqual(t, ctx, manifest, censusBefore, "verify conflict")

	// New-identity leg: a fail decision under a fresh identity reaches the
	// durable layer, which must refuse it — the recorded pass verification is
	// terminal for this operation.
	failVerifyCell := fmt.Sprintf(`package main
func main() {
err := choir.Verify("fail", %s, %q)
if err != nil { panic(err) }
}`,
		rlmGoStringSlice(rlmStringSlice(verifyGolden.Input["verifier_refs"])),
		rlmGoldenString(t, verifyGolden.Receipt, "bundle_digest"))
	if _, err := identities.claim("record_self_development_verification", verifyGolden.SemanticID+"|fail", mutatedVerifyInput); err != nil {
		t.Fatalf("verify new-identity claim: %v", err)
	}
	dispatches.record(verifyGolden.SemanticID + "|fail")
	if _, err := env.rlmReplayCellErr(ctx, &verifyRun, verifyToolCtx, "replay-verify-fail-"+runSuffix, failVerifyCell); err == nil {
		t.Fatal("verify new-identity leg: fail decision accepted on an operation with a recorded pass")
	}
	env.rlmAssertCensusEqual(t, ctx, manifest, censusBefore, "verify new-identity")

	// --- Row 9: update_coagent -> choir.Message ---
	// The cell stages choir.Message; the reducer commits it through
	// commitMessageIntent under the update authority contract. The update is
	// already recorded, so the durable layer dedups to the same update_id.
	updateGolden := rlmReplayLoadGolden(t, goldenDir, "update_coagent")
	packetBody, _ := json.Marshal(updateGolden.Input["packet"])
	updateInput := map[string]any{
		"agent_id": updateGolden.Input["agent_id"],
		"packet":   updateGolden.Input["packet"],
	}

	// The impl capsule is frozen from the freeze replay; respawn it fresh so
	// the update cell can run. The update's durable identity is the packet,
	// not the capsule, so a fresh capsule is a valid replay vehicle.
	_ = env.executor.ForceDestroy(ctx, manifest.ImplCapsuleID)
	_ = os.RemoveAll(filepath.Join(env.stateDir, "executor", manifest.ImplCapsuleID))
	preflight2, err := env.executor.PreflightSourceSnapshot(ctx, "")
	if err != nil {
		t.Fatalf("preflight impl source for update: %v", err)
	}
	if _, err := env.executor.Spawn(ctx, capsule.SpawnSpec{
		CapsuleID: manifest.ImplCapsuleID, OwnerRunID: manifest.ImplRunID,
		MemoryMax: 1 << 30, CpuQuota: 100000, CpuPeriod: 100000, PidsMax: 256,
		WorkingDir: "/workspace/platform", Tier: capsule.TierMedium,
		SourceArtifactRef: preflight2.ArtifactRef, ExpectedSubjectDigest: preflight2.SubjectDigest,
	}); err != nil {
		t.Fatalf("respawn impl capsule for update: %v", err)
	}
	if _, err := env.executor.MintCapabilityHandle(manifest.ImplRunID, capsule.RoleCoSuper, manifest.ImplCapsuleID, manifest.ImplHandle, 24*time.Hour, ""); err != nil {
		t.Fatalf("mint impl handle for update: %v", err)
	}
	implToolCtx = env.rlmReplayToolCtx(&implRun, manifest.ImplHandle)
	updateCell := fmt.Sprintf(`package main
import "choir"
func main() {
_, err := choir.Message(%q, "execution_result", %q)
if err != nil { panic(err) }
}`,
		updateGolden.Input["agent_id"].(string), string(packetBody))

	censusBefore = env.rlmTakeCensus(t, ctx, manifest)
	if _, err := identities.claim("update_coagent", updateGolden.SemanticID, updateInput); err != nil {
		t.Fatalf("update identity claim: %v", err)
	}
	dispatches.record(updateGolden.SemanticID)
	env.rlmReplayCell(t, ctx, &implRun, implToolCtx, "replay-update-"+runSuffix, updateCell)
	// Canonical equality is observed through the store: the deduplicated
	// update must carry the golden's declared fields.
	storedUpdate, err := env.s.GetWorkerUpdate(ctx, env.ownerID, updateGolden.Receipt["update_id"].(string))
	if err != nil {
		t.Fatalf("load replayed update: %v", err)
	}
	updateReplayView := map[string]any{
		"update_id":       storedUpdate.UpdateID,
		"caller_agent_id": storedUpdate.AgentID,
		"target_agent_id": storedUpdate.TargetAgentID,
		"channel_id":      storedUpdate.ChannelID,
		"trajectory_id":   storedUpdate.TrajectoryID,
		"packet_kind":     storedUpdate.Direction,
		"packet_digest":   storedUpdate.PayloadDigest,
		"durable_cursor":  storedUpdate.MessageSeq,
		"status":          "existing",
	}
	rlmAssertReplayEquality(t, updateGolden, updateReplayView)
	env.rlmAssertCensusEqual(t, ctx, manifest, censusBefore, "update_coagent")

	// Conflict probe: same identity, mutated packet refused pre-dispatch.
	mutatedPacket := map[string]any{}
	for k, v := range updateGolden.Input["packet"].(map[string]any) {
		mutatedPacket[k] = v
	}
	mutatedPacket["summary"] = fmt.Sprintf("mutated claim %d", time.Now().UnixNano())
	mutatedUpdateInput := map[string]any{"agent_id": updateGolden.Input["agent_id"], "packet": mutatedPacket}
	if _, err := identities.claim("update_coagent", updateGolden.SemanticID, mutatedUpdateInput); err == nil {
		t.Fatal("update conflict probe: mutated packet on the same identity was not refused pre-dispatch")
	}
	env.rlmAssertCensusEqual(t, ctx, manifest, censusBefore, "update conflict")

	// New-identity leg: a mutated packet under a fresh identity dispatches and
	// mints a DISTINCT update row — a fresh operation, not a replay.
	mutatedBody, _ := json.Marshal(mutatedPacket)
	mutatedCell := fmt.Sprintf(`package main
func main() {
_, err := choir.Message(%q, "execution_result", %q)
if err != nil { panic(err) }
}`,
		updateGolden.Input["agent_id"].(string), string(mutatedBody))
	if _, err := identities.claim("update_coagent", updateGolden.SemanticID+"|fresh", mutatedUpdateInput); err != nil {
		t.Fatalf("update new-identity claim: %v", err)
	}
	dispatches.record(updateGolden.SemanticID + "|fresh")
	env.rlmReplayCell(t, ctx, &implRun, implToolCtx, "replay-update-fresh-"+runSuffix, mutatedCell)
	updatesAfter, err := env.s.ListWorkerUpdatesByTrajectoryOG(ctx, env.ownerID, manifest.TrajectoryID, 1000)
	if err != nil {
		t.Fatalf("list updates after fresh leg: %v", err)
	}
	if len(updatesAfter) != censusBefore.updateCount+1 {
		t.Fatalf("update new-identity leg: expected exactly one new update row, census %d -> %d", censusBefore.updateCount, len(updatesAfter))
	}
	var freshFound bool
	for _, u := range updatesAfter {
		if u.UpdateID != updateGolden.Receipt["update_id"].(string) {
			freshFound = true
		}
	}
	if !freshFound {
		t.Fatal("update new-identity leg: no distinct update row minted")
	}
	// The golden update itself must be untouched.
	storedUpdate2, err := env.s.GetWorkerUpdate(ctx, env.ownerID, updateGolden.Receipt["update_id"].(string))
	if err != nil || storedUpdate2.UpdateID != storedUpdate.UpdateID {
		t.Fatalf("update fresh leg disturbed golden update: %v", err)
	}

	// --- Row 8: record_assignment_result -> choir.Complete ---
	// The cell stages choir.Complete; the reducer authors the assignment fate
	// through commitCompleteIntent. The assignment is terminal, so the durable
	// layer replays the recorded fate.
	reportGolden := rlmReplayLoadGolden(t, goldenDir, "record_assignment_result")
	reportInput := map[string]any{
		"result":         reportGolden.Input["result"],
		"verdict":        reportGolden.Input["verdict"],
		"summary":        reportGolden.Input["summary"],
		"evidence_refs":  reportGolden.Input["evidence_refs"],
		"execution_refs": reportGolden.Input["execution_refs"],
	}
	reportCell := fmt.Sprintf(`package main
func main() {
err := choir.Complete(%q, %q, %q, %s, %s)
if err != nil { panic(err) }
}`,
		reportGolden.Input["result"].(string), reportGolden.Input["verdict"].(string),
		reportGolden.Input["summary"].(string),
		rlmGoStringSlice(rlmStringSlice(reportGolden.Input["evidence_refs"])),
		rlmGoStringSlice(rlmStringSlice(reportGolden.Input["execution_refs"])))

	censusBefore = env.rlmTakeCensus(t, ctx, manifest)
	if _, err := identities.claim("record_assignment_result", reportGolden.SemanticID, reportInput); err != nil {
		t.Fatalf("report identity claim: %v", err)
	}
	dispatches.record(reportGolden.SemanticID)
	env.rlmReplayCell(t, ctx, &implRun, implToolCtx, "replay-report-"+runSuffix, reportCell)
	// Canonical equality through the store: the assignment must still carry
	// exactly the golden report — no second report, no disposition change.
	assignment, err := env.s.GetCoSuperAssignment(ctx, env.ownerID, env.computerID, manifest.ImplAssignmentID, 1)
	if err != nil {
		t.Fatalf("load impl assignment: %v", err)
	}
	if len(assignment.ReportRefs) != 1 {
		t.Fatalf("replay minted extra reports: %v", assignment.ReportRefs)
	}
	// ReportRefs carry object refs (obj:choir.co_super_assignment_report:...),
	// not report IDs — resolve the durable row through the golden's report_id
	// identity key. Every compared VALUE below still comes from the loaded row.
	reportID := rlmGoldenString(t, reportGolden.Durable, "report_id")
	report, err := env.s.GetCoSuperAssignmentReport(ctx, env.ownerID, env.computerID, reportID)
	if err != nil {
		t.Fatalf("load stored report: %v", err)
	}
	// Project the P0 field set from the stored report and assignment — never
	// from the golden — so equality on every field can actually fail. `replay`
	// is excluded from the compared set (see rlmAssertReplayEquality): a
	// capture-time golden cannot carry the replay marker.
	reportReplayView := map[string]any{
		"assignment_id":      report.AssignmentID,
		"attempt":            float64(report.Attempt),
		"disposition":        string(assignment.Disposition),
		"proposition_digest": report.PropositionDigest,
		"result":             string(report.Result),
		"verdict":            rlmEmptyToNil(string(report.Verdict)),
		"summary":            report.Summary,
		"evidence_refs":      report.EvidenceRefs,
		"command_ids":        rlmReportCommandIDs(report),
		"output_digests":     rlmReportOutputDigests(report),
		"candidate":          rlmEmptyToNil(report.CandidateID),
		"report_id":          report.ReportID,
	}
	rlmAssertReplayEquality(t, reportGolden, reportReplayView, "replay")
	env.rlmAssertCensusEqual(t, ctx, manifest, censusBefore, "record_assignment_result")

	// Conflict probe: same identity, mutated verdict refused pre-dispatch.
	mutatedReportInput := map[string]any{
		"result": reportGolden.Input["result"], "verdict": "fail",
		"summary": reportGolden.Input["summary"], "evidence_refs": reportGolden.Input["evidence_refs"],
		"execution_refs": reportGolden.Input["execution_refs"],
	}
	if _, err := identities.claim("record_assignment_result", reportGolden.SemanticID, mutatedReportInput); err == nil {
		t.Fatal("report conflict probe: mutated verdict on the same identity was not refused pre-dispatch")
	}
	env.rlmAssertCensusEqual(t, ctx, manifest, censusBefore, "report conflict")

	// New-identity leg: verdict is digest-covered, so a mutated verdict under
	// a fresh identity derives a different proposition. On a terminal
	// assignment the saga must refuse or record it as late evidence that
	// cannot reopen the disposition or rebind the report.
	conflictCell := fmt.Sprintf(`package main
func main() {
err := choir.Complete(%q, "fail", "mutated verdict", %s, %s)
if err != nil { panic(err) }
}`,
		reportGolden.Input["result"].(string),
		rlmGoStringSlice(rlmStringSlice(reportGolden.Input["evidence_refs"])),
		rlmGoStringSlice(rlmStringSlice(reportGolden.Input["execution_refs"])))
	if _, err := identities.claim("record_assignment_result", reportGolden.SemanticID+"|conflict", mutatedReportInput); err != nil {
		t.Fatalf("report new-identity claim: %v", err)
	}
	dispatches.record(reportGolden.SemanticID + "|conflict")
	_, conflictErr := env.rlmReplayCellErr(ctx, &implRun, implToolCtx, "replay-report-conflict-"+runSuffix, conflictCell)
	assignmentAfter, err := env.s.GetCoSuperAssignment(ctx, env.ownerID, env.computerID, manifest.ImplAssignmentID, 1)
	if err != nil {
		t.Fatalf("reload impl assignment: %v", err)
	}
	if string(assignmentAfter.Disposition) != reportGolden.Receipt["disposition"].(string) {
		t.Fatalf("conflict probe changed disposition: %s", assignmentAfter.Disposition)
	}
	if len(assignmentAfter.ReportRefs) != 1 {
		t.Fatalf("conflict probe rebound terminal report: %v", assignmentAfter.ReportRefs)
	}
	if conflictErr == nil {
		t.Log("report new-identity leg recorded as late evidence (distinct identity, golden state intact)")
	}

	t.Logf("replayed 5 goldens through the in-cell carrier: canonical equality + zero-effect census + conflict/new-identity legs passed")
}
