//go:build linux

package agentcore

// RLM replay-harness capture driver (P4-replay, golden side).
//
// This file is NOT committed to main. It lives only in the pre-cutover
// worktree at the deployed guest build (a907f713) and drives each retiring
// JSON tool's Func against a real Runtime + real capsule executor + real
// store, writing canonical golden receipts to $RLM_CAPTURE_STATE/goldens/.
// The replay driver on main re-invokes the in-cell successor under the same
// semantic identity against the same state dir and asserts canonical
// equality on the declared fields.
//
// Run on Node B:
//   RLM_CAPTURE_STATE=/root/rlm-replay-state CHOIR_CAPSULE_BROKER=/tmp/capsule-broker \
//     go test ./internal/agentcore -run TestRLMCaptureGoldens -v -timeout 600s

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
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

const rlmCaptureBuildSHA = "a907f713"

// rlmGoldenRecord is the committed golden contract: one row per retiring
// operation. DeclaredFields is the canonical-equality surface the replay
// driver asserts; Receipt is the full tool output for audit.
type rlmGoldenRecord struct {
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

type rlmCaptureManifest struct {
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

type rlmCaptureEnv struct {
	stateDir   string
	goldenDir  string
	rt         *Runtime
	s          *store.Store
	executor   *capsule.Executor
	appender   *computerevent.ComputerEventAppender
	updaterDir string
	sourceDir  string
	ownerID    string
	computerID string
	seed       store.CoSuperAssignmentSeed
}

func rlmCaptureStateDir(t *testing.T) string {
	t.Helper()
	dir := strings.TrimSpace(os.Getenv("RLM_CAPTURE_STATE"))
	if dir == "" {
		t.Skip("RLM_CAPTURE_STATE unset; capture driver runs only on the Node B pre-cutover worktree")
	}
	return filepath.Clean(dir)
}

func rlmGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=rlm-capture", "GIT_AUTHOR_EMAIL=rlm@capture",
		"GIT_COMMITTER_NAME=rlm-capture", "GIT_COMMITTER_EMAIL=rlm@capture")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func rlmCaptureRuntime(t *testing.T) *rlmCaptureEnv {
	t.Helper()
	stateDir := rlmCaptureStateDir(t)
	// Capture owns the state dir: a stale store/executor tree would replay
	// prior identities instead of capturing fresh goldens. Lazy-unmount any
	// capsule overlay mounts left by a killed run before removing the tree.
	if mounts, err := exec.Command("sh", "-c",
		"mount | grep "+stateDir+" | awk '{print $3}' | sort -r").Output(); err == nil {
		for _, m := range strings.Fields(string(mounts)) {
			_ = exec.Command("umount", "-l", m).Run()
		}
	}
	if err := os.RemoveAll(stateDir); err != nil {
		t.Fatalf("reset capture state: %v", err)
	}
	env := &rlmCaptureEnv{
		stateDir:   stateDir,
		goldenDir:  filepath.Join(stateDir, "goldens"),
		updaterDir: filepath.Join(stateDir, "updater"),
		sourceDir:  filepath.Join(stateDir, "source"),
		ownerID:    "owner-rlm-replay",
		computerID: "computer-rlm-replay",
	}
	for _, dir := range []string{env.goldenDir, env.updaterDir, env.sourceDir,
		filepath.Join(stateDir, "executor"), filepath.Join(stateDir, "lower")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	// computer-surface frontend marker.
	if _, err := os.Stat(filepath.Join(env.sourceDir, ".git")); err != nil {
		if err := os.MkdirAll(filepath.Join(env.sourceDir, "frontend"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(env.sourceDir, "frontend", "index.html"), []byte("<html>rlm replay subject</html>\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(env.sourceDir, "go.mod"), []byte("module replay.subject\n\ngo 1.24\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		rlmGit(t, env.sourceDir, "init", "-q")
		rlmGit(t, env.sourceDir, "add", "-A")
		rlmGit(t, env.sourceDir, "commit", "-qm", "replay subject")
	}
	return rlmOpenRuntime(t, env)
}

// rlmReplayRuntime opens the capture state dir without wiping it: the replay
// driver re-runs the five retiring operations against the persisted
// assignments, store, and source tree the capture produced.
func rlmReplayRuntime(t *testing.T) *rlmCaptureEnv {
	t.Helper()
	stateDir := rlmCaptureStateDir(t)
	env := &rlmCaptureEnv{
		stateDir:   stateDir,
		goldenDir:  filepath.Join(stateDir, "goldens"),
		updaterDir: filepath.Join(stateDir, "updater"),
		sourceDir:  filepath.Join(stateDir, "source"),
		ownerID:    "owner-rlm-replay",
		computerID: "computer-rlm-replay",
	}
	return rlmOpenRuntime(t, env)
}

// rlmOpenRuntime builds the executor, store, runtime, and event appender over
// env's state dir. Shared by capture (fresh state) and replay (persisted
// state).
func rlmOpenRuntime(t *testing.T, env *rlmCaptureEnv) *rlmCaptureEnv {
	t.Helper()
	brokerPath := strings.TrimSpace(os.Getenv("CHOIR_CAPSULE_BROKER"))
	if brokerPath == "" {
		t.Skip("CHOIR_CAPSULE_BROKER unset; capture/replay needs the broker binary")
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

	// Event appender + genesis so the self-development operation can pin a
	// real canonical base head.
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signingKey := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "rlm-capture"}, PrivateKey: privateKey}
	appender, err := computerevent.NewComputerEventAppender(env.computerID,
		rlmCapturePinner{signingKey}, s, rollbackTestCAS{key: signingKey, projection: s}, rollbackTestReceiptVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	env.rt.eventAppender = appender
	env.appender = appender
	if head, err := s.Head(context.Background(), env.computerID); err != nil || head == nil || head.CanonicalEventHead == "" {
		genesisID, _ := computerevent.NewEventID()
		genesis := computerevent.Event{
			SchemaVersion: computerevent.SchemaVersionV1, EventID: genesisID, ComputerID: env.computerID,
			EventKind: computerevent.EventGenesisImported, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
			IdempotencyKey: "rlm-capture-genesis", ActorProfile: "management", AuthorityRef: "owner", PrivacyClass: "owner",
			PayloadCommitment: strings.Repeat("a", 64), ProposedEffectRef: strings.Repeat("b", 64),
			ResultingEffectiveCommitment: strings.Repeat("a", 64), ReducerVersion: computerevent.ReducerVersionV1,
		}
		if _, err := appender.AppendNew(context.Background(), genesis, computerevent.TransitionInput{TargetStateCommitment: strings.Repeat("a", 64)}, nil); err != nil {
			t.Fatalf("append genesis: %v", err)
		}
	}
	return env
}

// rlmSeedOperation inserts the self-development operation row bound to the
// assignment trajectory with the current canonical head as base.
func (env *rlmCaptureEnv) rlmSeedOperation(t *testing.T, ctx context.Context, trajectoryID string) string {
	t.Helper()
	head, err := env.s.Head(ctx, env.computerID)
	if err != nil || head == nil || head.CanonicalEventHead == "" {
		t.Fatalf("canonical head unavailable: %v", err)
	}
	idempotencyKey := "rlm-replay-operation"
	operationID := "selfdev-rlm-replay"
	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := env.s.DB().ExecContext(ctx, `INSERT INTO self_development_operations (operation_id,computer_id,idempotency_key,request_commitment,trajectory_id,base_head,prompt_artifact_ref,verifier_refs_json,desired_head,effective_head,state,created_at,updated_at) VALUES (?,?,?,?,?,?,?,'[]',?,?,?, ?,?)`,
		operationID, env.computerID, idempotencyKey,
		computerevent.DigestBytes([]byte(env.computerID+"\x00"+idempotencyKey)),
		trajectoryID, head.CanonicalEventHead,
		"artifact:sha256:"+strings.Repeat("b", 64), head.DesiredEventHead, head.EffectiveEventHead,
		"executing", now, now); err != nil {
		t.Fatalf("insert self-development operation: %v", err)
	}
	return operationID
}

// rlmOpenAssignment mirrors the production open/bind sequence for one
// assignment index with a driver-chosen capsule ID and opaque handle.
func (env *rlmCaptureEnv) rlmOpenAssignment(t *testing.T, ctx context.Context, assignmentID, capsuleID, opaque string, index int, kind types.CoSuperAssignmentKind, candidateID, artifactRef, subjectDigest string) types.OpenCoSuperAssignmentRequest {
	t.Helper()
	seed := env.seed
	req := types.OpenCoSuperAssignmentRequest{
		CommandID: "command-open-" + assignmentID + "-1", AssignmentID: assignmentID,
		Binding: types.CoSuperAssignmentBinding{
			OwnerID: seed.OwnerID, ComputerID: seed.ComputerID, TrajectoryID: seed.TrajectoryID,
			ParentAgentID: seed.ParentAgentID, ParentRunID: seed.ParentRunID,
			ParentDecisionID: seed.ParentDecisionID, ParentControlID: seed.ParentControlID,
			ParentWorkItemID: seed.ParentWorkID, AssignedWorkItemID: seed.AssignedWorkIDs[index], AssignedAgentID: seed.AssignedAgentIDs[index],
			Kind: kind, Attempt: 1,
			ScopeDigest: objectgraph.SHA256([]byte("scope:" + assignmentID)), RequestDigest: objectgraph.SHA256([]byte("request:" + assignmentID)),
			CapabilityDigest: store.DigestCoSuperOpaqueCapability(opaque), ExecutionHandleDigest: objectgraph.SHA256([]byte(opaque)),
			SubjectDigest:     subjectDigest,
			SourceArtifactRef: artifactRef, SourceCandidateID: candidateID,
			Writable: true, CapsuleID: capsuleID,
			NetworkMode:    types.CoSuperCapsuleNetworkForbidden,
			FilesystemMode: types.CoSuperCapsuleFilesystemAssignmentLocalWritableOverlay,
		},
		AssignedAgent: types.AgentRecord{AgentID: seed.AssignedAgentIDs[index]},
		AssignedWork:  types.WorkItemRecord{WorkItemID: seed.AssignedWorkIDs[index], AssignedAgentID: seed.AssignedAgentIDs[index], Objective: "bounded delegated assignment"},
	}
	var err error
	req.CommandDigest, err = store.ComputeOpenCoSuperAssignmentDigest(req)
	if err != nil {
		t.Fatalf("open digest: %v", err)
	}
	if _, err := env.s.OpenCoSuperAssignment(ctx, req); err != nil {
		t.Fatalf("open %s: %v", assignmentID, err)
	}
	return req
}

func (env *rlmCaptureEnv) rlmBindAssignment(t *testing.T, ctx context.Context, open types.OpenCoSuperAssignmentRequest, runID, opaque, slot string) types.RunRecord {
	t.Helper()
	metadata := map[string]any{
		"work_item_ids": []string{open.Binding.AssignedWorkItemID}, "lifecycle_work_item_id": open.Binding.AssignedWorkItemID,
		"requested_by_agent_id": open.Binding.ParentAgentID, "requested_by_profile": "management",
		"requested_by_run_id": open.Binding.ParentRunID,
		"assignment_id": open.AssignmentID, "assignment_attempt": 1, "assignment_kind": string(open.Binding.Kind),
		"assigned_work_item_id": open.Binding.AssignedWorkItemID, "parent_work_item_id": open.Binding.ParentWorkItemID,
		"parent_decision_id": open.Binding.ParentDecisionID, "parent_control_id": open.Binding.ParentControlID,
		"capsule_id": open.Binding.CapsuleID, "scope_digest": open.Binding.ScopeDigest, "request_digest": open.Binding.RequestDigest,
		"capability_digest": open.Binding.CapabilityDigest, "execution_handle_digest": open.Binding.ExecutionHandleDigest,
		"subject_digest": open.Binding.SubjectDigest, "source_artifact_ref": open.Binding.SourceArtifactRef,
		"source_candidate_id": open.Binding.SourceCandidateID,
	}
	if slot != "" {
		metadata["co_super_slot"] = slot
	}
	run := types.RunRecord{
		RunID: runID, AgentID: open.Binding.AssignedAgentID, ChannelID: open.Binding.AssignedAgentID,
		RequestedByRunID: open.Binding.ParentRunID, TrajectoryID: open.Binding.TrajectoryID,
		AgentProfile: "engineering", AgentRole: "engineering", OwnerID: open.Binding.OwnerID, ComputerID: open.Binding.ComputerID,
		State: types.RunPending, Prompt: open.AssignedWork.Objective, Metadata: metadata,
	}
	req := types.BindCoSuperAssignmentRequest{
		CommandID: "command-bind-" + open.AssignmentID + "-1",
		OwnerID:   open.Binding.OwnerID, ComputerID: open.Binding.ComputerID, AssignmentID: open.AssignmentID,
		Attempt: 1, ExpectedLifecycleVersion: 1, RunID: runID, Run: run,
		OpaqueCapability: opaque, CapsuleID: open.Binding.CapsuleID,
	}
	var err error
	req.CommandDigest, err = store.ComputeBindCoSuperAssignmentDigest(req)
	if err != nil {
		t.Fatalf("bind digest: %v", err)
	}
	if _, err := env.s.BindCoSuperAssignment(ctx, req); err != nil {
		t.Fatalf("bind %s: %v", open.AssignmentID, err)
	}
	return run
}

func (env *rlmCaptureEnv) rlmExecCtx(ctx context.Context, rec *types.RunRecord, toolCallID string) context.Context {
	return toolregistry.WithExecutionContext(ctx, toolregistry.ExecutionContext{
		RunID: rec.RunID, ToolCallID: toolCallID, AgentID: rec.AgentID, OwnerID: rec.OwnerID,
		Profile: "engineering", Role: "engineering", ChannelID: rec.ChannelID, ComputerID: rec.ComputerID,
		RunRecord: rec,
	})
}

// rlmCapturePinner extends the shared rollback test pinner with non-private
// payload pinning so the verification event's AppendNewPayload succeeds.
type rlmCapturePinner struct{ key computerevent.SigningKey }

func (p rlmCapturePinner) PinEvent(ctx context.Context, computerID string, canonical []byte, requestCommitment string) (computerevent.PinResult, error) {
	return rollbackTestPinner{key: p.key}.PinEvent(ctx, computerID, canonical, requestCommitment)
}

func (p rlmCapturePinner) PinNonPrivatePayload(_ context.Context, computerID string, payload []byte, mediaType, privacyClass, pinIntentCommitment string) (computerevent.PinResult, error) {
	digest := computerevent.DigestBytes(payload)
	receipt, err := computerevent.NewSignedReceipt("PayloadPinReceipt", "corpusd", map[string]any{
		"computer_id": computerID, "artifact_digest": digest, "media_type": mediaType,
		"privacy_class": privacyClass, "pin_intent_commitment": pinIntentCommitment,
	}, []computerevent.SigningKey{p.key}, time.Now().UTC())
	return computerevent.PinResult{ArtifactDigest: digest, Receipt: receipt}, err
}


// rlmStageFrozenBundle constructs the frozen effect bundle the deployed
// commit_transaction would have produced had its classifier path been
// reachable, persists it under the updater incoming root, and transitions
// the operation to frozen — the exact post-state rows 6-7 inspect/verify.
func (env *rlmCaptureEnv) rlmStageFrozenBundle(t *testing.T, ctx context.Context, rec *types.RunRecord, handle, operationID string, execRefs []string) string {
	t.Helper()
	changes, err := env.executor.ExtractGranted(ctx, rec.RunID, handle)
	if err != nil {
		t.Fatalf("extract granted diff: %v", err)
	}
	if _, err := env.executor.ResolveGrantedExecutionReceipts(ctx, rec.RunID, handle, execRefs); err != nil {
		t.Fatalf("resolve granted receipts: %v", err)
	}
	capsuleID, err := env.executor.ResolveGrantedCapsuleID(rec.RunID, handle)
	if err != nil {
		t.Fatalf("resolve capsule id: %v", err)
	}
	operation, err := env.rt.selfdevOperations.Get(ctx, env.computerID, operationID)
	if err != nil {
		t.Fatalf("load operation: %v", err)
	}
	head, err := env.s.Head(ctx, env.computerID)
	if err != nil || head == nil {
		t.Fatalf("canonical head: %v", err)
	}
	files, temporary, err := env.executor.StageGrantedRelease(ctx, rec.RunID, handle, filepath.Join(env.updaterDir, "incoming"))
	if err != nil {
		t.Fatalf("stage release: %v", err)
	}
	sourceTreeDigest, err := env.executor.ResolveGrantedSourceSnapshotDigest(rec.RunID, handle)
	if err != nil {
		t.Fatalf("source snapshot digest: %v", err)
	}
	capabilityPolicyDigest, resourceReceipt, err := env.executor.ResolveGrantedFreezeBindings(rec.RunID, handle)
	if err != nil {
		t.Fatalf("freeze bindings: %v", err)
	}
	runtimeIntent, err := computerevent.CanonicalJSON(files)
	if err != nil {
		t.Fatal(err)
	}
	runtimeDigest := computerevent.DigestBytes(runtimeIntent)
	generatedRefs := make([]string, len(files))
	for i, file := range files {
		generatedRefs[i] = "artifact:sha256:" + file.SHA256
	}
	effects := make([]transaction.ChangeRecord, 0, len(changes))
	for _, ch := range changes {
		effects = append(effects, transaction.ChangeRecord{Path: ch.Path, Kind: ch.Kind.String(), Mode: uint32(ch.Mode)})
	}
	record := transaction.CapsuleEffectBundle{
		BundleVersion: 1, ComputerID: env.computerID, BaseEventHead: operation.BaseHead,
		TrajectoryRef: operation.TrajectoryID, CapsuleIdentity: capsuleID,
		CapabilityPolicyDigest: capabilityPolicyDigest,
		SourceTreeRef:          "source-tree:sha256:" + strings.TrimPrefix(sourceTreeDigest, "sha256:"),
		OrderedFileEffects:     effects,
		GeneratedArtifactRefs:  generatedRefs,
		BuildRecipeRef:         execRefs[0],
		RuntimeArtifactRef:     "runtime-artifact:sha256:" + runtimeDigest,
		TestReceipts:           execRefs[1:2],
		VerifierReceipts:       []string{},
		DependencyToolchainRefs: execRefs[2:3],
		ResourceReceipts:       []string{resourceReceipt},
		RuntimeFiles:           files,
		ClassifierV:            "v1",
		ClassifierDigest:       computerevent.DigestBytes([]byte("rlm-replay-classifier")),
		Groups:                 map[string][]transaction.ChangeRecord{"source": effects},
	}
	record.ContentDigest, err = record.ComputeContentDigest()
	if err != nil || record.Validate(false) != nil {
		t.Fatalf("bundle invalid: digest=%v validate=%v", err, record.Validate(false))
	}
	draft, err := computerevent.CanonicalJSON(record)
	if err != nil {
		t.Fatalf("canonical draft: %v", err)
	}
	if err := os.WriteFile(filepath.Join(temporary, "bundle.draft.json"), draft, 0o400); err != nil {
		t.Fatal(err)
	}
	frozenRoot := filepath.Join(env.updaterDir, "incoming", record.ContentDigest)
	if err := os.Rename(temporary, frozenRoot); err != nil {
		t.Fatalf("freeze bundle draft: %v", err)
	}
	if _, err := env.rt.selfdevOperations.Transition(ctx, env.computerID, operation.OperationID, "executing", "frozen", func(next *selfdev.Operation) error {
		next.CapsuleID = capsuleID
		next.BundleDigest = record.ContentDigest
		next.DesiredHead = head.DesiredEventHead
		next.EffectiveHead = head.EffectiveEventHead
		return nil
	}); err != nil {
		t.Fatalf("freeze operation: %v", err)
	}
	return record.ContentDigest
}
func (env *rlmCaptureEnv) rlmToolCtx(rec *types.RunRecord, handle string) *CapsuleToolCtx {
	return env.rt.assignedCoSuperCapsuleToolCtx(rec, handle)
}

func (env *rlmCaptureEnv) rlmWriteGolden(t *testing.T, rec rlmGoldenRecord) {
	t.Helper()
	rec.BuildSHA = rlmCaptureBuildSHA
	rec.CapturedAt = time.Now().UTC().Format(time.RFC3339Nano)
	raw, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		t.Fatalf("marshal golden %s: %v", rec.Operation, err)
	}
	path := filepath.Join(env.goldenDir, rec.Operation+".golden.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write golden %s: %v", path, err)
	}
}

func rlmParseReceipt(t *testing.T, raw string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("tool receipt is not JSON: %v\n%s", err, raw)
	}
	return out
}

// rlmSpawnImpl opens the implementation assignment, spawns its capsule from
// the source snapshot, mints the capability handle, and binds the run.
// Shared by capture and replay.
func (env *rlmCaptureEnv) rlmSpawnImpl(t *testing.T, ctx context.Context, assignmentID, runID, capsuleID, handle string) (types.OpenCoSuperAssignmentRequest, types.RunRecord) {
	t.Helper()
	preflight, err := env.executor.PreflightSourceSnapshot(ctx, "")
	if err != nil {
		t.Fatalf("preflight source: %v", err)
	}
	subject := "sha256:" + strings.TrimPrefix(preflight.SubjectDigest, "sha256:")
	open := env.rlmOpenAssignment(t, ctx, assignmentID, capsuleID, handle, 0,
		types.CoSuperAssignmentImplementation, "", preflight.ArtifactRef, subject)
	caps, err := env.executor.Spawn(ctx, capsule.SpawnSpec{
		CapsuleID: capsuleID, OwnerRunID: runID,
		MemoryMax: 1 << 30, CpuQuota: 100000, CpuPeriod: 100000, PidsMax: 256,
		WorkingDir: "/workspace/platform", Tier: capsule.TierMedium,
		SourceArtifactRef: preflight.ArtifactRef, ExpectedSubjectDigest: preflight.SubjectDigest,
	})
	if err != nil {
		t.Fatalf("spawn impl capsule: %v", err)
	}
	if _, err := env.executor.MintCapabilityHandle(runID, capsule.RoleCoSuper, caps.ID, handle, 24*time.Hour); err != nil {
		t.Fatalf("mint impl handle: %v", err)
	}
	run := env.rlmBindAssignment(t, ctx, open, runID, handle, "")
	return open, run
}

// TestRLMCaptureGoldens drives the five retiring JSON tools against the real
// pre-cutover path and writes one golden per operation.
func TestRLMCaptureGoldens(t *testing.T) {
	env := rlmCaptureRuntime(t)
	ctx := context.Background()

	seed, err := store.SeedCoSuperAssignmentAuthority(env.s, env.ownerID, env.computerID, 2)
	if err != nil {
		t.Fatalf("seed assignment authority: %v", err)
	}
	env.seed = seed
	operationID := env.rlmSeedOperation(t, ctx, seed.TrajectoryID)

	// --- Implementation assignment: open, spawn, mint, bind. ---
	implAssignmentID := "assignment-rlm-impl"
	implRunID := "run:rlm-impl"
	implCapsuleID := "capsule-rlm-impl"
	implHandle := "h-rlm-impl"

	_, implRun := env.rlmSpawnImpl(t, ctx, implAssignmentID, implRunID, implCapsuleID, implHandle)
	t.Cleanup(func() {
		// A failed capture must not leak a live capsule: the next run's
		// spawn would collide on the cgroup name and time out.
		for _, id := range []string{implCapsuleID, "capsule-rlm-verify"} {
			_ = env.executor.ForceDestroy(context.Background(), id)
		}
	})
	toolCtx := env.rlmToolCtx(&implRun, implHandle)
	execCtx := WithCapsuleCtx(env.rlmExecCtx(ctx, &implRun, "tool-call-rlm-impl"), toolCtx)
	// Mutate the worktree BEFORE the evidence cells: granted execution
	// receipts bind the final frozen worktree digest, so cells must run
	// against the tree the freeze will record. The release path is where
	// StageGrantedRelease collects frozen runtime artifacts; it requires an
	// executable bin/autoputer plus frontend files.
	for _, m := range []struct{ path, body string; mode uint32 }{
		{"/var/lib/artifact/release/bin/autoputer", "#!/bin/sh\necho rlm-replay\n", 0o755},
		{"/var/lib/artifact/release/frontend/index.html", "<html>rlm replay release</html>\n", 0o644},
	} {
		if err := env.executor.WriteFile(ctx, implRunID, implHandle, m.path, []byte(m.body), m.mode); err != nil {
			t.Fatalf("mutate worktree %s: %v", m.path, err)
		}
	}

	// --- Three exec evidence cells: build, test, toolchain. The bundle
	// validator requires capsule-exec: receipts (not capsule-go-eval:). ---
	execReceiptRefs := make([]string, 0, 3)
	for i, arg := range []string{"build-ok", "test-ok", "toolchain-ok"} {
		res, err := env.executor.Exec(ctx, implRunID, implHandle, capsule.ExecRequest{
			Command: "echo", Args: []string{arg}, Cwd: "/workspace/platform", TimeoutMS: 30000,
		})
		if err != nil || res.ReceiptRef == "" {
			t.Fatalf("exec evidence cell %d: res=%+v err=%v", i, res, err)
		}
		execReceiptRefs = append(execReceiptRefs, res.ReceiptRef)
	}

	// --- Three go-eval cells for the row-8 execution_refs (the report tool
	// resolves capsule-go-eval: receipts). ---
	evalTool := newCapsuleGoEvalTool(env.rt)
	cellSources := []string{
		`package main; import "fmt"; func main(){ fmt.Print("build ok") }`,
		`package main; import "fmt"; func main(){ fmt.Print("test ok") }`,
		`package main; import "fmt"; func main(){ fmt.Print("toolchain ok") }`,
	}
	receiptRefs := make([]string, 0, 3)
	for i, source := range cellSources {
		raw, err := evalTool.Func(execCtx, json.RawMessage(fmt.Sprintf(`{"source":%q,"timeout_ms":30000}`, source)))
		if err != nil {
			t.Fatalf("evidence cell %d: %v", i, err)
		}
		var result struct {
			ReceiptRef string `json:"receipt_ref"`
			Error      string `json:"error"`
		}
		if err := json.Unmarshal([]byte(raw), &result); err != nil || result.Error != "" || result.ReceiptRef == "" {
			t.Fatalf("evidence cell %d result=%s err=%v", i, raw, err)
		}
		receiptRefs = append(receiptRefs, result.ReceiptRef)
	}

	// --- Row 5: commit_transaction ---
	// Deployed-build finding: the classifier matches absolute ledger prefixes
	// against RELATIVE upperdir paths, so every real diff rejects. The golden
	// is the rejection receipt the deployed tool actually produces.
	commitTool := newCommitTransactionTool()
	commitRaw, err := commitTool.Func(execCtx, json.RawMessage(fmt.Sprintf(`{"handle":%q,"build_recipe_ref":%q,"test_receipts":[%q],"dependency_toolchain_refs":[%q]}`,
		implHandle, execReceiptRefs[0], execReceiptRefs[1], execReceiptRefs[2])))
	if err != nil {
		t.Fatalf("commit_transaction: %v", err)
	}
	commitReceipt := rlmParseReceipt(t, commitRaw)
	env.rlmWriteGolden(t, rlmGoldenRecord{
		Operation: "commit_transaction", Tool: "commit_transaction", Successor: "choir.Freeze",
		SemanticID: operationID,
		Input: map[string]any{"handle": implHandle, "build_recipe_ref": execReceiptRefs[0],
			"test_receipts": execReceiptRefs[1:2], "dependency_toolchain_refs": execReceiptRefs[2:3]},
		DeclaredFields: []string{"rejected", "reject_reason"},
		Receipt:        commitReceipt,
	})

	// The deployed freeze path is unreachable (always rejects), so rows 6-7
	// need a frozen bundle the deployed tools can inspect/verify. Construct
	// it exactly as commit_transaction would have, from the same capsule
	// state, and persist it under the updater incoming root.
	bundleDigest := env.rlmStageFrozenBundle(t, ctx, &implRun, implHandle, operationID, execReceiptRefs)

	// --- Verifier assignment: open against the frozen subject candidate,
	// spawn with the mounted bundle, mint, bind. ---
	verifyAssignmentID := "assignment-rlm-verify"
	verifyRunID := "run:rlm-verify"
	verifyCapsuleID := "capsule-rlm-verify"
	verifyHandle := "h-rlm-verify"
	verifyCandidateID := "candidate-rlm-verify"

	candidate, err := env.executor.PersistGrantedCandidate(ctx, implRunID, implHandle)
	if err != nil {
		t.Fatalf("persist subject candidate: %v", err)
	}
	verifyPreflight, err := env.executor.PreflightSourceSnapshot(ctx, candidate.ArtifactRef)
	if err != nil {
		t.Fatalf("preflight candidate: %v", err)
	}
	verifySubject := "sha256:" + strings.TrimPrefix(verifyPreflight.SubjectDigest, "sha256:")
	openVerify := env.rlmOpenAssignment(t, ctx, verifyAssignmentID, verifyCapsuleID, verifyHandle, 1,
		types.CoSuperAssignmentVerification, verifyCandidateID, candidate.ArtifactRef, verifySubject)
	verifyCaps, err := env.executor.Spawn(ctx, capsule.SpawnSpec{
		CapsuleID: verifyCapsuleID, OwnerRunID: verifyRunID,
		MemoryMax: 1 << 30, CpuQuota: 100000, CpuPeriod: 100000, PidsMax: 256,
		WorkingDir: "/workspace/platform", Tier: capsule.TierMedium,
		SourceArtifactRef: candidate.ArtifactRef, ExpectedSubjectDigest: verifyPreflight.SubjectDigest,
	})
	if err != nil {
		t.Fatalf("spawn verifier capsule: %v", err)
	}
	if _, err := env.executor.MintCapabilityHandle(verifyRunID, capsule.RoleCoSuper, verifyCaps.ID, verifyHandle, 24*time.Hour); err != nil {
		t.Fatalf("mint verifier handle: %v", err)
	}
	verifyRun := env.rlmBindAssignment(t, ctx, openVerify, verifyRunID, verifyHandle, "verifier")
	verifyToolCtx := env.rlmToolCtx(&verifyRun, verifyHandle)
	verifyExecCtx := WithCapsuleCtx(env.rlmExecCtx(ctx, &verifyRun, "tool-call-rlm-verify"), verifyToolCtx)

	// --- Row 6: inspect_self_development_bundle ---
	inspectTool := newInspectSelfDevelopmentBundleTool()
	inspectRaw, err := inspectTool.Func(verifyExecCtx, json.RawMessage(fmt.Sprintf(`{"operation_id":%q,"bundle_digest":%q}`, operationID, bundleDigest)))
	if err != nil {
		t.Fatalf("inspect_self_development_bundle: %v", err)
	}
	env.rlmWriteGolden(t, rlmGoldenRecord{
		Operation: "inspect_self_development_bundle", Tool: "inspect_self_development_bundle", Successor: "choir.InspectBundle",
		SemanticID: verifyRunID + "|" + operationID + "|" + bundleDigest,
		Input:      map[string]any{"operation_id": operationID, "bundle_digest": bundleDigest},
		DeclaredFields: []string{"operation_id", "content_digest", "source_tree_ref", "runtime_artifact_ref",
			"base_event_head", "build_recipe_ref", "test_receipts", "dependency_toolchain_refs",
			"resource_receipts", "classifier_version", "classifier_digest"},
		Receipt: rlmParseReceipt(t, inspectRaw),
	})

	// --- Row 7: record_self_development_verification ---
	verifyTool := newRecordSelfDevelopmentVerificationTool()
	verifierRefs := []string{"verifier-evidence:rlm-replay"}
	verifyRaw, err := verifyTool.Func(verifyExecCtx, json.RawMessage(fmt.Sprintf(
		`{"operation_id":%q,"bundle_digest":%q,"decision":"pass","verifier_refs":[%q]}`,
		operationID, bundleDigest, verifierRefs[0])))
	if err != nil {
		t.Fatalf("record_self_development_verification: %v", err)
	}
	env.rlmWriteGolden(t, rlmGoldenRecord{
		Operation: "record_self_development_verification", Tool: "record_self_development_verification", Successor: "choir.Verify",
		SemanticID: verifyRunID + "|" + operationID + "|" + bundleDigest + "|pass",
		Input: map[string]any{"operation_id": operationID, "bundle_digest": bundleDigest,
			"decision": "pass", "verifier_refs": verifierRefs},
		DeclaredFields: []string{"operation_id", "state", "bundle_digest", "decision", "verifier_ref"},
		Receipt:        rlmParseReceipt(t, verifyRaw),
	})

	// --- Row 9: update_coagent (before the terminal report so the caller run
	// is still active). ---
	updateTool := newUpdateCoagentTool(env.rt)
	packet := map[string]any{
		"schema_version": types.CoagentSourcePacketSchemaV1, "kind": "execution_result",
		"summary": "rlm replay golden update",
		"claims":  []map[string]any{{"text": "rlm replay claim"}},
	}
	updateInput := map[string]any{
		"agent_id": seed.ParentAgentID, "schema_version": types.CoagentSourcePacketSchemaV1,
		"kind": "execution_result", "summary": "rlm replay golden update",
		"claims": []map[string]any{{"text": "rlm replay claim"}},
	}
	updateRawJSON, _ := json.Marshal(updateInput)
	updateRaw, err := updateTool.Func(execCtx, updateRawJSON)
	if err != nil {
		t.Fatalf("update_coagent: %v", err)
	}
	env.rlmWriteGolden(t, rlmGoldenRecord{
		Operation: "update_coagent", Tool: "update_coagent", Successor: "choir.Message",
		SemanticID: implRunID + "|" + seed.ParentAgentID,
		Input:      map[string]any{"agent_id": seed.ParentAgentID, "packet": packet},
		DeclaredFields: []string{"update_id", "agent_id", "channel_id", "trajectory_id", "status"},
		Receipt:        rlmParseReceipt(t, updateRaw),
	})

	// --- Row 8: record_assignment_result (terminal completed; implementation
	// assignments cannot issue verdicts, so verdict is none). ---
	reportToolCallID := "tool-call-rlm-report"
	reportExecCtx := WithCapsuleCtx(env.rlmExecCtx(ctx, &implRun, reportToolCallID), toolCtx)
	reportTool := newRecordAssignedCoSuperReportTool(env.rt)
	reportRaw, err := reportTool.Func(reportExecCtx, json.RawMessage(fmt.Sprintf(
		`{"result":"completed","verdict":"none","summary":"rlm replay golden report","evidence_refs":[%q],"execution_refs":[%q,%q,%q]}`,
		"evidence:rlm-replay", receiptRefs[0], receiptRefs[1], receiptRefs[2])))
	if err != nil {
		t.Fatalf("record_assignment_result: %v", err)
	}
	env.rlmWriteGolden(t, rlmGoldenRecord{
		Operation: "record_assignment_result", Tool: "record_assignment_result", Successor: "choir.Complete",
		SemanticID: implRunID + "|completed|none",
		Input: map[string]any{"result": "completed", "verdict": "none", "summary": "rlm replay golden report",
			"evidence_refs": []string{"evidence:rlm-replay"}, "execution_refs": receiptRefs},
		DeclaredFields: []string{"assignment_id", "attempt", "disposition"},
		Receipt:        rlmParseReceipt(t, reportRaw),
	})

	// --- Manifest ---
	manifest := rlmCaptureManifest{
		BuildSHA: rlmCaptureBuildSHA, OwnerID: env.ownerID, ComputerID: env.computerID,
		TrajectoryID: seed.TrajectoryID, ImplAssignmentID: implAssignmentID, ImplRunID: implRunID,
		ImplAgentID: implRun.AgentID, ImplCapsuleID: implCapsuleID, ImplHandle: implHandle,
		VerifyAssignID: verifyAssignmentID, VerifyRunID: verifyRunID, VerifyAgentID: verifyRun.AgentID,
		VerifyCapsuleID: verifyCapsuleID, VerifyHandle: verifyHandle, VerifyCandidateID: verifyCandidateID,
		OperationID: operationID, BundleDigest: bundleDigest, CellReceiptRefs: receiptRefs,
		ReportToolCallID: reportToolCallID, ParentAgentID: seed.ParentAgentID, ParentRunID: seed.ParentRunID,
		CapturedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.goldenDir, "manifest.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("captured 5 goldens + manifest to %s", env.goldenDir)
}

// ---------------------------------------------------------------------------
// Replay driver (P4-replay, replay side).
//
// TestRLMReplayGoldens opens the persisted capture state dir and re-invokes
// each retiring operation's in-cell successor under the same semantic
// identity. For every row it asserts canonical equality on the golden's
// declared fields plus a zero-effect census: the canonical event head
// sequence must not advance and no new bundle/update/report may appear.
// A conflict probe per row mutates one input and asserts the replay either
// errors or produces a different semantic identity without new effects.
//
// Run on Node B after TestRLMCaptureGoldens:
//   RLM_CAPTURE_STATE=/root/rlm-replay-state CHOIR_CAPSULE_BROKER=/tmp/capsule-broker \
//     go test ./internal/agentcore -run TestRLMReplayGoldens -v -timeout 600s
// ---------------------------------------------------------------------------

func rlmLoadGolden(t *testing.T, dir, name string) rlmGoldenRecord {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, name+".golden.json"))
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	var rec rlmGoldenRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatalf("parse golden %s: %v", name, err)
	}
	return rec
}

func rlmLoadManifest(t *testing.T, dir string) rlmCaptureManifest {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m rlmCaptureManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	return m
}

// rlmHeadSeq returns the canonical event head sequence — the zero-effect
// probe. Any replay that appends an event advances it.
func (env *rlmCaptureEnv) rlmHeadSeq(t *testing.T, ctx context.Context) uint64 {
	t.Helper()
	head, err := env.s.Head(ctx, env.computerID)
	if err != nil || head == nil {
		t.Fatalf("canonical head: %v", err)
	}
	return head.Sequence
}

// rlmIncomingDirs snapshots the updater incoming root — a second freeze must
// not stage a new bundle directory.
func (env *rlmCaptureEnv) rlmIncomingDirs(t *testing.T) map[string]bool {
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
// cannot false-conflict.
func rlmAssertDeclaredFields(t *testing.T, golden rlmGoldenRecord, replay map[string]any) {
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
func (env *rlmCaptureEnv) rlmReplayReduction(t *testing.T, ctx context.Context, rec *types.RunRecord, toolCtx *CapsuleToolCtx) *rlmCallReduction {
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

func TestRLMReplayGoldens(t *testing.T) {
	env := rlmReplayRuntime(t)
	ctx := context.Background()
	manifest := rlmLoadManifest(t, env.goldenDir)

	implRun, err := env.s.GetRun(ctx, manifest.ImplRunID)
	if err != nil {
		t.Fatalf("load impl run: %v", err)
	}
	verifyRun, err := env.s.GetRun(ctx, manifest.VerifyRunID)
	if err != nil {
		t.Fatalf("load verify run: %v", err)
	}

	// Rebuild the impl capsule: respawn from the same source snapshot, mint a
	// fresh handle under the recorded name, and re-apply the release mutation
	// so the frozen worktree digest matches the capture.
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
	if _, err := env.executor.MintCapabilityHandle(manifest.ImplRunID, capsule.RoleCoSuper, manifest.ImplCapsuleID, manifest.ImplHandle, 24*time.Hour); err != nil {
		t.Fatalf("mint impl handle: %v", err)
	}
	for _, m := range []struct{ path, body string; mode uint32 }{
		{"/var/lib/artifact/release/bin/autoputer", "#!/bin/sh\necho rlm-replay\n", 0o755},
		{"/var/lib/artifact/release/frontend/index.html", "<html>rlm replay release</html>\n", 0o644},
	} {
		if err := env.executor.WriteFile(ctx, manifest.ImplRunID, manifest.ImplHandle, m.path, []byte(m.body), m.mode); err != nil {
			t.Fatalf("mutate worktree %s: %v", m.path, err)
		}
	}
	implToolCtx := env.rlmToolCtx(&implRun, manifest.ImplHandle)
	verifyToolCtx := env.rlmToolCtx(&verifyRun, manifest.VerifyHandle)

	// --- Row 5: commit_transaction -> freezeCapsuleEffectBundle ---
	// The successor body is invoked directly: the obligation gate is
	// enforcement, not operation semantics, and the impl assignment is
	// terminal after capture.
	commitGolden := rlmLoadGolden(t, env.goldenDir, "commit_transaction")
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
		t.Fatalf("commit_transaction conflict probe advanced canonical head")
	}

	// --- Row 6: inspect_self_development_bundle -> inspectSelfDevelopmentBundle ---
	inspectGolden := rlmLoadGolden(t, env.goldenDir, "inspect_self_development_bundle")
	inspectReplay, err := inspectSelfDevelopmentBundle(ctx, verifyToolCtx, &verifyRun,
		inspectGolden.Input["operation_id"].(string), inspectGolden.Input["bundle_digest"].(string))
	if err != nil {
		t.Fatalf("replay inspect_self_development_bundle: %v", err)
	}
	rlmAssertDeclaredFields(t, inspectGolden, inspectReplay)
	// Conflict probe: a wrong digest must error.
	if _, err := inspectSelfDevelopmentBundle(ctx, verifyToolCtx, &verifyRun,
		inspectGolden.Input["operation_id"].(string), strings.Repeat("0", 64)); err == nil {
		t.Fatal("inspect conflict probe: wrong digest accepted")
	}

	// --- Row 7: record_self_development_verification -> commitVerifyIntent ---
	// Replay binds the operation's CURRENT durable digest (the finalized
	// bundle), which is the receipt's bundle_digest — not the draft digest in
	// the golden input.
	verifyGolden := rlmLoadGolden(t, env.goldenDir, "record_self_development_verification")
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
		t.Fatalf("verify conflict probe advanced canonical head")
	}

	// --- Row 9: update_coagent -> commitMessageIntent ---
	updateGolden := rlmLoadGolden(t, env.goldenDir, "update_coagent")
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
		// A mutated body is a legitimately new update; the golden update must
		// be untouched.
		storedUpdate2, err2 := env.s.GetWorkerUpdate(ctx, env.ownerID, updateGolden.Receipt["update_id"].(string))
		if err2 != nil || storedUpdate2.UpdateID != storedUpdate.UpdateID {
			t.Fatalf("update conflict probe disturbed golden update: %v", err2)
		}
	}

	// --- Row 8: record_assignment_result -> commitCompleteIntent ---
	reportGolden := rlmLoadGolden(t, env.goldenDir, "record_assignment_result")
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
	if assignment.Report.ReportID != wantReportID {
		t.Fatalf("replay report mismatch: got %q want %q", assignment.Report.ReportID, wantReportID)
	}
	if len(assignment.ReportRefs) != 1 {
		t.Fatalf("replay minted extra reports: %v", assignment.ReportRefs)
	}
	// Conflict probe: a different summary on a terminal assignment must error.
	if _, err := reportReduction.commitCompleteIntent(reportCtx, yaegikernel.StagedIntent{
		LocalID: "replay-report-conflict", Kind: yaegikernel.IntentComplete,
		Result: "completed", Verdict: "none", Summary: "mutated summary",
		EvidenceRefs:  rlmStringSlice(reportGolden.Input["evidence_refs"]),
		ExecutionRefs: rlmStringSlice(reportGolden.Input["execution_refs"]),
	}); err == nil {
		t.Fatal("report conflict probe: mutated summary accepted on terminal assignment")
	}

	t.Logf("replayed 5 goldens: canonical equality + zero-effect census passed")
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
