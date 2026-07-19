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

func TestSelfDevelopmentDecisionRecoversAfterCanonicalAppendBeforeOperationProjection(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-decision-recovery"
	productStore, err := choirstore.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer productStore.Close()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signingKey := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "recovery-test"}, PrivateKey: privateKey}
	appender, err := computerevent.NewComputerEventAppender(computerID, rollbackTestPinner{signingKey}, productStore, rollbackTestCAS{key: signingKey, projection: productStore}, rollbackTestReceiptVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	genesisID, _ := computerevent.NewEventID()
	genesis := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: genesisID, ComputerID: computerID,
		EventKind: computerevent.EventGenesisImported, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: "recovery-genesis", ActorProfile: "super", AuthorityRef: "owner", PrivacyClass: "owner",
		PayloadCommitment: strings.Repeat("a", 64), ProposedEffectRef: strings.Repeat("b", 64),
		ResultingEffectiveCommitment: strings.Repeat("a", 64), ReducerVersion: computerevent.ReducerVersionV1,
	}
	if _, err := appender.AppendNew(ctx, genesis, computerevent.TransitionInput{TargetStateCommitment: strings.Repeat("a", 64)}, nil); err != nil {
		t.Fatal(err)
	}
	operations, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	operation, err := operations.Start(ctx, selfdev.StartRequest{
		ComputerID: computerID, IdempotencyKey: "recovery-operation",
		PromptArtifactRef: "artifact:sha256:" + strings.Repeat("c", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	bundleDigest := strings.Repeat("d", 64)
	operation, err = operations.Transition(ctx, computerID, operation.OperationID, selfdev.StateRequested, selfdev.StateExecuting, func(next *selfdev.Operation) error {
		next.CapsuleID = "capsule-recovery"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	operation, err = operations.Transition(ctx, computerID, operation.OperationID, selfdev.StateExecuting, selfdev.StateFrozen, func(next *selfdev.Operation) error {
		next.BundleDigest = bundleDigest
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	operation, err = operations.Transition(ctx, computerID, operation.OperationID, selfdev.StateFrozen, selfdev.StateVerified, func(next *selfdev.Operation) error {
		next.VerifierRefs = []string{strings.Repeat("e", 64)}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	operation, err = operations.Transition(ctx, computerID, operation.OperationID, selfdev.StateVerified, selfdev.StateAwaitingApproval, nil)
	if err != nil {
		t.Fatal(err)
	}
	head, err := productStore.Head(ctx, computerID)
	if err != nil || head == nil {
		t.Fatal("decision recovery head unavailable")
	}
	decisionID, _ := computerevent.NewEventID()
	decision := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: decisionID, ComputerID: computerID,
		EventKind: computerevent.EventEffectAccepted, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: "recovery-decision", RequestCommitment: computerevent.ZeroHead,
		TrajectoryID: operation.TrajectoryID, CapsuleID: operation.CapsuleID, PreviousHead: head.CanonicalEventHead,
		ParentEventID: operation.OperationID,
		ActorProfile:  "super", AuthorityRef: "external-owner:owner", PrivacyClass: "owner",
		ExpectedDesiredEventHead: head.DesiredEventHead, ExpectedEffectiveEventHead: head.EffectiveEventHead,
		ExpectedDesiredStateCommitment: head.DesiredStateCommitment, ExpectedEffectiveStateCommitment: head.EffectiveStateCommitment,
		RequireExpectedHead: true, PayloadCommitment: computerevent.ZeroHead, ProposedEffectRef: bundleDigest,
		DecisionRef: strings.Repeat("d", 64), VerifierRefs: []string{strings.Repeat("e", 64)}, ReducerVersion: computerevent.ReducerVersionV1,
	}
	target, err := computerevent.CanonicalJSON(map[string]string{"base_head": operation.BaseHead, "bundle_digest": bundleDigest})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := appender.AppendNew(ctx, decision, computerevent.TransitionInput{TargetStateCommitment: computerevent.DigestBytes(target)}, nil); err != nil {
		t.Fatal(err)
	}
	stale, err := operations.Get(ctx, computerID, operation.OperationID)
	if err != nil || stale.State != selfdev.StateAwaitingApproval || stale.DecisionEvent != "" {
		t.Fatalf("crash window was not reproduced: %+v err=%v", stale, err)
	}
	runtime := &Runtime{store: productStore, selfdevOperations: operations}
	recovered, found, err := runtime.recoverSelfDevelopmentDecision(ctx, stale)
	if err != nil || !found {
		t.Fatalf("recover decision: found=%v err=%v", found, err)
	}
	storedDecision, found, err := productStore.EventByIdempotency(ctx, computerID, decision.IdempotencyKey)
	if err != nil || !found {
		t.Fatalf("stored decision unavailable: found=%v err=%v", found, err)
	}
	decisionDigest, _ := storedDecision.Digest()
	if recovered.State != selfdev.StateAccepted || recovered.DecisionEvent != decisionDigest || recovered.DecisionActor != "owner" || recovered.DecisionReceipt == "" {
		t.Fatalf("recovered operation = %+v", recovered)
	}
	replayed, found, err := runtime.recoverSelfDevelopmentDecision(ctx, recovered)
	if err != nil || !found || replayed.DecisionEvent != decisionDigest {
		t.Fatalf("idempotent recovery = %+v found=%v err=%v", replayed, found, err)
	}
}

func TestGenesisAuthoritySeparatesReviewedCandidateFromDeployedRelease(t *testing.T) {
	request := selfDevelopmentGenesisRequest{
		G0Receipt: "g0-receipt", G1Receipt: "g1-receipt",
		CandidateRef: "reviewed-candidate", DeployedReleaseRef: "deployed-release",
	}
	ref, err := selfDevelopmentGenesisAuthorityRef(request, "g0-receipt", "g1-receipt", "reviewed-candidate", "deployed-release")
	if err != nil || !strings.HasPrefix(ref, "genesis-authority:sha256:") {
		t.Fatalf("separate candidate/deployed binding refused: ref=%q err=%v", ref, err)
	}
	changed := request
	changed.DeployedReleaseRef = changed.CandidateRef
	if _, err := selfDevelopmentGenesisAuthorityRef(changed, "g0-receipt", "g1-receipt", "reviewed-candidate", "deployed-release"); err == nil {
		t.Fatal("genesis accepted reviewed candidate as the deployed release")
	}
	changed = request
	changed.CandidateRef = changed.DeployedReleaseRef
	if _, err := selfDevelopmentGenesisAuthorityRef(changed, "g0-receipt", "g1-receipt", "reviewed-candidate", "deployed-release"); err == nil {
		t.Fatal("genesis accepted deployed release as the reviewed candidate")
	}
}

func TestExactTerminalDecisionReplayDoesNotDependOnLaterCurrentMode(t *testing.T) {
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	decisionRef := strings.Repeat("d", 64)
	publicDecision := selfDevelopmentDecisionRequest{
		Decision: "reject", IdempotencyKey: "decision-replay", BundleDigest: strings.Repeat("a", 64),
		VerifierRef: strings.Repeat("1", 64), Reason: "owner rejected",
		ExpectedDesiredEventHead: strings.Repeat("a", 64), ExpectedEffectiveEventHead: strings.Repeat("b", 64),
		ExpectedDesiredStateCommitment: strings.Repeat("c", 64), ExpectedEffectiveStateCommitment: strings.Repeat("c", 64),
	}
	pending := ""
	publicDecision.ExpectedPendingTransitionRef = &pending
	withReceipt := publicDecision
	withReceipt.ModeReceipt = &computerevent.Receipt{ReceiptKind: "ModeReceipt", ReceiptID: "mode-receipt"}
	publicRef, err := selfDevelopmentDecisionRef(publicDecision)
	if err != nil {
		t.Fatal(err)
	}
	proxiedRef, err := selfDevelopmentDecisionRef(withReceipt)
	if err != nil {
		t.Fatal(err)
	}
	if publicRef != proxiedRef {
		t.Fatal("proxy-injected mode receipt changed the public decision identity")
	}
	decisionRef = publicRef
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: "computer-replay",
		Sequence: 1, PreviousHead: computerevent.ZeroHead, EventKind: computerevent.EventEffectRejected,
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), IdempotencyKey: "decision-replay", RequestCommitment: computerevent.ZeroHead,
		TrajectoryID: "trajectory-replay", CapsuleID: "capsule-replay", ParentEventID: "operation-replay",
		ActorProfile: "super", AuthorityRef: "external-owner:owner", PrivacyClass: "owner",
		ExpectedDesiredEventHead: strings.Repeat("a", 64), ExpectedEffectiveEventHead: strings.Repeat("b", 64),
		ExpectedDesiredStateCommitment: strings.Repeat("c", 64), ExpectedEffectiveStateCommitment: strings.Repeat("c", 64),
		RequireExpectedHead: true, PayloadCommitment: strings.Repeat("e", 64),
		ProposedEffectRef: publicDecision.BundleDigest, DecisionRef: decisionRef,
		VerifierRefs: []string{strings.Repeat("1", 64)}, ReducerVersion: computerevent.ReducerVersionV1,
	}
	if !exactSelfDevelopmentDecisionRequestMatches(event, "computer-replay", "operation-replay", "owner", computerevent.EventEffectRejected, decisionRef, publicDecision) {
		t.Fatal("exact terminal retry was not recognized before current-mode authorization")
	}
	inconsistent := event
	inconsistent.ProposedEffectRef = strings.Repeat("2", 64)
	if exactSelfDevelopmentDecisionRequestMatches(inconsistent, "computer-replay", "operation-replay", "owner", computerevent.EventEffectRejected, decisionRef, publicDecision) {
		t.Fatal("semantically inconsistent terminal projection was accepted")
	}
}

func TestGuestStartRefusesAbsentModeBeforeAnyEffect(t *testing.T) {
	computerID := "computer-mode-off"
	productStore, err := choirstore.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer productStore.Close()
	operations, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	handler := &APIHandler{rt: &Runtime{
		cfg: provideriface.Config{SandboxID: computerID}, store: productStore, selfdevOperations: operations,
	}}
	body, err := json.Marshal(selfDevelopmentStartRequest{IdempotencyKey: "mode-off-start", Prompt: "change runtime"})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/computers/"+computerID+"/self-development/operations", strings.NewReader(string(body)))
	request.Header.Set("X-Authenticated-User", "owner")
	request.Header.Set("X-Authenticated-Computer", computerID)
	response := httptest.NewRecorder()
	handler.HandleComputersRouter(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("absent-mode start status=%d body=%s", response.Code, response.Body.String())
	}
	if _, found, err := operations.GetByIdempotency(context.Background(), computerID, "mode-off-start"); err != nil || found {
		t.Fatalf("absent-mode start created operation: found=%v err=%v", found, err)
	}
	if event, found, err := productStore.EventByIdempotency(context.Background(), computerID, "selfdev-start-"+computerevent.DigestBytes([]byte(computerID+"\x00mode-off-start"))); err != nil || found {
		t.Fatalf("absent-mode start appended event: event=%+v found=%v err=%v", event, found, err)
	}
}

func TestRecoveredStartEventRequiresExactCausalBinding(t *testing.T) {
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, ComputerID: "computer-crash",
		EventKind: computerevent.EventTrajectoryStarted, TrajectoryID: "trajectory-crash",
		IdempotencyKey: "selfdev-start-crash", RequestCommitment: strings.Repeat("a", 64),
		AuthorityRef: "public-self-development-api:owner", PrivacyClass: "private",
		OutputArtifactRefs: []string{"artifact:sha256:" + strings.Repeat("b", 64)},
	}
	ref, err := recoveredStartPromptRef(event, "computer-crash", "trajectory-crash", "selfdev-start-crash", "owner")
	if err != nil || ref != event.OutputArtifactRefs[0] {
		t.Fatalf("exact recovered event ref=%q err=%v", ref, err)
	}
	event.AuthorityRef = "public-self-development-api:other"
	if _, err := recoveredStartPromptRef(event, "computer-crash", "trajectory-crash", "selfdev-start-crash", "owner"); err == nil {
		t.Fatal("changed trajectory authority recovered the old event")
	}
}

func TestFinalizedDecisionBindingRejectsCrossAuthorityJoinsAndAllowsAcceptedDescendants(t *testing.T) {
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: "computer-binding",
		Sequence: 1, PreviousHead: computerevent.ZeroHead, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		EventKind: computerevent.EventEffectAccepted, IdempotencyKey: "decision-binding", RequestCommitment: strings.Repeat("1", 64),
		TrajectoryID: "trajectory-binding", CapsuleID: "capsule-binding", ParentEventID: "operation-binding",
		ActorProfile: "super", AuthorityRef: "external-owner:owner-binding", PrivacyClass: "owner",
		ExpectedDesiredEventHead: strings.Repeat("9", 64), ExpectedEffectiveEventHead: strings.Repeat("a", 64),
		ExpectedDesiredStateCommitment: strings.Repeat("b", 64), ExpectedEffectiveStateCommitment: strings.Repeat("c", 64),
		RequireExpectedHead: true,
		PayloadCommitment:   computerevent.ZeroHead, ProposedEffectRef: strings.Repeat("2", 64), DecisionRef: strings.Repeat("3", 64),
		VerifierRefs: []string{strings.Repeat("4", 64)}, ReducerVersion: computerevent.ReducerVersionV1,
	}
	eventDigest, err := event.Digest()
	if err != nil {
		t.Fatal(err)
	}
	transition := computerevent.DurableEvent{
		Request: computerevent.CASRequest{
			Event: event, EventDigest: eventDigest,
			Next: computerevent.Head{DesiredEventHead: strings.Repeat("5", 64), EffectiveEventHead: strings.Repeat("6", 64)},
		},
		Receipt: computerevent.Receipt{ReceiptKind: "EventHeadReceipt", ReceiptID: "receipt-binding", KindFields: map[string]any{"event_digest": eventDigest}},
	}
	operation := selfdev.Operation{
		OperationID: event.ParentEventID, ComputerID: event.ComputerID, TrajectoryID: event.TrajectoryID,
		CapsuleID: event.CapsuleID, BundleDigest: event.ProposedEffectRef, VerifierRefs: append([]string(nil), event.VerifierRefs...),
		DecisionActor: "owner-binding", DecisionEvent: eventDigest, DecisionReceipt: transition.Receipt.ReceiptID,
		DesiredHead: transition.Request.Next.DesiredEventHead, EffectiveHead: transition.Request.Next.EffectiveEventHead,
		State: selfdev.StateMaterializing,
	}
	if _, err := verifyFinalizedSelfDevelopmentDecision(operation, transition); err != nil {
		t.Fatalf("accepted descendant refused: %v", err)
	}
	for name, mutate := range map[string]func(*selfdev.Operation, *computerevent.DurableEvent){
		"actor": func(_ *selfdev.Operation, durable *computerevent.DurableEvent) {
			durable.Request.Event.AuthorityRef = "external-owner:other"
		},
		"capsule": func(op *selfdev.Operation, _ *computerevent.DurableEvent) {
			op.CapsuleID = "capsule-other"
		},
		"verifier": func(_ *selfdev.Operation, durable *computerevent.DurableEvent) {
			durable.Request.Event.VerifierRefs = []string{strings.Repeat("7", 64)}
		},
		"receipt": func(_ *selfdev.Operation, durable *computerevent.DurableEvent) {
			durable.Receipt.KindFields = map[string]any{"event_digest": strings.Repeat("8", 64)}
		},
	} {
		t.Run(name, func(t *testing.T) {
			changedOperation := operation
			changedTransition := transition
			mutate(&changedOperation, &changedTransition)
			if _, err := verifyFinalizedSelfDevelopmentDecision(changedOperation, changedTransition); err == nil {
				t.Fatal("cross-authority decision join was accepted")
			}
		})
	}
	rejected := transition
	rejected.Request.Event.EventKind = computerevent.EventEffectRejected
	rejectedDigest, err := rejected.Request.Event.Digest()
	if err != nil {
		t.Fatal(err)
	}
	rejected.Request.EventDigest = rejectedDigest
	rejected.Receipt.KindFields = map[string]any{"event_digest": rejectedDigest}
	if _, err := verifyFinalizedSelfDevelopmentDecision(operation, rejected); err == nil {
		t.Fatal("rejected decision was accepted as an applied descendant")
	}
}
