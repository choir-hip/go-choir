package store

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestProjectionV1RejectsRuntimeControlRows(t *testing.T) {
	batch := computerevent.ProjectionBatch{
		Version: computerevent.ProjectionBatchV1, ProjectorVersion: computerevent.ProjectorVersionV1,
		ComputerID: "computer-version", EventID: "event-version", Ops: []computerevent.ProjectionOp{{Kind: computerevent.ProjectionOpRunMemoryEntry}},
	}
	if err := batch.Validate(); err == nil {
		t.Fatal("projection v1 admitted a v2 runtime-control operation")
	}
}

func TestProjectBatchAppliesRuntimeControlRowsAndCASGuards(t *testing.T) {
	ctx := context.Background()
	productStore := openProjectStore(t)
	computerID := "computer-runtime-projection"
	prepareGenesis(t, productStore, computerID, "genesis-runtime-projection")
	head, err := productStore.Head(ctx, computerID)
	if err != nil || head == nil {
		t.Fatalf("head: %v %#v", err, head)
	}
	now := time.Date(2026, 8, 18, 6, 30, 0, 0, time.UTC).Format(time.RFC3339Nano)
	commitment := storeTestDigest('a')
	operation := computerevent.SelfDevelopmentOperationProjection{
		OperationID: "operation-runtime", IdempotencyKey: "idem-runtime", RequestCommitment: commitment,
		ComputerID: computerID, TrajectoryID: "trajectory-runtime", BaseHead: commitment,
		PromptArtifactRef: "artifact:sha256:" + strings.Repeat("b", 64), VerifierRefsJSON: "[]",
		DesiredHead: commitment, EffectiveHead: commitment, State: selfdev.StateRequested,
		CreatedAt: now, UpdatedAt: now,
	}
	intent := computerevent.SelfDevelopmentStartIntentProjection{
		ComputerID: computerID, IdempotencyKey: "intent-runtime", RequestCommitment: commitment, CreatedAt: now,
	}
	memory := computerevent.RunMemoryEntryProjection{
		EntryID: "memory-runtime", RunID: "run-runtime", OwnerID: "owner-runtime", AgentID: "agent-runtime",
		Seq: 1, Kind: string(types.RunMemoryEntryMessage), Role: "user", DetailsJSON: "{}", CreatedAt: now,
	}
	mutation := computerevent.TextureAgentMutationProjection{
		DocID: "doc-runtime", RunID: "run-texture-runtime", OwnerID: "owner-runtime", ComputerID: computerID,
		State: "pending", CreatedAt: now,
	}
	batch := computerevent.ProjectionBatch{
		Version: computerevent.ProjectionBatchV2, ProjectorVersion: computerevent.ProjectorVersionV2,
		ComputerID: computerID,
		Ops: []computerevent.ProjectionOp{
			{Kind: computerevent.ProjectionOpRunMemoryEntry, CanonicalID: memory.EntryID, Body: mustJSON(t, memory)},
			{Kind: computerevent.ProjectionOpSelfDevelopmentStartIntent, CanonicalID: intent.IdempotencyKey, Body: mustJSON(t, intent)},
			{Kind: computerevent.ProjectionOpSelfDevelopmentOperation, CanonicalID: operation.OperationID, Body: mustJSON(t, operation)},
			{Kind: computerevent.ProjectionOpTextureAgentMutation, CanonicalID: mutation.RunID, Body: mustJSON(t, mutation)},
		},
	}
	event, digest := prepareProjectionEvent(t, productStore, *head, mustEventID(t), "runtime-projection-1")
	batch.EventID, batch.EventDigest = event.EventID, digest
	if err := productStore.FinalizeBatch(ctx, computerID, digest, signedReceipt(t, computerID, digest, event.Sequence), &batch); err != nil {
		t.Fatal(err)
	}
	assertCount(t, productStore, `SELECT COUNT(*) FROM run_memory_entries WHERE entry_id='memory-runtime'`, 1)
	assertCount(t, productStore, `SELECT COUNT(*) FROM self_development_start_intents WHERE computer_id='computer-runtime-projection'`, 1)
	assertCount(t, productStore, `SELECT COUNT(*) FROM self_development_operations WHERE operation_id='operation-runtime'`, 1)
	assertCount(t, productStore, `SELECT COUNT(*) FROM texture_agent_mutations WHERE loop_id='run-texture-runtime'`, 1)

	head, err = productStore.Head(ctx, computerID)
	if err != nil || head == nil {
		t.Fatalf("second head: %v %#v", err, head)
	}
	operation.State = selfdev.StateExecuting
	operation.ExpectedState = selfdev.StateRequested
	operation.UpdatedAt = time.Date(2026, 8, 18, 6, 31, 0, 0, time.UTC).Format(time.RFC3339Nano)
	mutation.State = "completed"
	mutation.RevisionID = "revision-runtime"
	mutation.ExpectedStates = []string{"pending"}
	completedAt := now
	mutation.CompletedAt = &completedAt
	batch2 := computerevent.ProjectionBatch{
		Version: computerevent.ProjectionBatchV2, ProjectorVersion: computerevent.ProjectorVersionV2,
		ComputerID: computerID,
		Ops: []computerevent.ProjectionOp{
			{Kind: computerevent.ProjectionOpSelfDevelopmentOperation, CanonicalID: operation.OperationID, Body: mustJSON(t, operation)},
			{Kind: computerevent.ProjectionOpTextureAgentMutation, CanonicalID: mutation.RunID, Body: mustJSON(t, mutation)},
		},
	}
	event2, digest2 := prepareProjectionEvent(t, productStore, *head, mustEventID(t), "runtime-projection-2")
	batch2.EventID, batch2.EventDigest = event2.EventID, digest2
	if err := productStore.FinalizeBatch(ctx, computerID, digest2, signedReceipt(t, computerID, digest2, event2.Sequence), &batch2); err != nil {
		t.Fatal(err)
	}
	var operationState, mutationState, revisionID string
	if err := productStore.db.QueryRowContext(ctx, `SELECT state FROM self_development_operations WHERE operation_id=?`, operation.OperationID).Scan(&operationState); err != nil {
		t.Fatal(err)
	}
	if err := productStore.db.QueryRowContext(ctx, `SELECT state, revision_id FROM texture_agent_mutations WHERE loop_id=?`, mutation.RunID).Scan(&mutationState, &revisionID); err != nil {
		t.Fatal(err)
	}
	if operationState != selfdev.StateExecuting || mutationState != "completed" || revisionID != "revision-runtime" {
		t.Fatalf("projected transitions operation=%q mutation=%q revision=%q", operationState, mutationState, revisionID)
	}

	head, err = productStore.Head(ctx, computerID)
	if err != nil || head == nil {
		t.Fatalf("third head: %v %#v", err, head)
	}
	operation.State = selfdev.StateVerified
	operation.ExpectedState = selfdev.StateRequested
	badBatch := computerevent.ProjectionBatch{
		Version: computerevent.ProjectionBatchV2, ProjectorVersion: computerevent.ProjectorVersionV2,
		ComputerID: computerID,
		Ops:        []computerevent.ProjectionOp{{Kind: computerevent.ProjectionOpSelfDevelopmentOperation, CanonicalID: operation.OperationID, Body: mustJSON(t, operation)}},
	}
	event3, digest3 := prepareProjectionEvent(t, productStore, *head, mustEventID(t), "runtime-projection-bad")
	badBatch.EventID, badBatch.EventDigest = event3.EventID, digest3
	if err := productStore.FinalizeBatch(ctx, computerID, digest3, signedReceipt(t, computerID, digest3, event3.Sequence), &badBatch); !errors.Is(err, computerevent.ErrProjectionMismatch) {
		t.Fatalf("stale operation CAS error=%v, want projection mismatch", err)
	}
}

func TestFinalizeBatchRejectsMissingLegacyTextureTransition(t *testing.T) {
	ctx := context.Background()
	productStore := openProjectStore(t)
	computerID := "computer-missing-legacy-texture"
	prepareGenesis(t, productStore, computerID, "genesis-missing-legacy-texture")
	head, err := productStore.Head(ctx, computerID)
	if err != nil || head == nil {
		t.Fatalf("head: %v %#v", err, head)
	}
	mutation := computerevent.TextureAgentMutationProjection{
		DocID: "doc-missing-legacy", RunID: "run-missing-legacy", OwnerID: "owner-legacy",
		State: "completed", ExpectedStates: []string{"sleeping"}, CreatedAt: "2026-08-18T07:20:00Z",
	}
	batch := computerevent.ProjectionBatch{
		Version: computerevent.ProjectionBatchV2, ProjectorVersion: computerevent.ProjectorVersionV2,
		ComputerID: computerID, Ops: []computerevent.ProjectionOp{{
			Kind: computerevent.ProjectionOpTextureAgentMutation, CanonicalID: mutation.RunID, Body: mustJSON(t, mutation),
		}},
	}
	event, digest := prepareProjectionEvent(t, productStore, *head, mustEventID(t), "missing-legacy-texture-transition")
	batch.EventID, batch.EventDigest = event.EventID, digest
	if err := productStore.FinalizeBatch(ctx, computerID, digest, signedReceipt(t, computerID, digest, event.Sequence), &batch); !errors.Is(err, computerevent.ErrProjectionMismatch) {
		t.Fatalf("missing legacy Texture transition error=%v, want projection mismatch", err)
	}
}

func TestFinalizeReplayBatchSeedsMissingLegacyTextureTransition(t *testing.T) {
	ctx := context.Background()
	productStore := openProjectStore(t)
	computerID := "computer-replay-legacy-texture"
	prepareGenesis(t, productStore, computerID, "genesis-replay-legacy-texture")
	head, err := productStore.Head(ctx, computerID)
	if err != nil || head == nil {
		t.Fatalf("head: %v %#v", err, head)
	}
	mutation := computerevent.TextureAgentMutationProjection{
		DocID: "doc-replay-legacy", RunID: "run-replay-legacy", OwnerID: "owner-legacy",
		State: "completed", ExpectedStates: []string{"sleeping"}, CreatedAt: "2026-08-18T07:21:00Z",
	}
	batch := computerevent.ProjectionBatch{
		Version: computerevent.ProjectionBatchV2, ProjectorVersion: computerevent.ProjectorVersionV2,
		ComputerID: computerID, Ops: []computerevent.ProjectionOp{{
			Kind: computerevent.ProjectionOpTextureAgentMutation, CanonicalID: mutation.RunID, Body: mustJSON(t, mutation),
		}},
	}
	event, digest := prepareProjectionEvent(t, productStore, *head, mustEventID(t), "replay-legacy-texture-transition")
	batch.EventID, batch.EventDigest = event.EventID, digest
	if err := productStore.FinalizeReplayBatch(ctx, computerID, digest, signedReceipt(t, computerID, digest, event.Sequence), &batch); err != nil {
		t.Fatal(err)
	}
	var state, storedComputerID string
	if err := productStore.db.QueryRowContext(ctx,
		`SELECT state, computer_id FROM texture_agent_mutations WHERE owner_id=? AND computer_id=? AND doc_id=? AND loop_id=?`,
		mutation.OwnerID, mutation.ComputerID, mutation.DocID, mutation.RunID).Scan(&state, &storedComputerID); err != nil {
		t.Fatal(err)
	}
	if state != mutation.State || storedComputerID != "" {
		t.Fatalf("seeded legacy Texture mutation state=%q computer_id=%q", state, storedComputerID)
	}
}

func TestFinalizeReplayBatchSeedsMissingScopedTextureTransitionBeforeResidueSnapshot(t *testing.T) {
	ctx := context.Background()
	productStore := openProjectStore(t)
	computerID := "computer-replay-scoped-texture"
	prepareGenesis(t, productStore, computerID, "genesis-replay-scoped-texture")
	head, err := productStore.Head(ctx, computerID)
	if err != nil || head == nil {
		t.Fatalf("head: %v %#v", err, head)
	}
	mutation := computerevent.TextureAgentMutationProjection{
		DocID: "doc-replay-scoped", RunID: "run-replay-scoped", OwnerID: "owner-scoped",
		ComputerID: computerID, State: "completed", CreatedAt: "2026-08-18T07:22:00Z",
		ExpectedStates: []string{"sleeping"},
	}
	first := computerevent.ProjectionBatch{
		Version: computerevent.ProjectionBatchV2, ProjectorVersion: computerevent.ProjectorVersionV2,
		ComputerID: computerID, Ops: []computerevent.ProjectionOp{{
			Kind: computerevent.ProjectionOpTextureAgentMutation, CanonicalID: mutation.RunID, Body: mustJSON(t, mutation),
		}},
	}
	event, digest := prepareProjectionEvent(t, productStore, *head, mustEventID(t), "replay-scoped-texture-transition")
	first.EventID, first.EventDigest = event.EventID, digest
	if err := productStore.FinalizeReplayBatch(ctx, computerID, digest, signedReceipt(t, computerID, digest, event.Sequence), &first); err != nil {
		t.Fatalf("seed scoped transition: %v", err)
	}

	// A residue snapshot is appended after the transition. It must be an
	// idempotent witness, not the predecessor that makes the earlier event replayable.
	head, err = productStore.Head(ctx, computerID)
	if err != nil || head == nil {
		t.Fatalf("post-transition head: %v %#v", err, head)
	}
	mutation.ExpectedStates = nil
	second := computerevent.ProjectionBatch{
		Version: computerevent.ProjectionBatchV2, ProjectorVersion: computerevent.ProjectorVersionV2,
		ComputerID: computerID, Ops: []computerevent.ProjectionOp{{
			Kind: computerevent.ProjectionOpTextureAgentMutation, CanonicalID: mutation.RunID, Body: mustJSON(t, mutation),
		}},
	}
	event2, digest2 := prepareProjectionEvent(t, productStore, *head, mustEventID(t), "replay-scoped-texture-residue")
	second.EventID, second.EventDigest = event2.EventID, digest2
	if err := productStore.FinalizeReplayBatch(ctx, computerID, digest2, signedReceipt(t, computerID, digest2, event2.Sequence), &second); err != nil {
		t.Fatalf("apply residue witness: %v", err)
	}
	var state, storedComputerID string
	if err := productStore.db.QueryRowContext(ctx,
		`SELECT state, computer_id FROM texture_agent_mutations WHERE owner_id=? AND computer_id=? AND doc_id=? AND loop_id=?`,
		mutation.OwnerID, computerID, mutation.DocID, mutation.RunID).Scan(&state, &storedComputerID); err != nil {
		t.Fatal(err)
	}
	if state != mutation.State || storedComputerID != computerID {
		t.Fatalf("replayed scoped Texture mutation state=%q computer_id=%q", state, storedComputerID)
	}
}

func TestProjectBatchRejectsMissingSelfDevelopmentTransition(t *testing.T) {
	ctx := context.Background()
	productStore := openProjectStore(t)
	computerID := "computer-missing-operation"
	prepareGenesis(t, productStore, computerID, "genesis-missing-operation")
	head, err := productStore.Head(ctx, computerID)
	if err != nil || head == nil {
		t.Fatalf("head: %v %#v", err, head)
	}
	operation := computerevent.SelfDevelopmentOperationProjection{
		OperationID: "missing-operation", IdempotencyKey: "missing-idem",
		RequestCommitment: storeTestDigest('e'), ComputerID: computerID,
		State: selfdev.StateExecuting, ExpectedState: selfdev.StateRequested,
		CreatedAt: time.Date(2026, 8, 18, 6, 35, 0, 0, time.UTC).Format(time.RFC3339Nano),
		UpdatedAt: time.Date(2026, 8, 18, 6, 35, 0, 0, time.UTC).Format(time.RFC3339Nano),
	}
	batch := computerevent.ProjectionBatch{
		Version: computerevent.ProjectionBatchV2, ProjectorVersion: computerevent.ProjectorVersionV2,
		ComputerID: computerID, Ops: []computerevent.ProjectionOp{{
			Kind:        computerevent.ProjectionOpSelfDevelopmentOperation,
			CanonicalID: operation.OperationID, Body: mustJSON(t, operation),
		}},
	}
	event, digest := prepareProjectionEvent(t, productStore, *head, mustEventID(t), "missing-operation-transition")
	batch.EventID, batch.EventDigest = event.EventID, digest
	if err := productStore.FinalizeBatch(ctx, computerID, digest, signedReceipt(t, computerID, digest, event.Sequence), &batch); !errors.Is(err, computerevent.ErrProjectionMismatch) {
		t.Fatalf("missing operation CAS error=%v, want projection mismatch", err)
	}
}

func TestLiveRuntimeWritersAppendProjectionBatches(t *testing.T) {
	ctx := context.Background()
	productStore := openProjectStore(t)
	computerID := "computer-live-runtime"
	prepareGenesis(t, productStore, computerID, "genesis-live-runtime")
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "test"}, PrivateKey: privateKey}
	appender, err := computerevent.NewComputerEventAppender(computerID, liveTapePinner{signer: signer}, productStore, liveTapeCAS{signer: signer, projection: productStore}, liveTapeVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	if err := productStore.BindProjectionTape(computerID, appender); err != nil {
		t.Fatal(err)
	}

	createdAt := time.Date(2026, 8, 18, 6, 40, 0, 0, time.UTC)
	if _, err := productStore.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{EntryID: "memory-live", RunID: "run-live", OwnerID: "owner-live", AgentID: "agent-live", Kind: types.RunMemoryEntryMessage, Role: "user", CreatedAt: createdAt}); err != nil {
		t.Fatal(err)
	}
	operations, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	operations.BindProjectionSink(productStore)
	commitment := storeTestDigest('c')
	if err := operations.BindStartIntent(ctx, computerID, "intent-live", commitment); err != nil {
		t.Fatal(err)
	}
	operation, err := operations.Start(ctx, selfdev.StartRequest{ComputerID: computerID, IdempotencyKey: "operation-live", PromptArtifactRef: "artifact:sha256:" + strings.Repeat("d", 64)})
	if err != nil {
		t.Fatal(err)
	}
	if operation.State != selfdev.StateRequested {
		t.Fatalf("start state=%q", operation.State)
	}
	if _, err := operations.Transition(ctx, computerID, operation.OperationID, selfdev.StateRequested, selfdev.StateExecuting, nil); err != nil {
		t.Fatal(err)
	}
	if err := productStore.CreateAgentMutation(ctx, AgentMutation{DocID: "doc-live", RunID: "run-texture-live", OwnerID: "owner-live", ComputerID: computerID, State: "pending", CreatedAt: createdAt}); err != nil {
		t.Fatal(err)
	}
	if err := productStore.CompleteAgentMutation(ctx, "owner-live", computerID, "run-texture-live", "revision-live"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, productStore, `SELECT COUNT(*) FROM run_memory_entries WHERE entry_id='memory-live'`, 1)
	assertCount(t, productStore, `SELECT COUNT(*) FROM self_development_start_intents WHERE idempotency_key='intent-live'`, 1)
	var operationCount int
	if err := productStore.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM self_development_operations WHERE operation_id=?`, operation.OperationID).Scan(&operationCount); err != nil {
		t.Fatal(err)
	}
	if operationCount != 1 {
		t.Fatalf("projected operation count=%d", operationCount)
	}
	assertCount(t, productStore, `SELECT COUNT(*) FROM texture_agent_mutations WHERE loop_id='run-texture-live' AND state='completed'`, 1)
	var projectionEvents int
	if err := productStore.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM computer_event_index WHERE computer_id=? AND event_kind=?`, computerID, string(computerevent.EventProjectionBatchRecorded)).Scan(&projectionEvents); err != nil {
		t.Fatal(err)
	}
	if projectionEvents != 6 {
		t.Fatalf("projection event count=%d, want 6", projectionEvents)
	}
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func mustEventID(t *testing.T) string {
	t.Helper()
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	return eventID
}
