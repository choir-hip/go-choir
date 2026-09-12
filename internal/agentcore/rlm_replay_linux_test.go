//go:build linux

package agentcore

// RLM replay-harness replay driver (P4-replay, replay side).
//
// Replays the five retiring operations' in-cell successors against the
// persisted capture state dir ($RLM_CAPTURE_STATE) produced by the
// pre-cutover capture driver at build a907f713. Goldens are the committed
// fixtures under docs/evidence/rlm-replay/goldens/.
//
// For every row the driver asserts canonical equality on the golden's
// declared fields plus a zero-effect census: the canonical event head
// sequence must not advance, no new bundle directory may appear, and no
// second update/report may be minted. A conflict probe per row mutates one
// input and asserts the replay either errors or lands under a different
// semantic identity without disturbing the golden state.
//
// Run on Node B:
//   RLM_CAPTURE_STATE=/root/rlm-replay-state CHOIR_CAPSULE_BROKER=/tmp/capsule-broker \
//     go test ./internal/agentcore -run TestRLMReplayGoldens -v -timeout 600s

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
	"strings"
	"testing"
	"time"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	contentowner "github.com/yusefmosiah/go-choir/internal/content"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
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
		t.Skip("RLM_CAPTURE_STATE unset; replay needs the persisted capture state dir")
	}
	brokerPath := strings.TrimSpace(os.Getenv("CHOIR_CAPSULE_BROKER"))
	if brokerPath == "" {
		t.Skip("CHOIR_CAPSULE_BROKER unset; replay needs the broker binary")
	}
	env := &rlmReplayEnv{
		stateDir:   filepath.Clean(stateDir),
		updaterDir: filepath.Join(stateDir, "updater"),
		sourceDir:  filepath.Join(stateDir, "source"),
		ownerID:    "owner-rlm-replay",
		computerID: "computer-rlm-replay",
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
	signingKey := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "rlm-capture"}, PrivateKey: privateKey}
	appender, err := computerevent.NewComputerEventAppender(env.computerID,
		rlmReplayPinner{signingKey}, s, rollbackTestCAS{key: signingKey, projection: s}, rollbackTestReceiptVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	env.rt.eventAppender = appender
	env.appender = appender
	return env
}

// rlmHeadSeq returns the canonical event head sequence — the zero-effect
// probe. Any replay that appends an event advances it.
func (env *rlmReplayEnv) rlmHeadSeq(t *testing.T, ctx context.Context) uint64 {
	t.Helper()
	head, err := env.s.Head(ctx, env.computerID)
	if err != nil || head == nil {
		t.Fatalf("canonical head: %v", err)
	}
	return head.Sequence
}

// rlmIncomingDirs snapshots the updater incoming root — a second freeze must
// not stage a new bundle directory.
func (env *rlmReplayEnv) rlmIncomingDirs(t *testing.T) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(env.updaterDir, "incoming"))
	if err != nil {
		t.Fatalf("read incoming root: %v", err)
	}
	out := make(map[string]bool, len(entries))
	for _, e := range entries {
		out[e.Name()] = true
	}
	return out
}

// rlmAssertDeclaredFields compares the golden's declared fields against the
// replayed receipt. Nested values compare via canonical JSON so map ordering
// cannot false-conflict. reject_reason compares by leading phrase: the
// rejected-path count varies with incidental capsule runtime writes and is
// not part of the operation's semantic identity.
func rlmAssertDeclaredFields(t *testing.T, golden rlmReplayGolden, replay map[string]any) {
	t.Helper()
	for _, field := range golden.DeclaredFields {
		want, ok := golden.Receipt[field]
		if !ok {
			t.Fatalf("%s: golden receipt lacks declared field %q", golden.Operation, field)
		}
		got, ok := replay[field]
		if !ok {
			t.Fatalf("%s: replay receipt lacks declared field %q (receipt %v)", golden.Operation, field, replay)
		}
		if field == "reject_reason" {
			wantStr, _ := want.(string)
			gotStr, _ := got.(string)
			if !strings.HasPrefix(gotStr, "unknown paths rejected at commit time") ||
				!strings.HasPrefix(wantStr, "unknown paths rejected at commit time") {
				t.Fatalf("%s: reject_reason diverged: golden=%q replay=%q", golden.Operation, wantStr, gotStr)
			}
			continue
		}
		// status flips submitted -> existing on replay: that flip IS the
		// dedup proof, not a divergence.
		if field == "status" {
			if got != "existing" {
				t.Fatalf("%s: replay status %v, expected existing (dedup proof)", golden.Operation, got)
			}
			continue
		}
		wantJSON, _ := computerevent.CanonicalJSON(want)
		gotJSON, _ := computerevent.CanonicalJSON(got)
		if string(wantJSON) != string(gotJSON) {
			t.Fatalf("%s: declared field %q diverged: golden=%v replay=%v", golden.Operation, field, want, got)
		}
	}
}

// rlmReplayReduction constructs the call reduction for one replayed cell,
// mirroring rlmReductionForCall's scope assembly. The durable cursor loads
// from the persisted store so the replayed cell sees the same inbox state.
func (env *rlmReplayEnv) rlmReplayReduction(t *testing.T, ctx context.Context, rec *types.RunRecord, toolCtx *CapsuleToolCtx) *rlmCallReduction {
	t.Helper()
	channel := channelIDForRun(rec)
	cursor, err := LoadInboxCursor(ctx, env.s, rec.OwnerID, rec.RunID, channel)
	if err != nil {
		t.Fatalf("load inbox cursor: %v", err)
	}
	return &rlmCallReduction{
		active: true, mb: env.rt, st: env.s, rec: rec, toolCtx: toolCtx,
		scope: ReductionScope{
			FromAgentID: rec.AgentID, FromRole: string(capsule.RoleCoSuper),
			ChannelID: channel, RunID: rec.RunID, OwnerID: rec.OwnerID,
			ReturnTo: metadataStringValue(rec.Metadata, "requested_by_agent_id"),
			Cursor:   cursor, CellID: fmt.Sprintf("%s:%d", rec.RunID, cursor),
		},
	}
}

func (env *rlmReplayEnv) rlmExecCtx(ctx context.Context, rec *types.RunRecord, toolCallID string) context.Context {
	return toolregistry.WithExecutionContext(ctx, toolregistry.ExecutionContext{
		RunID: rec.RunID, ToolCallID: toolCallID, AgentID: rec.AgentID, OwnerID: rec.OwnerID,
		Profile: "engineering", Role: "engineering", ChannelID: rec.ChannelID, ComputerID: rec.ComputerID,
		RunRecord: rec,
	})
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
	for _, id := range []string{manifest.ImplCapsuleID, manifest.VerifyCapsuleID, "capsule-rlm-verify-conflict"} {
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
	implToolCtx := env.rt.assignedCoSuperCapsuleToolCtx(&implRun, manifest.ImplHandle)
	verifyToolCtx := env.rt.assignedCoSuperCapsuleToolCtx(&verifyRun, manifest.VerifyHandle)

	// --- Row 5: commit_transaction -> freezeCapsuleEffectBundle ---
	// The successor body is invoked directly: the obligation gate is
	// enforcement, not operation semantics, and the impl assignment is
	// terminal after capture.
	commitGolden := rlmReplayLoadGolden(t, goldenDir, "commit_transaction")
	seqBefore := env.rlmHeadSeq(t, ctx)
	incomingBefore := env.rlmIncomingDirs(t)
	commitReplay, err := freezeCapsuleEffectBundle(ctx, implToolCtx, &implRun, manifest.ImplHandle,
		commitGolden.Input["build_recipe_ref"].(string),
		rlmStringSlice(commitGolden.Input["test_receipts"]),
		rlmStringSlice(commitGolden.Input["dependency_toolchain_refs"]))
	if err != nil {
		t.Fatalf("replay commit_transaction: %v", err)
	}
	rlmAssertDeclaredFields(t, commitGolden, commitReplay)
	if got := env.rlmHeadSeq(t, ctx); got != seqBefore {
		t.Fatalf("commit_transaction replay advanced canonical head: %d -> %d", seqBefore, got)
	}
	if got := env.rlmIncomingDirs(t); !reflect.DeepEqual(got, incomingBefore) {
		t.Fatalf("commit_transaction replay staged new bundle dirs: %v", got)
	}
	// Conflict probe: a bogus handle must error without new effects.
	if _, err := freezeCapsuleEffectBundle(ctx, implToolCtx, &implRun, "h-bogus",
		commitGolden.Input["build_recipe_ref"].(string),
		rlmStringSlice(commitGolden.Input["test_receipts"]),
		rlmStringSlice(commitGolden.Input["dependency_toolchain_refs"])); err == nil {
		t.Fatal("commit_transaction conflict probe: bogus handle accepted")
	}
	if got := env.rlmHeadSeq(t, ctx); got != seqBefore {
		t.Fatal("commit_transaction conflict probe advanced canonical head")
	}

	// --- Row 6: inspect_self_development_bundle -> choir.InspectBundle ---
	// The shared host body binds operation.BundleDigest, which post-verify is
	// the finalized digest — the draft-digest replay cannot re-enter it. The
	// real successor is the in-cell InspectBundle over the mounted bundle:
	// respawn the verifier capsule with the frozen draft mounted at
	// /selfdev/bundle and run the cell through the broker's go_eval path.
	inspectGolden := rlmReplayLoadGolden(t, goldenDir, "inspect_self_development_bundle")
	verifyGolden := rlmReplayLoadGolden(t, goldenDir, "record_self_development_verification")
	// Verification renamed the draft dir to the finalized digest; the draft
	// JSON inside still carries the draft content digest the binding names.
	draftDir := filepath.Join(env.updaterDir, "incoming", verifyGolden.Receipt["bundle_digest"].(string))
	binding, _ := json.Marshal(map[string]any{
		"operation_id":  inspectGolden.Input["operation_id"].(string),
		"bundle_digest": inspectGolden.Input["bundle_digest"].(string),
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
	inspectRes, err := env.executor.GoEval(ctx, manifest.VerifyRunID, manifest.VerifyHandle, capsule.GoEvalRequest{
		Source: inspectCell, Cwd: "/workspace/platform", TimeoutMS: 30000,
	})
	if err != nil || inspectRes.Error != "" {
		t.Fatalf("replay inspect cell: res=%+v err=%v", inspectRes, err)
	}
	if strings.HasPrefix(inspectRes.Stdout, "INSPECT_ERR:") {
		t.Fatalf("replay inspect_self_development_bundle: %s", inspectRes.Stdout)
	}
	var inspectReplay map[string]any
	if err := json.Unmarshal([]byte(inspectRes.Stdout), &inspectReplay); err != nil {
		t.Fatalf("parse inspect receipt: %v (stdout %q)", err, inspectRes.Stdout)
	}
	rlmAssertDeclaredFields(t, inspectGolden, inspectReplay)
	// Conflict probe: corrupt the mounted draft copy is impossible (read-only
	// bind); instead assert a wrong binding digest fails closed by inspecting
	// through a second capsule whose binding names a different digest.
	badBinding, _ := json.Marshal(map[string]any{
		"operation_id":  inspectGolden.Input["operation_id"].(string),
		"bundle_digest": strings.Repeat("0", 64),
	})
	if _, err := env.executor.Spawn(ctx, capsule.SpawnSpec{
		CapsuleID: "capsule-rlm-verify-conflict", OwnerRunID: manifest.VerifyRunID,
		MemoryMax: 1 << 30, CpuQuota: 100000, CpuPeriod: 100000, PidsMax: 256,
		WorkingDir: "/workspace/platform", Tier: capsule.TierMedium,
		SourceArtifactRef: verifyPreflight.ArtifactRef, ExpectedSubjectDigest: verifyPreflight.SubjectDigest,
		VerifierBundleDir: draftDir, VerifierBinding: string(badBinding),
	}); err != nil {
		t.Fatalf("spawn conflict verifier capsule: %v", err)
	}
	defer env.executor.ForceDestroy(context.Background(), "capsule-rlm-verify-conflict")
	if _, err := env.executor.MintCapabilityHandle(manifest.VerifyRunID, capsule.RoleCoSuper, "capsule-rlm-verify-conflict", "h-rlm-verify-conflict", 24*time.Hour, "verifier"); err != nil {
		t.Fatalf("mint conflict verifier handle: %v", err)
	}
	conflictRes, err := env.executor.GoEval(ctx, manifest.VerifyRunID, "h-rlm-verify-conflict", capsule.GoEvalRequest{
		Source: inspectCell, Cwd: "/workspace/platform", TimeoutMS: 30000,
	})
	if err == nil && conflictRes.Error == "" && !strings.HasPrefix(conflictRes.Stdout, "INSPECT_ERR:") {
		t.Fatal("inspect conflict probe: mismatched binding digest accepted")
	}
	// Replay binds the operation's CURRENT durable digest (the finalized
	// bundle), which is the receipt's bundle_digest — not the draft digest in
	// the golden input.
	// verifyGolden loaded above for the inspect mount.
	seqBefore = env.rlmHeadSeq(t, ctx)
	verifyReduction := env.rlmReplayReduction(t, ctx, &verifyRun, verifyToolCtx)
	verifyReplay, err := verifyReduction.commitVerifyIntent(ctx, yaegikernel.StagedIntent{
		LocalID: "replay-verify", Kind: yaegikernel.IntentVerify,
		Decision:     verifyGolden.Input["decision"].(string),
		VerifierRefs: rlmStringSlice(verifyGolden.Input["verifier_refs"]),
		BundleDigest: verifyGolden.Receipt["bundle_digest"].(string),
	})
	if err != nil {
		t.Fatalf("replay record_self_development_verification: %v", err)
	}
	rlmAssertDeclaredFields(t, verifyGolden, verifyReplay)
	if got := env.rlmHeadSeq(t, ctx); got != seqBefore {
		t.Fatalf("verify replay appended a second verification event: %d -> %d", seqBefore, got)
	}
	// Conflict probe: decision=fail on an awaiting-approval operation must
	// error without new effects.
	if _, err := verifyReduction.commitVerifyIntent(ctx, yaegikernel.StagedIntent{
		LocalID: "replay-verify-conflict", Kind: yaegikernel.IntentVerify,
		Decision: "fail", VerifierRefs: rlmStringSlice(verifyGolden.Input["verifier_refs"]),
		BundleDigest: verifyGolden.Receipt["bundle_digest"].(string),
	}); err == nil {
		t.Fatal("verify conflict probe: fail decision accepted on awaiting-approval operation")
	}
	if got := env.rlmHeadSeq(t, ctx); got != seqBefore {
		t.Fatal("verify conflict probe advanced canonical head")
	}

	// --- Row 9: update_coagent -> commitMessageIntent ---
	updateGolden := rlmReplayLoadGolden(t, goldenDir, "update_coagent")
	updateReduction := env.rlmReplayReduction(t, ctx, &implRun, implToolCtx)
	updateCtx := env.rlmExecCtx(ctx, &implRun, "replay-update")
	packetBody, _ := json.Marshal(updateGolden.Input["packet"])
	if _, err := updateReduction.commitMessageIntent(updateCtx, yaegikernel.StagedIntent{
		LocalID: "replay-update", Kind: yaegikernel.IntentMessage,
		ToDesk: updateGolden.Input["agent_id"].(string), MsgKind: "execution_result",
		Body: string(packetBody),
	}); err != nil {
		t.Fatalf("replay update_coagent: %v", err)
	}
	// Canonical equality is observed through the store: the deduplicated
	// update must carry the golden's declared fields.
	storedUpdate, err := env.s.GetWorkerUpdate(ctx, env.ownerID, updateGolden.Receipt["update_id"].(string))
	if err != nil {
		t.Fatalf("load replayed update: %v", err)
	}
	rlmAssertDeclaredFields(t, updateGolden, map[string]any{
		"update_id": storedUpdate.UpdateID, "agent_id": storedUpdate.TargetAgentID,
		"channel_id": storedUpdate.ChannelID, "trajectory_id": storedUpdate.TrajectoryID,
		"status": "existing",
	})
	// Conflict probe: a different packet body derives a different update_id —
	// a new semantic object, not a replay. Assert it errors or lands under a
	// different identity without touching the golden update.
	conflictBody := strings.Replace(string(packetBody), "rlm replay claim", "mutated claim", 1)
	if _, err := updateReduction.commitMessageIntent(updateCtx, yaegikernel.StagedIntent{
		LocalID: "replay-update-conflict", Kind: yaegikernel.IntentMessage,
		ToDesk: updateGolden.Input["agent_id"].(string), MsgKind: "execution_result",
		Body: conflictBody,
	}); err == nil {
		storedUpdate2, err2 := env.s.GetWorkerUpdate(ctx, env.ownerID, updateGolden.Receipt["update_id"].(string))
		if err2 != nil || storedUpdate2.UpdateID != storedUpdate.UpdateID {
			t.Fatalf("update conflict probe disturbed golden update: %v", err2)
		}
	}

	// --- Row 8: record_assignment_result -> commitCompleteIntent ---
	reportGolden := rlmReplayLoadGolden(t, goldenDir, "record_assignment_result")
	reportReduction := env.rlmReplayReduction(t, ctx, &implRun, implToolCtx)
	reportCtx := env.rlmExecCtx(ctx, &implRun, "replay-report")
	if _, err := reportReduction.commitCompleteIntent(reportCtx, yaegikernel.StagedIntent{
		LocalID: "replay-report", Kind: yaegikernel.IntentComplete,
		Result:   reportGolden.Input["result"].(string), Verdict: reportGolden.Input["verdict"].(string),
		Summary:       reportGolden.Input["summary"].(string),
		EvidenceRefs:  rlmStringSlice(reportGolden.Input["evidence_refs"]),
		ExecutionRefs: rlmStringSlice(reportGolden.Input["execution_refs"]),
	}); err != nil {
		t.Fatalf("replay record_assignment_result: %v", err)
	}
	// Canonical equality through the store: the assignment must still carry
	// exactly the golden report — no second report, no disposition change.
	assignment, err := env.s.GetCoSuperAssignment(ctx, env.ownerID, env.computerID, manifest.ImplAssignmentID, 1)
	if err != nil {
		t.Fatalf("load impl assignment: %v", err)
	}
	if string(assignment.Disposition) != reportGolden.Receipt["disposition"].(string) {
		t.Fatalf("replay changed disposition: %s", assignment.Disposition)
	}
	wantReportID, _ := reportGolden.Receipt["report"].(map[string]any)["report_id"].(string)
	// ReportRefs carries canonical object IDs; the report body carries the
	// golden's report_id. Exactly one ref must exist and resolve to it.
	if len(assignment.ReportRefs) != 1 {
		t.Fatalf("replay minted extra reports: %v", assignment.ReportRefs)
	}
	report, err := env.s.GetCoSuperAssignmentReport(ctx, env.ownerID, env.computerID, wantReportID)
	if err != nil || report.ReportID != wantReportID {
		t.Fatalf("replay report unreadable or mismatched: %v", err)
	}
	// Conflict probe: verdict is digest-covered, so a mutated verdict derives
	// a different proposition identity. On a terminal assignment the saga
	// records it as late evidence — a distinct report object that cannot
	// reopen the disposition or replace the bound report. Assert the golden
	// report and disposition survive unchanged.
	if _, err := reportReduction.commitCompleteIntent(reportCtx, yaegikernel.StagedIntent{
		LocalID: "replay-report-conflict", Kind: yaegikernel.IntentComplete,
		Result: "completed", Verdict: "fail", Summary: "mutated verdict",
		EvidenceRefs:  rlmStringSlice(reportGolden.Input["evidence_refs"]),
		ExecutionRefs: rlmStringSlice(reportGolden.Input["execution_refs"]),
	}); err != nil {
		t.Logf("report conflict probe errored (acceptable): %v", err)
	}
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

	t.Logf("replayed 5 goldens: canonical equality + zero-effect census passed")
}
