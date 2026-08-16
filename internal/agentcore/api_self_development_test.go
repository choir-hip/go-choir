package agentcore

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/decisionpolicy"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/updater"
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
	runtime := &Runtime{cfg: provideriface.Config{ComputerID: computerID}, store: productStore, eventAppender: appender, selfdevOperations: operations, selfdevRoute: vmctl.NewClient(routeServer.URL), selfdevRouteOwnerID: "owner", selfdevRouteDesktopID: "primary"}
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
		InputArtifactRefs: []string{"artifact:sha256:" + strings.Repeat("f", 64)},
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
	g0 := "sha256:" + strings.Repeat("a", 64)
	g1 := "sha256:" + strings.Repeat("b", 64)
	candidate := strings.Repeat("c", 40)
	deployed := strings.Repeat("d", 40)
	request := selfDevelopmentGenesisRequest{
		G0Receipt: g0, G1Receipt: g1,
		CandidateRef: candidate, DeployedReleaseRef: deployed,
	}
	ref, payload, err := selfDevelopmentGenesisAuthorityRef(request, g0, g1, candidate, deployed)
	if err != nil || !strings.HasPrefix(ref, "artifact:sha256:") || computerevent.DigestBytes(payload) != strings.TrimPrefix(ref, "artifact:sha256:") {
		t.Fatalf("separate candidate/deployed artifact refused: ref=%q err=%v", ref, err)
	}
	changed := request
	changed.DeployedReleaseRef = changed.CandidateRef
	if _, _, err := selfDevelopmentGenesisAuthorityRef(changed, g0, g1, candidate, deployed); err == nil {
		t.Fatal("genesis accepted reviewed candidate as the deployed release")
	}
	changed = request
	changed.CandidateRef = changed.DeployedReleaseRef
	if _, _, err := selfDevelopmentGenesisAuthorityRef(changed, g0, g1, candidate, deployed); err == nil {
		t.Fatal("genesis accepted deployed release as the reviewed candidate")
	}
	for _, test := range []struct {
		name, g0, g1, candidate, deployed string
	}{
		{name: "placeholder G0", g0: "pending_g0_receipt", g1: g1, candidate: candidate, deployed: deployed},
		{name: "placeholder G1", g0: g0, g1: "pending_g1_receipt", candidate: candidate, deployed: deployed},
		{name: "placeholder candidate", g0: g0, g1: g1, candidate: "pending_candidate_ref", deployed: deployed},
		{name: "local release", g0: g0, g1: g1, candidate: candidate, deployed: "local"},
		{name: "non-hex release", g0: g0, g1: g1, candidate: candidate, deployed: strings.Repeat("z", 40)},
	} {
		t.Run(test.name, func(t *testing.T) {
			invalid := selfDevelopmentGenesisRequest{
				G0Receipt: test.g0, G1Receipt: test.g1,
				CandidateRef: test.candidate, DeployedReleaseRef: test.deployed,
			}
			if _, _, err := selfDevelopmentGenesisAuthorityRef(invalid, test.g0, test.g1, test.candidate, test.deployed); err == nil {
				t.Fatal("genesis accepted an unfrozen authority identity")
			}
		})
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
		cfg: provideriface.Config{ComputerID: computerID}, store: productStore, selfdevOperations: operations,
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

func TestOwnerRecoveryControlDoesNotAuthorizeProposal(t *testing.T) {
	computerID := "computer-owner-recovery-only"
	productStore, err := choirstore.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer productStore.Close()
	operations, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	credentials := selfdev.GuestCredentialsWithCapability("http://127.0.0.1:1", computerID, "unused", time.Now().UTC().Add(time.Hour))
	rt := &Runtime{cfg: provideriface.Config{ComputerID: computerID}, store: productStore, selfdevOperations: operations}
	WithOwnerRecoveryControl(credentials)(rt)
	handler := &APIHandler{rt: rt}
	body, err := json.Marshal(selfDevelopmentStartRequest{IdempotencyKey: "owner-recovery-start", Prompt: "change runtime"})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/computers/"+computerID+"/self-development/operations", strings.NewReader(string(body)))
	request.Header.Set("X-Authenticated-User", "owner")
	request.Header.Set("X-Authenticated-Computer", computerID)
	response := httptest.NewRecorder()
	handler.HandleComputersRouter(response, request)
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "self-development mode authority unavailable") {
		t.Fatalf("owner-recovery-only start status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestGuestStartRefusesModeOffBeforeAnyEffect(t *testing.T) {
	computerID := "computer-mode-off-mounted"
	platformServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/internal/computers/self-development/mode" || r.URL.Query().Get("computer_id") != computerID {
			http.Error(w, "unexpected mode probe", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"computer_id":"` + computerID + `","mode":"off","generation":0}`))
	}))
	defer platformServer.Close()
	productStore, err := choirstore.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer productStore.Close()
	operations, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	credentials := selfdev.GuestCredentialsWithCapability(platformServer.URL, computerID, "test-capability", time.Now().UTC().Add(time.Hour))
	rt := &Runtime{cfg: provideriface.Config{ComputerID: computerID}, store: productStore, selfdevOperations: operations}
	WithSelfDevelopmentControl(credentials)(rt)
	handler := &APIHandler{rt: rt}
	body, err := json.Marshal(selfDevelopmentStartRequest{IdempotencyKey: "mode-off-mounted-start", Prompt: "change runtime"})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/computers/"+computerID+"/self-development/operations", strings.NewReader(string(body)))
	request.Header.Set("X-Authenticated-User", "owner")
	request.Header.Set("X-Authenticated-Computer", computerID)
	response := httptest.NewRecorder()
	handler.HandleComputersRouter(response, request)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "does not authorize proposal") {
		t.Fatalf("mode-off start status=%d body=%s", response.Code, response.Body.String())
	}
	if _, found, err := operations.GetByIdempotency(context.Background(), computerID, "mode-off-mounted-start"); err != nil || found {
		t.Fatalf("mode-off start created operation: found=%v err=%v", found, err)
	}
	if event, found, err := productStore.EventByIdempotency(context.Background(), computerID, "selfdev-start-"+computerevent.DigestBytes([]byte(computerID+"\x00mode-off-mounted-start"))); err != nil || found {
		t.Fatalf("mode-off start appended event: event=%+v found=%v err=%v", event, found, err)
	}
}

func TestGuestKernelCapabilitiesRefuseUnmountedAuthority(t *testing.T) {
	computerID := "computer-kernel-unmounted"
	productStore, err := choirstore.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer productStore.Close()
	operations, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	handler := &APIHandler{rt: &Runtime{cfg: provideriface.Config{ComputerID: computerID}, store: productStore, selfdevOperations: operations}}
	request := httptest.NewRequest(http.MethodGet, "/api/computers/"+computerID+"/self-development/kernel-capabilities", nil)
	request.Header.Set("X-Authenticated-User", "owner")
	request.Header.Set("X-Authenticated-Computer", computerID)
	response := httptest.NewRecorder()
	handler.HandleComputersRouter(response, request)
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "kernel capability authority unavailable") {
		t.Fatalf("unmounted kernel status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestGuestKernelCapabilitiesRefuseMissingComputerVersionRoute(t *testing.T) {
	computerID := "computer-kernel-route"
	routeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no route", http.StatusNotFound)
	}))
	defer routeServer.Close()
	productStore, err := choirstore.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer productStore.Close()
	updaterClient, err := updater.NewClient(filepath.Join(t.TempDir(), "updater.sock"))
	if err != nil {
		t.Fatal(err)
	}
	operations, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	rt := &Runtime{cfg: provideriface.Config{ComputerID: computerID}, store: productStore, selfdevOperations: operations}
	WithSelfDevelopmentUpdater(updaterClient, t.TempDir(), computerID, "realization-test")(rt)
	WithSelfDevelopmentRoute(vmctl.NewClient(routeServer.URL), "owner", "primary")(rt)
	handler := &APIHandler{rt: rt}
	request := httptest.NewRequest(http.MethodGet, "/api/computers/"+computerID+"/self-development/kernel-capabilities", nil)
	request.Header.Set("X-Authenticated-User", "owner")
	request.Header.Set("X-Authenticated-Computer", computerID)
	response := httptest.NewRecorder()
	handler.HandleComputersRouter(response, request)
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "computer route identity unavailable") {
		t.Fatalf("missing-route kernel status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRecoveredStartEventRequiresExactCausalBinding(t *testing.T) {
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, ComputerID: "computer-crash",
		EventKind: computerevent.EventTrajectoryStarted, TrajectoryID: "trajectory-crash",
		IdempotencyKey: "selfdev-start-crash", RequestCommitment: strings.Repeat("a", 64),
		DecisionRef:  strings.Repeat("c", 64),
		AuthorityRef: "public-self-development-api:owner", PrivacyClass: "private",
		OutputArtifactRefs: []string{"artifact:sha256:" + strings.Repeat("b", 64)},
	}
	ref, err := recoveredStartPromptRef(event, "computer-crash", "trajectory-crash", "selfdev-start-crash", "owner", strings.Repeat("c", 64))
	if err != nil || ref != event.OutputArtifactRefs[0] {
		t.Fatalf("exact recovered event ref=%q err=%v", ref, err)
	}
	event.AuthorityRef = "public-self-development-api:other"
	if _, err := recoveredStartPromptRef(event, "computer-crash", "trajectory-crash", "selfdev-start-crash", "owner", strings.Repeat("c", 64)); err == nil {
		t.Fatal("changed trajectory authority recovered the old event")
	}
	event.AuthorityRef = "public-self-development-api:owner"
	if _, err := recoveredStartPromptRef(event, "computer-crash", "trajectory-crash", "selfdev-start-crash", "owner", strings.Repeat("d", 64)); err == nil {
		t.Fatal("changed public request commitment recovered the old event")
	}
}

func TestFinalizedStartEventRepairsMissingOperationWithoutCurrentMode(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-start-recovery"
	idempotencyKey := "start-recovery"
	prompt := "recover exact proposal"
	runtime, _ := testRuntime(t)
	productStore := runtime.store
	runtime.cfg.ComputerID = computerID
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signingKey := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "start-recovery"}, PrivateKey: privateKey}
	appender, err := computerevent.NewComputerEventAppender(computerID, rollbackTestPinner{signingKey}, productStore, rollbackTestCAS{key: signingKey, projection: productStore}, rollbackTestReceiptVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	genesisID, _ := computerevent.NewEventID()
	genesis := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: genesisID, ComputerID: computerID,
		EventKind: computerevent.EventGenesisImported, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: "start-recovery-genesis", ActorProfile: "super", AuthorityRef: "owner", PrivacyClass: "owner",
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
	requestCommitment := computerevent.DigestBytes([]byte(computerID + "\x00" + idempotencyKey + "\x00" + computerevent.DigestBytes([]byte(prompt))))
	if err := operations.BindStartIntent(ctx, computerID, idempotencyKey, requestCommitment); err != nil {
		t.Fatal(err)
	}
	identityDigest := computerevent.DigestBytes([]byte(computerID + "\x00" + idempotencyKey))
	eventID, _ := computerevent.NewEventID()
	started := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: computerID,
		EventKind: computerevent.EventTrajectoryStarted, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: "selfdev-start-" + identityDigest, TrajectoryID: "trajectory-" + identityDigest[32:],
		ActorProfile: "super", AuthorityRef: "public-self-development-api:owner", PrivacyClass: "private",
		PayloadCommitment: strings.Repeat("c", 64), ProposedEffectRef: strings.Repeat("c", 64),
		DecisionRef: requestCommitment, OutputArtifactRefs: []string{"artifact:sha256:" + strings.Repeat("c", 64)},
		ReducerVersion: computerevent.ReducerVersionV1,
	}
	if _, err := appender.AppendNew(ctx, started, computerevent.TransitionInput{}, nil); err != nil {
		t.Fatal(err)
	}
	runtime.selfdevOperations = operations
	handler := &APIHandler{rt: runtime}
	body, _ := json.Marshal(selfDevelopmentStartRequest{IdempotencyKey: idempotencyKey, Prompt: prompt})
	httpRequest := httptest.NewRequest(http.MethodPost, "/api/computers/"+computerID+"/self-development/operations", strings.NewReader(string(body)))
	httpRequest.Header.Set("X-Authenticated-User", "owner")
	httpRequest.Header.Set("X-Authenticated-Computer", computerID)
	response := httptest.NewRecorder()
	handler.HandleComputersRouter(response, httpRequest)
	if response.Code != http.StatusOK {
		t.Fatalf("finalized start recovery status=%d body=%s", response.Code, response.Body.String())
	}
	operation, found, err := operations.GetByIdempotency(ctx, computerID, idempotencyKey)
	if err != nil || !found || operation.PromptArtifactRef != started.OutputArtifactRefs[0] || operation.State != selfdev.StateExecuting {
		t.Fatalf("recovered operation=%+v found=%v err=%v", operation, found, err)
	}
	runs, err := productStore.ListRunsBySelfDevelopmentOperation(ctx, "owner", operation.OperationID, 2)
	if err != nil || len(runs) != 1 {
		t.Fatalf("recovered operation runs=%d err=%v", len(runs), err)
	}
}
func TestConcurrentExactRetriesRepairOneRequestedOperationRun(t *testing.T) {
	ctx := context.Background()
	runtime, productStore := testRuntime(t)
	computerID := "computer-requested-retry"
	idempotencyKey := "requested-retry"
	prompt := "repair one durable run"
	runtime.cfg.ComputerID = computerID
	operations, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	runtime.selfdevOperations = operations
	requestCommitment := computerevent.DigestBytes([]byte(computerID + "\x00" + idempotencyKey + "\x00" + computerevent.DigestBytes([]byte(prompt))))
	identityDigest := computerevent.DigestBytes([]byte(computerID + "\x00" + idempotencyKey))
	now := time.Now().UTC().Truncate(time.Microsecond)
	operationID := "selfdev-" + identityDigest[:32]
	if _, err := productStore.DB().ExecContext(ctx, `INSERT INTO self_development_operations (operation_id,computer_id,idempotency_key,request_commitment,trajectory_id,base_head,prompt_artifact_ref,verifier_refs_json,desired_head,effective_head,state,created_at,updated_at) VALUES (?,?,?,?,?,?,?,'[]',?,?,?, ?,?)`,
		operationID, computerID, idempotencyKey, requestCommitment, "trajectory-"+identityDigest[32:], strings.Repeat("a", 64),
		"artifact:sha256:"+strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("c", 64), selfdev.StateRequested, now, now); err != nil {
		t.Fatal(err)
	}
	handler := &APIHandler{rt: runtime}
	body, _ := json.Marshal(selfDevelopmentStartRequest{IdempotencyKey: idempotencyKey, Prompt: prompt})
	var wait sync.WaitGroup
	errs := make(chan string, 16)
	for range 16 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			request := httptest.NewRequest(http.MethodPost, "/api/computers/"+computerID+"/self-development/operations", strings.NewReader(string(body)))
			request.Header.Set("X-Authenticated-User", "owner")
			request.Header.Set("X-Authenticated-Computer", computerID)
			response := httptest.NewRecorder()
			handler.HandleComputersRouter(response, request)
			if response.Code != http.StatusOK {
				errs <- response.Body.String()
			}
		}()
	}
	wait.Wait()
	close(errs)
	for message := range errs {
		t.Fatalf("exact retry failed: %s", message)
	}
	operation, err := operations.Get(ctx, computerID, operationID)
	if err != nil || operation.State != selfdev.StateExecuting {
		t.Fatalf("repaired operation=%+v err=%v", operation, err)
	}
	runs, err := productStore.ListRunsBySelfDevelopmentOperation(ctx, "owner", operationID, 2)
	if err != nil || len(runs) != 1 {
		t.Fatalf("operation-bound runs=%d err=%v", len(runs), err)
	}
	if runs[0].AgentID != persistentSuperAgentID("owner") || runs[0].TrajectoryID != "" ||
		metadataStringValue(runs[0].Metadata, runMetadataTrajectoryID) != "" {
		t.Fatalf("self-development Super was not the non-lifecycle persistent Super: %+v", runs[0])
	}
}

func TestSelfDevelopmentStartRevivesTerminalPersistentSuper(t *testing.T) {
	ctx := context.Background()
	runtime, productStore := testRuntime(t)
	computerID := "computer-selfdev-revive"
	idempotencyKey := "selfdev-revive"
	prompt := "revive persistent Super"
	runtime.cfg.ComputerID = computerID
	operations, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	runtime.selfdevOperations = operations
	requestCommitment := computerevent.DigestBytes([]byte(computerID + "\x00" + idempotencyKey + "\x00" + computerevent.DigestBytes([]byte(prompt))))
	identityDigest := computerevent.DigestBytes([]byte(computerID + "\x00" + idempotencyKey))
	now := time.Now().UTC().Truncate(time.Microsecond)
	operationID := "selfdev-" + identityDigest[:32]
	if _, err := productStore.DB().ExecContext(ctx, `INSERT INTO self_development_operations (operation_id,computer_id,idempotency_key,request_commitment,trajectory_id,base_head,prompt_artifact_ref,verifier_refs_json,desired_head,effective_head,state,created_at,updated_at) VALUES (?,?,?,?,?,?,?,'[]',?,?,?, ?,?)`,
		operationID, computerID, idempotencyKey, requestCommitment, "trajectory-"+identityDigest[32:], strings.Repeat("a", 64),
		"artifact:sha256:"+strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("c", 64), selfdev.StateExecuting, now, now); err != nil {
		t.Fatal(err)
	}
	rec, err := runtime.createRunWithMetadata(ctx, prompt, "owner", map[string]any{
		runMetadataAgentProfile:         agentprofile.Super,
		runMetadataAgentRole:            agentprofile.Super,
		"request_source":                "self_development_operation",
		"self_development_operation_id": operationID,
	})
	if err != nil {
		t.Fatal(err)
	}
	rec.State = types.RunCompleted
	rec.Result = "blocked"
	rec.TrajectoryID = "trajectory-" + identityDigest[32:]
	rec.Metadata = cloneMetadata(rec.Metadata)
	rec.Metadata[runMetadataTrajectoryID] = rec.TrajectoryID
	if err := productStore.UpdateRun(ctx, *rec); err != nil {
		t.Fatal(err)
	}
	handler := &APIHandler{rt: runtime}
	body, _ := json.Marshal(selfDevelopmentStartRequest{IdempotencyKey: idempotencyKey, Prompt: prompt})
	retry := httptest.NewRequest(http.MethodPost, "/api/computers/"+computerID+"/self-development/operations", strings.NewReader(string(body)))
	retry.Header.Set("X-Authenticated-User", "owner")
	retry.Header.Set("X-Authenticated-Computer", computerID)
	retryResponse := httptest.NewRecorder()
	handler.HandleComputersRouter(retryResponse, retry)
	if retryResponse.Code != http.StatusOK {
		t.Fatalf("revive retry status=%d body=%s", retryResponse.Code, retryResponse.Body.String())
	}
	revived, err := productStore.ListRunsBySelfDevelopmentOperation(ctx, "owner", operationID, 2)
	if err != nil || len(revived) != 1 {
		t.Fatalf("revived Super runs=%d err=%v", len(revived), err)
	}
	if revived[0].RunID == rec.RunID || revived[0].TrajectoryID != "" || metadataStringValue(revived[0].Metadata, runMetadataTrajectoryID) != "" {
		t.Fatalf("fresh Super was not started after unbinding terminal Super: old=%s new=%+v", rec.RunID, revived[0])
	}
	if _, err := requirePersistentSuperExecution(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(&revived[0]))); err != nil {
		t.Fatalf("fresh Super failed persistent Super gate: %v rec=%+v", err, revived[0])
	}
	if metadataStringValue(revived[0].Metadata, "assignment_trajectory_id") == "" ||
		metadataStringValue(revived[0].Metadata, "request_source") != "lifecycle_texture_control" ||
		strings.TrimSpace(revived[0].RequestedByRunID) != "" {
		t.Fatalf("fresh Super lacked Texture control join: %+v requested_by_run_id=%q", revived[0].Metadata, revived[0].RequestedByRunID)
	}
	old, err := productStore.GetRunByOwner(ctx, "owner", rec.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if metadataStringValue(old.Metadata, "self_development_operation_id") != "" {
		t.Fatalf("terminal Super stayed bound to the operation: %+v", old.Metadata)
	}
}

func TestSelfDevelopmentPersistentSuperPassesAssignCoSuperGate(t *testing.T) {
	ctx := context.Background()
	runtime, productStore := testRuntime(t)
	operation := selfdev.Operation{
		OperationID:       "selfdev-gate",
		ComputerID:        "computer-selfdev-gate",
		PromptArtifactRef: "artifact:sha256:" + strings.Repeat("b", 64),
	}
	runtime.cfg.ComputerID = operation.ComputerID
	if err := runtime.startSelfDevelopmentPersistentSuper(ctx, operation, "owner", "gate"); err != nil {
		t.Fatal(err)
	}
	runs, err := productStore.ListRunsBySelfDevelopmentOperation(ctx, "owner", operation.OperationID, 2)
	if err != nil || len(runs) != 1 {
		t.Fatalf("started Super runs=%d err=%v", len(runs), err)
	}
	if _, err := requirePersistentSuperExecution(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(&runs[0]))); err != nil {
		t.Fatalf("self-development Super failed persistent Super gate: %v rec=%+v", err, runs[0])
	}
	superAgent, err := productStore.GetAgentByScope(ctx, "owner", operation.ComputerID, persistentSuperAgentID("owner"))
	if err != nil || superAgent.ChannelID != superAgent.AgentID || superAgent.LifecycleVersion != 0 {
		t.Fatalf("persistent Super agent lost non-lifecycle identity: %+v err=%v", superAgent, err)
	}
	if metadataStringValue(runs[0].Metadata, "assignment_trajectory_id") == "" ||
		metadataStringValue(runs[0].Metadata, "request_source") != "lifecycle_texture_control" ||
		strings.TrimSpace(runs[0].RequestedByRunID) != "" ||
		metadataStringValue(runs[0].Metadata, "requested_by_run_id") != "" {
		t.Fatalf("self-development Super lacked Texture control join: %+v requested_by_run_id=%q", runs[0].Metadata, runs[0].RequestedByRunID)
	}
	pending, err := runtime.listPendingLifecyclePacketsDeliveredToRun(ctx, &runs[0])
	if err != nil || len(pending) == 0 {
		t.Fatalf("Texture Super delivered controls=%d err=%v", len(pending), err)
	}
	injected, err := runtime.coagentUpdateTurnInjectorWithInitialPhase(&runs[0], coagentPacketDeliveryThread)(false)
	if err != nil || len(injected) == 0 {
		t.Fatalf("Texture Super inject turns=%d err=%v", len(injected), err)
	}
	runs[0].UpdatedAt = time.Now().UTC()
	if err := productStore.UpdateRun(ctx, runs[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.coagentUpdateTurnInjectorWithInitialPhase(&runs[0], coagentPacketDeliveryThread)(false); err != nil {
		t.Fatalf("Texture Super inject after UpdateRun: %v", err)
	}
	_, _, _, trajectoryID, _, _ := selfDevelopmentTextureJoinIDs("owner", operation.ComputerID, operation.OperationID)
	snapshot, err := productStore.GetLifecycleSnapshot(ctx, "owner", operation.ComputerID, trajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	foundOperationSource := false
	for _, update := range snapshot.Updates {
		if update.Direction == types.LifecyclePacketDirectionControl &&
			selfDevelopmentOperationIDFromPacketSources(update.Packet.Sources) == operation.OperationID {
			foundOperationSource = true
			break
		}
	}
	if !foundOperationSource {
		t.Fatalf("Texture Super control did not cite operation in packet.sources: %+v", snapshot.Updates)
	}
	head := snapshot.HeadRevision
	meta := map[string]any{}
	if len(head.Metadata) > 0 {
		if err := json.Unmarshal(head.Metadata, &meta); err != nil {
			t.Fatal(err)
		}
	}
	if fmt.Sprint(meta["self_development_operation_id"]) != operation.OperationID {
		t.Fatalf("revision metadata missing compact operation join: %#v", meta)
	}
	if _, ok := meta["texture_available_source_entities"]; ok {
		t.Fatalf("source entities leaked into revision metadata: %#v", meta)
	}
	if strings.Contains(head.Content, operation.OperationID) {
		t.Fatalf("operation identity leaked into revision prose: %q", head.Content)
	}
}

func TestSelfDevelopmentTextureJoinRewakesTerminalPersistentSuper(t *testing.T) {
	ctx := context.Background()
	runtime, productStore := testRuntime(t)
	operation := selfdev.Operation{
		OperationID:       "selfdev-rewake",
		ComputerID:        "computer-selfdev-rewake",
		PromptArtifactRef: "artifact:sha256:" + strings.Repeat("b", 64),
	}
	runtime.cfg.ComputerID = operation.ComputerID
	if err := runtime.startSelfDevelopmentPersistentSuper(ctx, operation, "owner", "rewake"); err != nil {
		t.Fatal(err)
	}
	first, err := productStore.ListRunsBySelfDevelopmentOperation(ctx, "owner", operation.OperationID, 2)
	if err != nil || len(first) != 1 {
		t.Fatalf("first Super runs=%d err=%v", len(first), err)
	}
	first[0].State = types.RunFailed
	first[0].Error = "tool loop inject turns after tools: list pending update_coagent turns: record not found"
	first[0].UpdatedAt = time.Now().UTC()
	if err := productStore.UpdateRun(ctx, first[0]); err != nil {
		t.Fatal(err)
	}
	if err := runtime.unbindSelfDevelopmentSuper(ctx, &first[0]); err != nil {
		t.Fatal(err)
	}
	if err := runtime.startSelfDevelopmentPersistentSuper(ctx, operation, "owner", "rewake"); err != nil {
		t.Fatalf("Texture rewake after terminal Super: %v", err)
	}
	second, err := productStore.ListRunsBySelfDevelopmentOperation(ctx, "owner", operation.OperationID, 2)
	if err != nil || len(second) != 1 {
		t.Fatalf("rewake Super runs=%d err=%v", len(second), err)
	}
	if second[0].RunID == first[0].RunID {
		t.Fatalf("Texture rewake reused terminal Super %s", first[0].RunID)
	}
	if metadataStringValue(second[0].Metadata, "assignment_trajectory_id") == "" ||
		metadataStringValue(second[0].Metadata, "request_source") != "lifecycle_texture_control" {
		t.Fatalf("rewake Super lacked Texture control join: %+v", second[0].Metadata)
	}
	if _, err := requirePersistentSuperExecution(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(&second[0]))); err != nil {
		t.Fatalf("rewake Super failed persistent Super gate: %v rec=%+v", err, second[0])
	}
}

func TestSelfDevelopmentExecutingRetryStartsWhenUnbound(t *testing.T) {
	ctx := context.Background()
	runtime, productStore := testRuntime(t)
	computerID := "computer-selfdev-unbound-retry"
	idempotencyKey := "selfdev-unbound-retry"
	prompt := "start unbound executing Super"
	runtime.cfg.ComputerID = computerID
	operations, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	runtime.selfdevOperations = operations
	requestCommitment := computerevent.DigestBytes([]byte(computerID + "\x00" + idempotencyKey + "\x00" + computerevent.DigestBytes([]byte(prompt))))
	identityDigest := computerevent.DigestBytes([]byte(computerID + "\x00" + idempotencyKey))
	now := time.Now().UTC().Truncate(time.Microsecond)
	operationID := "selfdev-" + identityDigest[:32]
	if _, err := productStore.DB().ExecContext(ctx, `INSERT INTO self_development_operations (operation_id,computer_id,idempotency_key,request_commitment,trajectory_id,base_head,prompt_artifact_ref,verifier_refs_json,desired_head,effective_head,state,created_at,updated_at) VALUES (?,?,?,?,?,?,?,'[]',?,?,?, ?,?)`,
		operationID, computerID, idempotencyKey, requestCommitment, "trajectory-"+identityDigest[32:], strings.Repeat("a", 64),
		"artifact:sha256:"+strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("c", 64), selfdev.StateExecuting, now, now); err != nil {
		t.Fatal(err)
	}
	handler := &APIHandler{rt: runtime}
	body, _ := json.Marshal(selfDevelopmentStartRequest{IdempotencyKey: idempotencyKey, Prompt: prompt})
	retry := httptest.NewRequest(http.MethodPost, "/api/computers/"+computerID+"/self-development/operations", strings.NewReader(string(body)))
	retry.Header.Set("X-Authenticated-User", "owner")
	retry.Header.Set("X-Authenticated-Computer", computerID)
	retryResponse := httptest.NewRecorder()
	handler.HandleComputersRouter(retryResponse, retry)
	if retryResponse.Code != http.StatusOK {
		t.Fatalf("unbound executing retry status=%d body=%s", retryResponse.Code, retryResponse.Body.String())
	}
	started, err := productStore.ListRunsBySelfDevelopmentOperation(ctx, "owner", operationID, 2)
	if err != nil || len(started) != 1 {
		t.Fatalf("unbound executing Super runs=%d err=%v", len(started), err)
	}
	if metadataStringValue(started[0].Metadata, "assignment_trajectory_id") == "" ||
		metadataStringValue(started[0].Metadata, "request_source") != "lifecycle_texture_control" {
		t.Fatalf("unbound executing Super lacked Texture control join: %+v", started[0].Metadata)
	}
}

func TestMaterializerBindsBundleToDurableOperationIdentity(t *testing.T) {
	operation := selfdev.Operation{
		ComputerID: "computer-bundle", TrajectoryID: "trajectory-bundle", CapsuleID: "capsule-bundle",
		BaseHead: strings.Repeat("a", 64),
	}
	bundle := transaction.CapsuleEffectBundle{
		ComputerID: operation.ComputerID, TrajectoryRef: operation.TrajectoryID,
		CapsuleIdentity: operation.CapsuleID, BaseEventHead: operation.BaseHead,
	}
	if !selfDevelopmentBundleMatchesOperation(bundle, operation) {
		t.Fatal("exact bundle identity was refused")
	}
	for name, mutate := range map[string]func(*transaction.CapsuleEffectBundle){
		"computer":   func(value *transaction.CapsuleEffectBundle) { value.ComputerID = "other-computer" },
		"trajectory": func(value *transaction.CapsuleEffectBundle) { value.TrajectoryRef = "other-trajectory" },
		"capsule":    func(value *transaction.CapsuleEffectBundle) { value.CapsuleIdentity = "other-capsule" },
		"base":       func(value *transaction.CapsuleEffectBundle) { value.BaseEventHead = strings.Repeat("b", 64) },
	} {
		t.Run(name, func(t *testing.T) {
			changed := bundle
			mutate(&changed)
			if selfDevelopmentBundleMatchesOperation(changed, operation) {
				t.Fatal("cross-operation bundle identity was accepted")
			}
		})
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
		InputArtifactRefs: []string{"artifact:sha256:" + strings.Repeat("d", 64)},
		VerifierRefs:      []string{strings.Repeat("4", 64)}, ReducerVersion: computerevent.ReducerVersionV1,
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

func TestKernelCapabilityUnavailableResponseIsTypedAndNonSecret(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeKernelCapabilityUnavailable(recorder, "probe_unavailable")
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
	var response apiError
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Error != "kernel capability receipt unavailable" || response.Reason != "probe_unavailable" {
		t.Fatalf("response = %#v, want stable typed refusal", response)
	}
}

func TestFinalizedDecisionBindingAcceptsQualifiedConsensusReceipt(t *testing.T) {
	store, input := decisionpolicyValidInput(t)
	receipt, err := decisionpolicy.Reduce(store, input)
	if err != nil {
		t.Fatal(err)
	}
	stored := receipt
	stored.ReceiptDigest = ""
	receiptJSON, err := computerevent.CanonicalJSON(stored)
	if err != nil {
		t.Fatal(err)
	}
	receiptArtifact := computerevent.DigestBytes(receiptJSON)
	if receiptArtifact != receipt.ReceiptDigest {
		t.Fatalf("artifact digest %s != receipt digest %s", receiptArtifact, receipt.ReceiptDigest)
	}
	modeArtifact := strings.Repeat("d", 64)
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: "computer-binding",
		Sequence: 1, PreviousHead: computerevent.ZeroHead, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		EventKind: computerevent.EventEffectAccepted, IdempotencyKey: "decision-binding-consensus", RequestCommitment: strings.Repeat("1", 64),
		TrajectoryID: "trajectory-binding", CapsuleID: "capsule-binding", ParentEventID: "operation-binding",
		ActorProfile: "super", AuthorityRef: decisionpolicy.AuthorityRef(receipt), PrivacyClass: "owner",
		ExpectedDesiredEventHead: strings.Repeat("9", 64), ExpectedEffectiveEventHead: strings.Repeat("a", 64),
		ExpectedDesiredStateCommitment: strings.Repeat("b", 64), ExpectedEffectiveStateCommitment: strings.Repeat("c", 64),
		RequireExpectedHead: true,
		PayloadCommitment:   computerevent.ZeroHead, ProposedEffectRef: strings.Repeat("2", 64), DecisionRef: strings.Repeat("3", 64),
		InputArtifactRefs: []string{"artifact:sha256:" + modeArtifact, "artifact:sha256:" + receiptArtifact},
		VerifierRefs:      []string{strings.Repeat("4", 64)}, ReducerVersion: computerevent.ReducerVersionV1,
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
		State: selfdev.StateAwaitingApproval,
	}
	got, err := verifyFinalizedSelfDevelopmentDecision(operation, transition)
	if err != nil {
		t.Fatalf("qualified consensus decision refused: %v", err)
	}
	if got.AuthorityKind != "qualified-consensus" || got.ConsensusReceiptDigest != receipt.ReceiptDigest || got.Actor != receipt.ReceiptDigest {
		t.Fatalf("verified decision = %+v", got)
	}
	unowned := transition
	unowned.Request.Event.AuthorityRef = "nobody:" + receipt.ReceiptDigest
	if _, err := verifyFinalizedSelfDevelopmentDecision(operation, unowned); err == nil {
		t.Fatal("decision with neither owner nor consensus authority was accepted")
	}
}

func decisionpolicyValidInput(t *testing.T) (*decisionpolicy.Store, decisionpolicy.ConsensusInput) {
	t.Helper()
	store := decisionpolicy.MustReversibleSelfDevV1Store()
	policy, _, err := store.Get(decisionpolicy.PolicyDigestReversibleSelfDevV1)
	if err != nil {
		t.Fatal(err)
	}
	digest := func(seed string) string { return computerevent.DigestBytes([]byte(seed)) }
	subject := decisionpolicy.EffectSubject{
		ComputerID: "computer-test", OperationID: "operation-test", BundleDigest: digest("bundle"),
		DesiredEventHead: digest("desired"), EffectiveEventHead: digest("effective"),
		DesiredStateCommitment: digest("desired-state"), EffectiveStateCommitment: digest("effective-state"),
		EffectClass: decisionpolicy.EffectClassReversible,
	}
	manifest := decisionpolicy.SeatManifest{Seats: []decisionpolicy.Seat{
		{SeatID: "cosuper-author", IndependenceDomain: "authoring", Kind: "agent_profile", EligibilityProof: "assigned-cosuper"},
		{SeatID: "capsule-verifier", IndependenceDomain: "verification", Kind: "independent_verifier", EligibilityProof: "capsule-exec-receipts"},
		{SeatID: "independent-reviewer", IndependenceDomain: "verification", Kind: "agent_profile", EligibilityProof: "not-authoring-cosuper"},
	}}
	subjectDigest, err := subject.Digest()
	if err != nil {
		t.Fatal(err)
	}
	manifestDigest, err := manifest.Digest()
	if err != nil {
		t.Fatal(err)
	}
	selection := decisionpolicy.PolicySelectionReceipt{
		ReceiptKind: decisionpolicy.ReceiptKindPolicySelection, PolicyDigest: decisionpolicy.PolicyDigestReversibleSelfDevV1,
		SeatManifestDigest: manifestDigest, SubjectDigest: subjectDigest,
		SelectedAtHead: digest("head"), SelectedSequence: 4,
	}
	selectionDigest, err := selection.Digest()
	if err != nil {
		t.Fatal(err)
	}
	selection.SelectionDigest = selectionDigest
	sign := func(id, seat, domain, signer string) decisionpolicy.BallotAttestation {
		b := decisionpolicy.BallotAttestation{
			BallotID: id, SeatID: seat, EligibilityProofDigest: digest(seat + "-elig"),
			IndependenceDomain: domain, PolicyDigest: decisionpolicy.PolicyDigestReversibleSelfDevV1,
			SeatManifestDigest: manifestDigest, SubjectDigest: subjectDigest,
			PolicySelectionDigest: selectionDigest, Vote: decisionpolicy.VoteAccept, WindowID: selectionDigest,
			SignerProvenance: signer,
		}
		if err := b.Sign(); err != nil {
			t.Fatal(err)
		}
		return b
	}
	return store, decisionpolicy.ConsensusInput{
		Policy: policy, Manifest: manifest, Subject: subject, Selection: selection,
		Now: time.Date(2026, 8, 16, 0, 50, 0, 0, time.UTC).Format(time.RFC3339Nano),
		Ballots: []decisionpolicy.BallotAttestation{
			sign("b-author", "cosuper-author", "authoring", "signer-author"),
			sign("b-verifier", "capsule-verifier", "verification", "signer-verifier"),
			sign("b-reviewer", "independent-reviewer", "verification", "signer-reviewer"),
		},
	}
}
