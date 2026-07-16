package vmctl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
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
	authority, version := newRouteAuthorityFixture(t)
	slotID, err := routeledger.RouteSlotID("owner", "primary")
	if err != nil {
		t.Fatal(err)
	}
	resolution, err := authority.Transition(context.Background(), routeledger.TransitionCommand{RouteSlotID: slotID, Kind: routeledger.TransitionBootstrap, New: version, ApprovalRef: testApprovalRef, PromotionCertificateRef: testCertificateRef, IdempotencyKey: "idempotency:bootstrap"})
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
	authority, version := newRouteAuthorityFixture(t)
	fixture := authority.inputs.(routeInputFixture)
	fixture.code.SourceCommit = "forged-mutable-source"
	authority.inputs = fixture
	slotID, _ := routeledger.RouteSlotID("owner", "primary")
	_, err := authority.Transition(context.Background(), routeledger.TransitionCommand{RouteSlotID: slotID, Kind: routeledger.TransitionBootstrap, New: version, ApprovalRef: testApprovalRef, PromotionCertificateRef: testCertificateRef, IdempotencyKey: "idempotency:forged"})
	if err == nil {
		t.Fatal("forged resolver output advanced route")
	}
	if _, _, resolveErr := authority.ledger.Resolve(context.Background(), slotID); resolveErr == nil {
		t.Fatal("forged resolver output mutated route ledger")
	}
}

func TestRouteAuthorityRefusesUnpinnedVersion(t *testing.T) {
	authority, _ := newRouteAuthorityFixture(t)
	slotID, _ := routeledger.RouteSlotID("owner", "primary")
	_, err := authority.Transition(context.Background(), routeledger.TransitionCommand{RouteSlotID: slotID, Kind: routeledger.TransitionBootstrap,
		New: computerversion.ComputerVersion{CodeRef: "code:missing", ArtifactProgramRef: "program:missing"}, ApprovalRef: testApprovalRef, PromotionCertificateRef: testCertificateRef, IdempotencyKey: "idempotency:missing"})
	if err == nil {
		t.Fatal("unpinned ComputerVersion transition succeeded")
	}
	if _, _, resolveErr := authority.ledger.Resolve(context.Background(), slotID); resolveErr == nil {
		t.Fatal("unpinned transition mutated route ledger")
	}
}

func TestClientPinsInputsAndTransitionsRoute(t *testing.T) {
	authority, version := newRouteAuthorityFixture(t)
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
	resolution, err := client.TransitionComputerVersionRoute(context.Background(), routeledger.TransitionCommand{RouteSlotID: slotID, Kind: routeledger.TransitionBootstrap, New: version, ApprovalRef: testApprovalRef, PromotionCertificateRef: testCertificateRef, IdempotencyKey: "idempotency:client-control-bootstrap"})
	if err != nil {
		t.Fatalf("transition route: %v", err)
	}
	if resolution.Slot.Current != version || resolution.TransitionReceipt == nil {
		t.Fatalf("transition response join mismatch: %+v", resolution)
	}
}

func TestClientResolvesAndVerifiesComputerVersionRoute(t *testing.T) {
	authority, version := newRouteAuthorityFixture(t)
	slotID, _ := routeledger.RouteSlotID("owner", "primary")
	if _, err := authority.Transition(context.Background(), routeledger.TransitionCommand{RouteSlotID: slotID, Kind: routeledger.TransitionBootstrap, New: version, ApprovalRef: testApprovalRef, PromotionCertificateRef: testCertificateRef, IdempotencyKey: "idempotency:client-bootstrap"}); err != nil {
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
	authority, version := newRouteAuthorityFixture(t)
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
	if _, err := authority.Transition(context.Background(), routeledger.TransitionCommand{RouteSlotID: slotID, Kind: routeledger.TransitionBootstrap, New: version, ApprovalRef: testApprovalRef, PromotionCertificateRef: testCertificateRef, IdempotencyKey: "idempotency:resolve-bootstrap"}); err != nil {
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
	authority, _ := newRouteAuthorityFixture(t)
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
	authority, _ := newRouteAuthorityFixture(t)
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
	authority, _ := newRouteAuthorityFixture(t)
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

func newRouteAuthorityFixture(t *testing.T) (*RouteAuthority, computerversion.ComputerVersion) {
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
	authority, err := NewRouteAuthority(routeledger.NewMemoryLedger(), routeInputFixture{code: closure, program: program}, routeEvidenceFixture{})
	if err != nil {
		t.Fatal(err)
	}
	return authority, computerversion.ComputerVersion{CodeRef: closure.Ref, ArtifactProgramRef: program.Ref}
}

type routeEvidenceFixture struct{}

func (routeEvidenceFixture) VerifyTransitionEvidence(_ context.Context, command routeledger.TransitionCommand) error {
	if command.ApprovalRef != testApprovalRef || command.PromotionCertificateRef != testCertificateRef {
		return errors.New("transition evidence not pinned")
	}
	return nil
}

func repeatedHex(ch byte) string {
	value := make([]byte, 64)
	for i := range value {
		value[i] = ch
	}
	return string(value)
}
