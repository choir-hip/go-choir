package store

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestProjectBatchAppliesDesktopAndOGAtomically(t *testing.T) {
	ctx := context.Background()
	productStore := openProjectStore(t)
	computerID := "computer-project"
	_, genesisDigest := prepareGenesis(t, productStore, computerID, "genesis-project")
	head, err := productStore.Head(ctx, computerID)
	if err != nil || head == nil {
		t.Fatalf("genesis head: %v %#v", err, head)
	}

	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	desktopBody, objectBody, edgeBody := projectBatchBodies(t, "win-a", "obj-a", "edge-a")
	event, digest := prepareProjectionEvent(t, productStore, *head, eventID, "project-1")
	batch := computerevent.ProjectionBatch{
		Version: computerevent.ProjectionBatchV1, ProjectorVersion: computerevent.ProjectorVersionV1,
		ComputerID: computerID, EventID: event.EventID, EventDigest: digest,
		Ops: []computerevent.ProjectionOp{
			{Kind: computerevent.ProjectionOpDesktopState, Body: desktopBody},
			{Kind: computerevent.ProjectionOpObject, CanonicalID: "obj-a", Body: objectBody},
			{Kind: computerevent.ProjectionOpObjectEdge, CanonicalID: "edge-a", Body: edgeBody},
		},
	}
	if err := productStore.FinalizeBatch(ctx, computerID, digest, signedReceipt(t, computerID, digest, event.Sequence), &batch); err != nil {
		t.Fatal(err)
	}

	assertCount(t, productStore, `SELECT COUNT(*) FROM desktop_workspaces`, 1)
	assertCount(t, productStore, `SELECT COUNT(*) FROM desktop_app_instances`, 1)
	assertCount(t, productStore, `SELECT COUNT(*) FROM desktop_window_placements`, 1)
	assertCount(t, productStore, `SELECT COUNT(*) FROM og_objects`, 1)
	assertCount(t, productStore, `SELECT COUNT(*) FROM og_edges`, 1)
	assertCount(t, productStore, `SELECT COUNT(*) FROM desktop_sessions`, 0)

	state, err := productStore.GetDesktopStateForSession(ctx, "owner-1", types.PrimaryDesktopID, "projected")
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Windows) != 1 || state.Windows[0].WindowID != "win-a" {
		t.Fatalf("projected desktop=%+v", state)
	}
	_ = genesisDigest
}

func TestProjectBatchReplacesDesktopAndOGOnLaterHead(t *testing.T) {
	ctx := context.Background()
	productStore := openProjectStore(t)
	computerID := "computer-project-replace"
	prepareGenesis(t, productStore, computerID, "genesis-replace")
	head, err := productStore.Head(ctx, computerID)
	if err != nil {
		t.Fatal(err)
	}
	firstID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	desktopBody, objectBody, edgeBody := projectBatchBodies(t, "win-a", "obj-a", "edge-a")
	event, digest := prepareProjectionEvent(t, productStore, *head, firstID, "project-first")
	batch := computerevent.ProjectionBatch{
		Version: computerevent.ProjectionBatchV1, ProjectorVersion: computerevent.ProjectorVersionV1,
		ComputerID: computerID, EventID: event.EventID, EventDigest: digest,
		Ops: []computerevent.ProjectionOp{
			{Kind: computerevent.ProjectionOpDesktopState, Body: desktopBody},
			{Kind: computerevent.ProjectionOpObject, Body: objectBody},
			{Kind: computerevent.ProjectionOpObjectEdge, Body: edgeBody},
		},
	}
	if err := productStore.FinalizeBatch(ctx, computerID, digest, signedReceipt(t, computerID, digest, event.Sequence), &batch); err != nil {
		t.Fatal(err)
	}
	head, err = productStore.Head(ctx, computerID)
	if err != nil {
		t.Fatal(err)
	}
	secondID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	desktopBody2, objectBody2, edgeBody2 := projectBatchBodies(t, "win-b", "obj-b", "edge-b")
	event2, digest2 := prepareProjectionEvent(t, productStore, *head, secondID, "project-second")
	batch2 := computerevent.ProjectionBatch{
		Version: computerevent.ProjectionBatchV1, ProjectorVersion: computerevent.ProjectorVersionV1,
		ComputerID: computerID, EventID: event2.EventID, EventDigest: digest2,
		Ops: []computerevent.ProjectionOp{
			{Kind: computerevent.ProjectionOpDesktopState, Body: desktopBody2},
			{Kind: computerevent.ProjectionOpObject, Body: objectBody2},
			{Kind: computerevent.ProjectionOpObjectEdge, Body: edgeBody2},
		},
	}
	if err := productStore.FinalizeBatch(ctx, computerID, digest2, signedReceipt(t, computerID, digest2, event2.Sequence), &batch2); err != nil {
		t.Fatal(err)
	}
	state, err := productStore.GetDesktopStateForSession(ctx, "owner-1", types.PrimaryDesktopID, "projected")
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Windows) != 1 || state.Windows[0].WindowID != "win-b" {
		t.Fatalf("later snapshot not folded: %+v", state)
	}
	assertCount(t, productStore, `SELECT COUNT(*) FROM og_objects`, 2)
}

func TestProjectBatchRefusesSessionPresence(t *testing.T) {
	batch := computerevent.ProjectionBatch{
		Version: computerevent.ProjectionBatchV1, ProjectorVersion: computerevent.ProjectorVersionV1,
		ComputerID: "computer-project", EventID: "event-1", EventDigest: storeTestDigest('a'),
		Ops: []computerevent.ProjectionOp{{
			Kind:  computerevent.ProjectionOpDesktopState,
			Table: "desktop_sessions",
			Body:  json.RawMessage(`{"owner_id":"o","last_input_at":"now"}`),
		}},
	}
	if err := batch.Validate(); !errors.Is(err, computerevent.ErrProjectionPresence) {
		t.Fatalf("presence batch error=%v", err)
	}
}

func TestSaveDesktopStateDoesNotWriteDoltSessions(t *testing.T) {
	productStore := openProjectStore(t)
	ctx := context.Background()
	state := types.DesktopState{
		OwnerID: "user-1",
		Windows: []types.WindowState{{
			WindowID: "win-1", AppID: "texture", Title: "T",
			Geometry: types.WindowGeometry{X: 1, Y: 2, Width: 3, Height: 4},
			Mode:     types.WindowNormal, ZIndex: 1,
		}},
		ActiveWindowID: "win-1",
		UpdatedAt:      time.Now().UTC(),
	}
	if err := productStore.SaveDesktopStateForSession(ctx, state, types.DesktopSessionContext{
		SessionID: "browser-1", IsDriver: true, DriverUntil: time.Now().UTC().Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	assertCount(t, productStore, `SELECT COUNT(*) FROM desktop_sessions`, 0)
	assertCount(t, productStore, `SELECT COUNT(*) FROM desktop_app_instances`, 1)
	if _, ok := productStore.desktopSessionPresence("user-1", types.PrimaryDesktopID, "browser-1"); !ok {
		t.Fatal("in-memory presence missing after driver save")
	}
}

func TestFinalizeWithoutBatchRefusesProjectionKind(t *testing.T) {
	ctx := context.Background()
	productStore := openProjectStore(t)
	computerID := "computer-project-nobatch"
	prepareGenesis(t, productStore, computerID, "genesis-nobatch")
	head, err := productStore.Head(ctx, computerID)
	if err != nil {
		t.Fatal(err)
	}
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	event, digest := prepareProjectionEvent(t, productStore, *head, eventID, "missing-batch")
	err = productStore.Finalize(ctx, computerID, digest, signedReceipt(t, computerID, digest, event.Sequence))
	if err == nil {
		t.Fatal("head-only finalize of projection_batch_recorded was admitted")
	}
}

func openProjectStore(t *testing.T) *Store {
	t.Helper()
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	productStore, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = productStore.Close() })
	return productStore
}

func prepareGenesis(t *testing.T, productStore *Store, computerID, idempotency string) (computerevent.Event, string) {
	t.Helper()
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: computerID,
		Sequence: 1, PreviousHead: computerevent.ZeroHead, EventKind: computerevent.EventGenesisImported,
		OccurredAt:     time.Date(2026, 8, 16, 17, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		IdempotencyKey: idempotency, ActorProfile: "trusted-core", AuthorityRef: "authority:test",
		PayloadCommitment: storeTestDigest('a'), PrivacyClass: "private", ReducerVersion: computerevent.ReducerVersionV1,
		ExpectedDesiredEventHead: computerevent.ZeroHead, ExpectedEffectiveEventHead: computerevent.ZeroHead,
		ExpectedDesiredStateCommitment: computerevent.ZeroHead, ExpectedEffectiveStateCommitment: computerevent.ZeroHead,
		ResultingEffectiveCommitment: storeTestDigest('b'),
	}
	input := computerevent.TransitionInput{TargetStateCommitment: storeTestDigest('b')}
	pinIntent, err := computerevent.ComputePinIntentCommitment(event, input)
	if err != nil {
		t.Fatal(err)
	}
	event.RequestCommitment, err = computerevent.ComputeRequestCommitment(event, input, pinIntent, nil)
	if err != nil {
		t.Fatal(err)
	}
	next, err := computerevent.Reduce(nil, event, input)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := event.Digest()
	if err != nil {
		t.Fatal(err)
	}
	request := computerevent.CASRequest{
		Event: event, EventDigest: digest, EventArtifactDigest: digest,
		EventPinReceiptDigest: storeTestDigest('c'), Input: input, Next: next, PinIntentCommitment: pinIntent,
	}
	if err := productStore.Prepare(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if err := productStore.Finalize(context.Background(), computerID, digest, signedReceipt(t, computerID, digest, 1)); err != nil {
		t.Fatal(err)
	}
	return event, digest
}

func prepareProjectionEvent(t *testing.T, productStore *Store, current computerevent.Head, eventID, idempotency string) (computerevent.Event, string) {
	t.Helper()
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: current.ComputerID,
		Sequence: current.Sequence + 1, PreviousHead: current.CanonicalEventHead,
		EventKind:      computerevent.EventProjectionBatchRecorded,
		OccurredAt:     time.Date(2026, 8, 16, 17, 1, 0, 0, time.UTC).Add(time.Duration(current.Sequence) * time.Second).UTC().Format(time.RFC3339Nano),
		IdempotencyKey: idempotency, ActorProfile: "trusted-core", AuthorityRef: "authority:test",
		PayloadCommitment: storeTestDigest('d'), PrivacyClass: "public", ReducerVersion: computerevent.ReducerVersionV1,
		ExpectedDesiredEventHead: current.DesiredEventHead, ExpectedEffectiveEventHead: current.EffectiveEventHead,
		ExpectedPendingTransitionRef:   current.PendingTransitionRef,
		ExpectedDesiredStateCommitment: current.DesiredStateCommitment, ExpectedEffectiveStateCommitment: current.EffectiveStateCommitment,
	}
	input := computerevent.TransitionInput{}
	pinIntent, err := computerevent.ComputePinIntentCommitment(event, input)
	if err != nil {
		t.Fatal(err)
	}
	event.RequestCommitment, err = computerevent.ComputeRequestCommitment(event, input, pinIntent, nil)
	if err != nil {
		t.Fatal(err)
	}
	next, err := computerevent.Reduce(&current, event, input)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := event.Digest()
	if err != nil {
		t.Fatal(err)
	}
	request := computerevent.CASRequest{
		Event: event, EventDigest: digest, EventArtifactDigest: digest,
		EventPinReceiptDigest: storeTestDigest('e'), Input: input, Next: next, PinIntentCommitment: pinIntent,
	}
	if err := productStore.Prepare(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	return event, digest
}

func projectBatchBodies(t *testing.T, windowID, objectID, edgeID string) (json.RawMessage, json.RawMessage, json.RawMessage) {
	t.Helper()
	now := time.Date(2026, 8, 16, 17, 2, 0, 0, time.UTC)
	desktop, err := json.Marshal(projectedDesktopState{
		OwnerID: "owner-1", DesktopID: types.PrimaryDesktopID, ActiveWindowID: windowID, UpdatedAt: now,
		CreatedBySessionID: "projected",
		Windows: []types.WindowState{{
			WindowID: windowID, AppID: "texture", Title: "Texture",
			Geometry: types.WindowGeometry{X: 10, Y: 20, Width: 300, Height: 200},
			Mode:     types.WindowNormal, ZIndex: 1,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	object, err := json.Marshal(objectgraph.Object{
		CanonicalID: objectID, ObjectKind: "choir.texture_revision", OwnerID: "owner-1",
		ComputerID: "computer-project", VersionID: "v1", ContentHash: storeTestDigest('f'),
		Body: []byte(`{"text":"ok"}`), Metadata: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	edge, err := json.Marshal(objectgraph.Edge{
		EdgeID: edgeID, FromID: objectID, ToID: "doc-1", Kind: "cites",
		Metadata: json.RawMessage(`{}`), CreatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	return desktop, object, edge
}

func signedReceipt(t *testing.T, computerID, digest string, sequence uint64) computerevent.Receipt {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := computerevent.NewSignedReceipt("EventHeadReceipt", "corpusd", map[string]any{
		"computer_id": computerID, "event_digest": digest, "sequence": sequence,
	}, []computerevent.SigningKey{{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "test"}, PrivateKey: privateKey}}, time.Date(2026, 8, 16, 17, 3, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

func assertCount(t *testing.T, productStore *Store, query string, want int) {
	t.Helper()
	var got int
	if err := productStore.db.QueryRow(query).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s = %d, want %d", query, got, want)
	}
}

func TestLiveDesktopAndOGWritersAppendProjectTogether(t *testing.T) {
	ctx := context.Background()
	productStore := openProjectStore(t)
	computerID := "computer-live-tape"
	prepareGenesis(t, productStore, computerID, "genesis-live-tape")
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "test"}, PrivateKey: privateKey}
	appender, err := computerevent.NewComputerEventAppender(
		computerID,
		liveTapePinner{signer: signer},
		productStore,
		liveTapeCAS{signer: signer, projection: productStore},
		liveTapeVerifier{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := productStore.BindProjectionTape(computerID, appender); err != nil {
		t.Fatal(err)
	}

	state := types.DesktopState{
		OwnerID: "owner-1",
		Windows: []types.WindowState{{
			WindowID: "win-live", AppID: "texture", Title: "Live",
			Geometry: types.WindowGeometry{X: 4, Y: 5, Width: 6, Height: 7},
			Mode:     types.WindowNormal, ZIndex: 1,
		}},
		ActiveWindowID: "win-live",
		UpdatedAt:      time.Date(2026, 8, 16, 18, 0, 0, 0, time.UTC),
	}
	if err := productStore.SaveDesktopStateForSession(ctx, state, types.DesktopSessionContext{
		SessionID: "browser-live", IsDriver: true, DriverUntil: time.Now().UTC().Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	obj := objectgraph.Object{
		CanonicalID: "obj:choir.texture_revision:owner-1:live",
		ObjectKind:  "choir.texture_revision", OwnerID: "owner-1", ComputerID: computerID,
		VersionID: "v1", ContentHash: storeTestDigest('9'),
		Body: []byte(`{"text":"live"}`), Metadata: json.RawMessage(`{}`),
		CreatedAt: time.Date(2026, 8, 16, 18, 1, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 8, 16, 18, 1, 0, 0, time.UTC),
	}
	if err := productStore.ogStore.PutObject(ctx, obj); err != nil {
		t.Fatal(err)
	}

	assertCount(t, productStore, `SELECT COUNT(*) FROM desktop_workspaces`, 1)
	assertCount(t, productStore, `SELECT COUNT(*) FROM desktop_app_instances`, 1)
	assertCount(t, productStore, `SELECT COUNT(*) FROM desktop_window_placements`, 1)
	assertCount(t, productStore, `SELECT COUNT(*) FROM og_objects`, 1)
	assertCount(t, productStore, `SELECT COUNT(*) FROM desktop_sessions`, 0)

	got, err := productStore.GetDesktopStateForSession(ctx, "owner-1", types.PrimaryDesktopID, "browser-live")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Windows) != 1 || got.Windows[0].WindowID != "win-live" {
		t.Fatalf("live desktop=%+v", got)
	}
	stored, err := productStore.ogStore.GetObject(ctx, obj.CanonicalID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.CanonicalID != obj.CanonicalID {
		t.Fatalf("live object=%+v", stored)
	}
}

type liveTapePinner struct{ signer computerevent.SigningKey }

func (p liveTapePinner) PinEvent(_ context.Context, computerID string, canonicalEvent []byte, requestCommitment string) (computerevent.PinResult, error) {
	digest := computerevent.DigestBytes(canonicalEvent)
	receipt, err := computerevent.NewSignedReceipt("PinReceipt", "corpusd", map[string]any{
		"computer_id": computerID, "artifact_digest": digest, "request_commitment": requestCommitment,
	}, []computerevent.SigningKey{p.signer}, time.Date(2026, 8, 16, 18, 2, 0, 0, time.UTC))
	return computerevent.PinResult{ArtifactDigest: digest, Receipt: receipt}, err
}

func (p liveTapePinner) PinNonPrivatePayload(_ context.Context, computerID string, payload []byte, mediaType, privacyClass, pinIntentCommitment string) (computerevent.PinResult, error) {
	digest := computerevent.DigestBytes(payload)
	receipt, err := computerevent.NewSignedReceipt("PinReceipt", "corpusd", map[string]any{
		"computer_id": computerID, "artifact_digest": digest, "media_type": mediaType,
		"privacy_class": privacyClass, "pin_intent_commitment": pinIntentCommitment,
	}, []computerevent.SigningKey{p.signer}, time.Date(2026, 8, 16, 18, 2, 0, 0, time.UTC))
	return computerevent.PinResult{ArtifactDigest: digest, Receipt: receipt}, err
}

type liveTapeCAS struct {
	signer     computerevent.SigningKey
	projection computerevent.ProjectionStore
}

func (c liveTapeCAS) Head(ctx context.Context, computerID string) (*computerevent.Head, error) {
	return c.projection.Head(ctx, computerID)
}

func (c liveTapeCAS) CompareAndSwap(_ context.Context, request computerevent.CASRequest) (computerevent.Receipt, error) {
	return computerevent.NewSignedReceipt("EventHeadReceipt", "corpusd", map[string]any{
		"computer_id": request.Event.ComputerID, "event_digest": request.EventDigest, "sequence": request.Event.Sequence,
	}, []computerevent.SigningKey{c.signer}, time.Date(2026, 8, 16, 18, 3, 0, 0, time.UTC))
}

type liveTapeVerifier struct{}

func (liveTapeVerifier) VerifyEventHeadReceipt(context.Context, computerevent.Receipt, computerevent.CASRequest) error {
	return nil
}
