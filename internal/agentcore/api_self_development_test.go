package agentcore

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

type rollbackTestPinner struct{ key computerevent.SigningKey }

func (p rollbackTestPinner) PinEvent(_ context.Context, computerID string, canonical []byte, requestCommitment string) (computerevent.PinResult, error) {
	digest := computerevent.DigestBytes(canonical)
	receipt, err := computerevent.NewSignedReceipt("PinReceipt", "corpusd", map[string]any{"computer_id": computerID, "artifact_digest": digest, "request_commitment": requestCommitment}, []computerevent.SigningKey{p.key}, time.Now().UTC())
	return computerevent.PinResult{ArtifactDigest: digest, Receipt: receipt}, err
}

type rollbackTestCAS struct {
	key        computerevent.SigningKey
	projection computerevent.ProjectionStore
}

func (c rollbackTestCAS) Head(ctx context.Context, computerID string) (*computerevent.Head, error) {
	return c.projection.Head(ctx, computerID)
}
func (c rollbackTestCAS) CompareAndSwap(_ context.Context, request computerevent.CASRequest) (computerevent.Receipt, error) {
	return computerevent.NewSignedReceipt("EventHeadReceipt", "corpusd", map[string]any{"event_digest": request.EventDigest}, []computerevent.SigningKey{c.key}, time.Now().UTC())
}

type rollbackTestReceiptVerifier struct{}

func (rollbackTestReceiptVerifier) VerifyEventHeadReceipt(context.Context, computerevent.Receipt, computerevent.CASRequest) error {
	return nil
}

func TestSelfDevelopmentRollbackCreatesOneHeadBoundPendingOperation(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-rollback"
	productStore, err := choirstore.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer productStore.Close()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signingKey := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "test"}, PrivateKey: privateKey}
	appender, err := computerevent.NewComputerEventAppender(computerID, rollbackTestPinner{signingKey}, productStore, rollbackTestCAS{key: signingKey, projection: productStore}, rollbackTestReceiptVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	genesisID, _ := computerevent.NewEventID()
	genesis := computerevent.Event{SchemaVersion: 1, EventID: genesisID, ComputerID: computerID, EventKind: computerevent.EventGenesisImported, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), IdempotencyKey: "genesis", ActorProfile: "super", AuthorityRef: "owner", PrivacyClass: "owner", PayloadCommitment: strings.Repeat("a", 64), ProposedEffectRef: strings.Repeat("b", 64), ResultingEffectiveCommitment: strings.Repeat("a", 64), ReducerVersion: 1}
	if _, err := appender.AppendNew(ctx, genesis, computerevent.TransitionInput{TargetStateCommitment: strings.Repeat("a", 64)}, nil); err != nil {
		t.Fatal(err)
	}
	genesis, found, err := productStore.EventByIdempotency(ctx, computerID, "genesis")
	if err != nil || !found {
		t.Fatal("genesis event unavailable")
	}
	genesisDigest, _ := genesis.Digest()
	operations, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	target, err := operations.RecordAppliedBaseline(ctx, selfdev.BaselineRequest{
		ComputerID: computerID, IdempotencyKey: "baseline", EventHead: genesisDigest, StateCommitment: strings.Repeat("a", 64),
		ReleaseDigest: strings.Repeat("c", 64), CodeRef: "code:baseline", ArtifactProgramRef: "artifact:baseline",
		VerifierRefs: []string{genesisDigest}, MaterializationReceipt: strings.Repeat("d", 64),
		CheckpointRef: "checkpoint:sha256:" + strings.Repeat("e", 64), RouteReceipt: "route-receipt-baseline", RouteGeneration: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	updateID, _ := computerevent.NewEventID()
	update := computerevent.Event{SchemaVersion: 1, EventID: updateID, ComputerID: computerID, EventKind: computerevent.EventResearcherUpdate, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), IdempotencyKey: "current-update", ActorProfile: "researcher", AuthorityRef: "typed-update", PayloadCommitment: strings.Repeat("0", 64), PrivacyClass: "owner", ResultingEffectiveCommitment: strings.Repeat("f", 64), ReducerVersion: 1}
	if _, err := appender.AppendNew(ctx, update, computerevent.TransitionInput{TargetStateCommitment: strings.Repeat("f", 64)}, nil); err != nil {
		t.Fatal(err)
	}
	currentHead, _ := productStore.Head(ctx, computerID)

	now := time.Now().UTC()
	makeInputs := func(seed byte) (computerversion.CodeClosure, computerversion.ArtifactProgram) {
		digest := strings.Repeat(string(seed), 64)
		closure, err := computerversion.NewCodeClosure(strings.Repeat(string(seed), 40), []computerversion.CodeArtifact{{Name: "bundle", SHA256: digest, URI: "artifact+sha256://" + digest + "/bundle"}}, now)
		if err != nil {
			t.Fatal(err)
		}
		program, err := computerversion.NewArtifactProgram([]computerversion.ArtifactProgramEntry{{Kind: "bundle", ContentSHA256: digest, ArtifactURI: "artifact+sha256://" + digest + "/bundle"}}, now)
		if err != nil {
			t.Fatal(err)
		}
		return closure, program
	}
	oldCode, oldProgram := makeInputs('1')
	newCode, newProgram := makeInputs('2')
	oldVersion := computerversion.ComputerVersion{CodeRef: oldCode.Ref, ArtifactProgramRef: oldProgram.Ref}
	newVersion := computerversion.ComputerVersion{CodeRef: newCode.Ref, ArtifactProgramRef: newProgram.Ref}
	slotID, _ := routeledger.RouteSlotID("owner", "primary")
	ledger := routeledger.NewMemoryLedger()
	transition := func(kind routeledger.TransitionKind, old, next computerversion.ComputerVersion, generation uint64, key string) (routeledger.Slot, routeledger.TransitionReceipt) {
		approval, _ := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidenceApproval, slotID, next, json.RawMessage(`{"approval":true}`), now)
		certificate, _ := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidencePromotionCertificate, slotID, next, json.RawMessage(`{"certificate":true}`), now)
		command := routeledger.TransitionCommand{RouteSlotID: slotID, Kind: kind, Old: old, New: next, ExpectedGeneration: generation, ApprovalRef: routeledger.ApprovalRef(approval.Ref), PromotionCertificateRef: routeledger.PromotionCertificateRef(certificate.Ref), IdempotencyKey: routeledger.IdempotencyKey(key)}
		slot, receipt, err := ledger.TransitionWithEvidence(ctx, command, []routeledger.AuthorizationEvidence{approval, certificate})
		if err != nil {
			t.Fatal(err)
		}
		return slot, receipt
	}
	_, _ = transition(routeledger.TransitionBootstrap, computerversion.ComputerVersion{}, oldVersion, 0, "idempotency:bootstrap")
	slot, currentRouteReceipt := transition(routeledger.TransitionPromote, oldVersion, newVersion, 1, "idempotency:promote")
	routeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(vmctl.RouteResolution{Slot: slot, LatestReceipt: currentRouteReceipt, CodeClosure: newCode, ArtifactProgram: newProgram})
	}))
	defer routeServer.Close()
	runtime := &Runtime{cfg: provideriface.Config{SandboxID: computerID}, store: productStore, eventAppender: appender, selfdevOperations: operations, selfdevRoute: vmctl.NewClient(routeServer.URL), selfdevRouteOwnerID: "owner", selfdevRouteDesktopID: "primary"}
	handler := &APIHandler{rt: runtime}
	requestBody := selfDevelopmentRollbackRequest{ExpectedDesiredHead: currentHead.DesiredEventHead, CurrentAppliedHead: currentHead.EffectiveEventHead, ToAppliedHead: genesisDigest, PriorMaterialization: target.MaterializationReceipt, PriorCheckpoint: target.CheckpointRef, ExpectedRouteGeneration: slot.Generation, IdempotencyKey: "rollback-api"}
	body, _ := json.Marshal(requestBody)
	recorder := httptest.NewRecorder()
	handler.startSelfDevelopmentRollback(recorder, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(body))), "owner", computerID)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("rollback status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var operation selfdev.Operation
	if err := json.NewDecoder(recorder.Body).Decode(&operation); err != nil {
		t.Fatal(err)
	}
	resultHead, _ := productStore.Head(ctx, computerID)
	if operation.State != selfdev.StateRollbackPending || operation.BaseHead != genesisDigest || resultHead.PendingTransitionRef == "" || resultHead.EffectiveEventHead != currentHead.EffectiveEventHead {
		t.Fatalf("rollback operation=%+v head=%+v", operation, resultHead)
	}
	replay := httptest.NewRecorder()
	handler.startSelfDevelopmentRollback(replay, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(body))), "owner", computerID)
	var replayed selfdev.Operation
	_ = json.NewDecoder(replay.Body).Decode(&replayed)
	if replay.Code != http.StatusOK || replayed.OperationID != operation.OperationID {
		t.Fatalf("rollback replay status=%d operation=%+v", replay.Code, replayed)
	}
}
