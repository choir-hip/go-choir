package vmctl

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/diskinstantiation"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
)

type routeInputFixture struct {
	code    computerversion.CodeClosure
	program computerversion.ArtifactProgram
}

func (f routeInputFixture) PinCode(_ context.Context, closure computerversion.CodeClosure) (computerversion.CodeClosure, error) {
	if err := closure.Verify(); err != nil {
		return computerversion.CodeClosure{}, err
	}
	return closure, nil
}

func (f routeInputFixture) PinArtifactProgram(_ context.Context, program computerversion.ArtifactProgram) (computerversion.ArtifactProgram, error) {
	if err := program.Verify(); err != nil {
		return computerversion.ArtifactProgram{}, err
	}
	return program, nil
}

func (f routeInputFixture) ResolveCode(_ context.Context, ref computerversion.CodeRef) (computerversion.CodeClosure, error) {
	if ref != f.code.Ref {
		return computerversion.CodeClosure{}, computerversion.ErrInputNotFound
	}
	return f.code, nil
}

func (f routeInputFixture) ResolveArtifactProgram(_ context.Context, ref computerversion.ArtifactProgramRef) (computerversion.ArtifactProgram, error) {
	if ref != f.program.Ref {
		return computerversion.ArtifactProgram{}, computerversion.ErrInputNotFound
	}
	return f.program, nil
}

const (
	testApprovalRef    routeledger.ApprovalRef             = "approval:sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	testCertificateRef routeledger.PromotionCertificateRef = "certificate:sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func TestRouteAuthorityPinsInputsBeforeTransition(t *testing.T) {
	authority, version, approvalRef, certificateRef := newRouteAuthorityFixture(t)
	slotID, err := routeledger.RouteSlotID("owner", "primary")
	if err != nil {
		t.Fatal(err)
	}
	resolution, err := authority.transitionAuthorized(context.Background(), routeledger.TransitionCommand{RouteSlotID: slotID, Kind: routeledger.TransitionBootstrap, New: version, ApprovalRef: approvalRef, PromotionCertificateRef: certificateRef, IdempotencyKey: "idempotency:bootstrap"})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if resolution.Slot.Current != version || resolution.CodeClosure.Ref != version.CodeRef || resolution.ArtifactProgram.Ref != version.ArtifactProgramRef {
		t.Fatalf("route/input join mismatch: %+v", resolution)
	}
	resolved, err := authority.Resolve(context.Background(), slotID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.LatestReceipt.ID != resolution.LatestReceipt.ID {
		t.Fatalf("resolve receipt = %q, want %q", resolved.LatestReceipt.ID, resolution.LatestReceipt.ID)
	}
}

func TestRouteAuthorityRefusesForgedResolverOutputBeforeCAS(t *testing.T) {
	authority, version, approvalRef, certificateRef := newRouteAuthorityFixture(t)
	fixture := authority.inputs.(routeInputFixture)
	fixture.code.SourceCommit = "forged-mutable-source"
	authority.inputs = fixture
	slotID, _ := routeledger.RouteSlotID("owner", "primary")
	_, err := authority.transitionAuthorized(context.Background(), routeledger.TransitionCommand{RouteSlotID: slotID, Kind: routeledger.TransitionBootstrap, New: version, ApprovalRef: approvalRef, PromotionCertificateRef: certificateRef, IdempotencyKey: "idempotency:forged"})
	if err == nil {
		t.Fatal("forged resolver output advanced route")
	}
	if _, _, resolveErr := authority.ledger.Resolve(context.Background(), slotID); resolveErr == nil {
		t.Fatal("forged resolver output mutated route ledger")
	}
}

func TestRouteAuthorityRefusesUnpinnedVersion(t *testing.T) {
	authority, _, approvalRef, certificateRef := newRouteAuthorityFixture(t)
	slotID, _ := routeledger.RouteSlotID("owner", "primary")
	_, err := authority.transitionAuthorized(context.Background(), routeledger.TransitionCommand{RouteSlotID: slotID, Kind: routeledger.TransitionBootstrap,
		New: computerversion.ComputerVersion{CodeRef: "code:missing", ArtifactProgramRef: "program:missing"}, ApprovalRef: approvalRef, PromotionCertificateRef: certificateRef, IdempotencyKey: "idempotency:missing"})
	if err == nil {
		t.Fatal("unpinned ComputerVersion transition succeeded")
	}
	if _, _, resolveErr := authority.ledger.Resolve(context.Background(), slotID); resolveErr == nil {
		t.Fatal("unpinned transition mutated route ledger")
	}
}

func TestClientPinsInputsAndTransitionsRoute(t *testing.T) {
	authority, version, _, _ := newRouteAuthorityFixture(t)
	h := NewHandler(NewOwnershipRegistry("http://sandbox"))
	h.SetRouteAuthority(authority)
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/vmctl/computer-version-inputs/pin-code", h.HandlePinComputerVersionCode)
	mux.HandleFunc("/internal/vmctl/computer-version-inputs/pin-artifact-program", h.HandlePinComputerVersionArtifactProgram)
	mux.HandleFunc("/internal/vmctl/computer-version-routes/transition", h.HandleTransitionComputerVersionRoute)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := NewClient(srv.URL)
	fixture := authority.inputs.(routeInputFixture)
	if _, err := client.PinComputerVersionCode(context.Background(), fixture.code); err != nil {
		t.Fatalf("pin code: %v", err)
	}
	if _, err := client.PinComputerVersionArtifactProgram(context.Background(), fixture.program); err != nil {
		t.Fatalf("pin program: %v", err)
	}
	slotID, _ := routeledger.RouteSlotID("owner", "primary")
	if _, err := client.TransitionComputerVersionRoute(context.Background(), routeledger.TransitionCommand{RouteSlotID: slotID, Kind: routeledger.TransitionBootstrap, New: version, ApprovalRef: testApprovalRef, PromotionCertificateRef: testCertificateRef, IdempotencyKey: "idempotency:client-control-bootstrap"}); err == nil {
		t.Fatal("raw HTTP bootstrap bypassed signed frozen candidate")
	}
	if _, _, err := authority.ledger.Resolve(context.Background(), slotID); !errors.Is(err, routeledger.ErrSlotNotFound) {
		t.Fatalf("raw bootstrap mutated route: %v", err)
	}
}

func TestPreparePromotionEndpointFreezesCandidateWithoutRouteCAS(t *testing.T) {
	authority, active, approvalRef, certificateRef := newRouteAuthorityFixture(t)
	slotID, _ := routeledger.RouteSlotID("owner", "primary")
	current, err := authority.transitionAuthorized(t.Context(), routeledger.TransitionCommand{RouteSlotID: slotID, Kind: routeledger.TransitionBootstrap, New: active, ApprovalRef: approvalRef, PromotionCertificateRef: certificateRef, IdempotencyKey: "idempotency:prepare-bootstrap"})
	if err != nil {
		t.Fatal(err)
	}
	candidateVersion := computerversion.ComputerVersion{CodeRef: "code:candidate", ArtifactProgramRef: "artifact:candidate"}
	verification := validVerificationReceipt(t, candidateVersion, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))
	privateKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))
	if err := authority.SetPromotionAuthorityPublicKey(privateKey.Public().(ed25519.PublicKey)); err != nil {
		t.Fatal(err)
	}
	approval := OwnerPromotionApproval{RouteSlotID: slotID, OwnerID: "owner", ComputerVersion: candidateVersion, ConstructionSHA256: verification.ConstructionSHA256, Decision: "approve", KeyID: "owner-test-key", ApprovedAt: verification.VerifiedAt}
	approvalPayload, err := approval.SigningPayload()
	if err != nil {
		t.Fatal(err)
	}
	approval.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, approvalPayload))
	payload, _ := json.Marshal(prepareRoutePromotionRequest{RouteSlotID: slotID, Construction: computerversion.ConstructionResult{Identity: computerversion.ConstructionIdentity{OwnerID: "owner", DesktopID: "primary", CandidateID: "primary"}}, Approval: approval})
	req := httptest.NewRequest(http.MethodPost, PrepareComputerVersionRoutePromotionEndpoint("http://vmctl"), bytes.NewReader(payload))
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	h := NewHandler(NewOwnershipRegistry("http://sandbox"))
	h.SetRouteAuthority(authority)
	h.construction = &constructionService{verifier: fixedRealizationVerifier{receipt: verification}}
	h.HandlePrepareComputerVersionRoutePromotion(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("prepare promotion status=%d body=%s", rec.Code, rec.Body.String())
	}
	var frozen FrozenRoutePromotionCandidate
	if err := json.NewDecoder(rec.Body).Decode(&frozen); err != nil || frozen.Validate() != nil {
		t.Fatalf("invalid frozen response: %+v decode=%v validate=%v", frozen, err, frozen.Validate())
	}
	after, err := authority.Resolve(t.Context(), slotID)
	if err != nil || after.Slot.Generation != current.Slot.Generation || after.Slot.Current != active {
		t.Fatalf("prepare endpoint mutated route: %+v err=%v", after, err)
	}
}

func TestClientResolvesAndVerifiesComputerVersionRoute(t *testing.T) {
	authority, version, approvalRef, certificateRef := newRouteAuthorityFixture(t)
	slotID, _ := routeledger.RouteSlotID("owner", "primary")
	if _, err := authority.transitionAuthorized(context.Background(), routeledger.TransitionCommand{RouteSlotID: slotID, Kind: routeledger.TransitionBootstrap, New: version, ApprovalRef: approvalRef, PromotionCertificateRef: certificateRef, IdempotencyKey: "idempotency:client-bootstrap"}); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	h := NewHandler(NewOwnershipRegistry("http://sandbox"))
	h.SetRouteAuthority(authority)
	srv := httptest.NewServer(http.HandlerFunc(h.HandleResolveComputerVersionRoute))
	defer srv.Close()

	resolution, err := NewClient(srv.URL).ResolveComputerVersionRoute(context.Background(), slotID)
	if err != nil {
		t.Fatalf("client resolve: %v", err)
	}
	if resolution.Slot.Current != version || resolution.LatestReceipt.ID != resolution.Slot.LatestReceiptID {
		t.Fatalf("client route join mismatch: %+v", resolution)
	}
}

func TestResolveRefusesBeforeOwnershipMutationWhenRouteMissing(t *testing.T) {
	authority, version, approvalRef, certificateRef := newRouteAuthorityFixture(t)
	registry := NewOwnershipRegistry("http://sandbox")
	h := NewHandler(registry)
	h.SetRouteAuthority(authority)
	payload := []byte(`{"user_id":"owner","desktop_id":"primary"}`)
	req := httptest.NewRequest(http.MethodPost, "/internal/vmctl/resolve", bytes.NewReader(payload))
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	h.HandleResolve(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("missing route resolve status = %d body=%s", rec.Code, rec.Body.String())
	}
	if own := registry.GetOwnershipForDesktop("owner", "primary"); own != nil {
		t.Fatalf("missing route created ownership: %+v", own)
	}

	slotID, _ := routeledger.RouteSlotID("owner", "primary")
	if _, err := authority.transitionAuthorized(context.Background(), routeledger.TransitionCommand{RouteSlotID: slotID, Kind: routeledger.TransitionBootstrap, New: version, ApprovalRef: approvalRef, PromotionCertificateRef: certificateRef, IdempotencyKey: "idempotency:resolve-bootstrap"}); err != nil {
		t.Fatalf("bootstrap route: %v", err)
	}
	req = httptest.NewRequest(http.MethodPost, "/internal/vmctl/resolve", bytes.NewReader(payload))
	req.Header.Set("X-Internal-Caller", "true")
	rec = httptest.NewRecorder()
	h.HandleResolve(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("joined route resolve status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestLookupAndLifecycleRefuseWhenRequiredRouteAuthorityIsUnavailable(t *testing.T) {
	registry := NewOwnershipRegistry("http://sandbox")
	own, err := registry.ResolveOrAssignDesktop("owner", PrimaryDesktopID)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(registry)
	h.RequireRouteAuthority()

	lookup := httptest.NewRequest(http.MethodGet, "/internal/vmctl/lookup?user_id=owner&desktop_id=primary", nil)
	lookup.Header.Set("X-Internal-Caller", "true")
	lookupRec := httptest.NewRecorder()
	h.HandleLookup(lookupRec, lookup)
	if lookupRec.Code != http.StatusConflict {
		t.Fatalf("lookup without route authority = %d body=%s", lookupRec.Code, lookupRec.Body.String())
	}

	payload := []byte(`{"user_id":"owner","desktop_id":"primary"}`)
	stop := httptest.NewRequest(http.MethodPost, "/internal/vmctl/stop", bytes.NewReader(payload))
	stop.Header.Set("X-Internal-Caller", "true")
	stopRec := httptest.NewRecorder()
	h.HandleStop(stopRec, stop)
	if stopRec.Code != http.StatusConflict {
		t.Fatalf("stop without route authority = %d body=%s", stopRec.Code, stopRec.Body.String())
	}
	current := registry.GetOwnershipForDesktop("owner", PrimaryDesktopID)
	if current == nil || current.VMID != own.VMID || current.State != VMStateActive {
		t.Fatalf("refused lifecycle route mutated ownership: before=%+v after=%+v", own, current)
	}
}

func TestRouteAuthorityPinCodeHandlerVerifiesImmutableInput(t *testing.T) {
	authority, _, _, _ := newRouteAuthorityFixture(t)
	h := NewHandler(NewOwnershipRegistry("http://sandbox"))
	h.SetRouteAuthority(authority)
	fixture := authority.inputs.(routeInputFixture)
	payload, _ := json.Marshal(fixture.code)
	req := httptest.NewRequest(http.MethodPost, "/internal/vmctl/computer-version-inputs/pin-code", bytes.NewReader(payload))
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	h.HandlePinComputerVersionCode(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("pin code status = %d body=%s", rec.Code, rec.Body.String())
	}

	forged := fixture.code
	forged.SourceCommit = "forged"
	payload, _ = json.Marshal(forged)
	req = httptest.NewRequest(http.MethodPost, "/internal/vmctl/computer-version-inputs/pin-code", bytes.NewReader(payload))
	req.Header.Set("X-Internal-Caller", "true")
	rec = httptest.NewRecorder()
	h.HandlePinComputerVersionCode(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("forged pin status = %d, want 400", rec.Code)
	}
}

func TestRouteAuthorityHandlersRequireInternalCaller(t *testing.T) {
	authority, _, _, _ := newRouteAuthorityFixture(t)
	h := NewHandler(NewOwnershipRegistry("http://sandbox"))
	h.SetRouteAuthority(authority)

	req := httptest.NewRequest(http.MethodGet, "/internal/vmctl/computer-version-routes/resolve?route_slot_id=missing", nil)
	rec := httptest.NewRecorder()
	h.HandleResolveComputerVersionRoute(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("resolve without internal caller = %d, want 403", rec.Code)
	}

	payload, _ := json.Marshal(routeledger.TransitionCommand{})
	req = httptest.NewRequest(http.MethodPost, "/internal/vmctl/computer-version-routes/transition", bytes.NewReader(payload))
	rec = httptest.NewRecorder()
	h.HandleTransitionComputerVersionRoute(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("transition without internal caller = %d, want 403", rec.Code)
	}
}

func TestHibernateWorkerResolvesRouteBeforeOwnershipLookup(t *testing.T) {
	authority, _, _, _ := newRouteAuthorityFixture(t)
	registry := NewOwnershipRegistry("http://sandbox")
	h := NewHandler(registry)
	h.SetRouteAuthority(authority)
	req := httptest.NewRequest(http.MethodPost, "/internal/vmctl/hibernate-worker", bytes.NewReader([]byte(`{"user_id":"owner","desktop_id":"primary","worker_id":"missing-worker"}`)))
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	h.HandleHibernateWorker(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("hibernate without D-ROUTE status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "worker ownership not found") {
		t.Fatalf("hibernate inspected ownership before route refusal: %s", rec.Body.String())
	}
}

type fixedRealizationVerifier struct {
	receipt computerversion.RealizationVerificationReceipt
	err     error
}

func (f fixedRealizationVerifier) Verify(context.Context, diskinstantiation.Plan, computerversion.ConstructionResult) (computerversion.RealizationVerificationReceipt, error) {
	return f.receipt, f.err
}

func newRouteAuthorityFixture(t *testing.T) (*RouteAuthority, computerversion.ComputerVersion, routeledger.ApprovalRef, routeledger.PromotionCertificateRef) {
	t.Helper()
	now := time.Date(2026, 7, 16, 4, 0, 0, 0, time.UTC)
	closure, err := computerversion.NewCodeClosure(strings.Repeat("1", 40), []computerversion.CodeArtifact{{
		Name: "sandbox", SHA256: repeatedHex('a'), URI: "nix-store+sha256://" + repeatedHex('a') + "/nix/store/sandbox",
	}}, now)
	if err != nil {
		t.Fatal(err)
	}
	program, err := computerversion.NewArtifactProgram([]computerversion.ArtifactProgramEntry{{
		Kind: "embedded_dolt_export", ContentSHA256: repeatedHex('b'), ArtifactURI: "artifact+sha256://" + repeatedHex('b') + "/owner/state",
	}}, now)
	if err != nil {
		t.Fatal(err)
	}
	ledger := routeledger.NewMemoryLedger()
	version := computerversion.ComputerVersion{CodeRef: closure.Ref, ArtifactProgramRef: program.Ref}
	approval, err := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidenceApproval, "computer:owner:primary", version, json.RawMessage(`{"fixture":"approval"}`), now)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidencePromotionCertificate, "computer:owner:primary", version, json.RawMessage(`{"fixture":"certificate"}`), now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.PinAuthorizationEvidence(t.Context(), approval); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.PinAuthorizationEvidence(t.Context(), certificate); err != nil {
		t.Fatal(err)
	}
	authority, err := newMemoryRouteAuthority(ledger, routeInputFixture{code: closure, program: program})
	if err != nil {
		t.Fatal(err)
	}
	return authority, version, routeledger.ApprovalRef(approval.Ref), routeledger.PromotionCertificateRef(certificate.Ref)
}

func repeatedHex(ch byte) string {
	value := make([]byte, 64)
	for i := range value {
		value[i] = ch
	}
	return string(value)
}
