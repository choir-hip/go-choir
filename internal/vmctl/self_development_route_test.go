package vmctl

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

type projectionInputCatalog struct {
	codes    map[computerversion.CodeRef]computerversion.CodeClosure
	programs map[computerversion.ArtifactProgramRef]computerversion.ArtifactProgram
}

func (c *projectionInputCatalog) ResolveCode(_ context.Context, ref computerversion.CodeRef) (computerversion.CodeClosure, error) {
	value, ok := c.codes[ref]
	if !ok {
		return computerversion.CodeClosure{}, computerversion.ErrInputNotFound
	}
	return value, nil
}

func (c *projectionInputCatalog) ResolveArtifactProgram(_ context.Context, ref computerversion.ArtifactProgramRef) (computerversion.ArtifactProgram, error) {
	value, ok := c.programs[ref]
	if !ok {
		return computerversion.ArtifactProgram{}, computerversion.ErrInputNotFound
	}
	return value, nil
}

func (c *projectionInputCatalog) PinCode(_ context.Context, closure computerversion.CodeClosure) (computerversion.CodeClosure, error) {
	if err := closure.Verify(); err != nil {
		return computerversion.CodeClosure{}, err
	}
	c.codes[closure.Ref] = closure
	return closure, nil
}

func (c *projectionInputCatalog) PinArtifactProgram(_ context.Context, program computerversion.ArtifactProgram) (computerversion.ArtifactProgram, error) {
	if err := program.Verify(); err != nil {
		return computerversion.ArtifactProgram{}, err
	}
	c.programs[program.Ref] = program
	return program, nil
}

func TestSelfDevelopmentRouteProjectionRequiresExactPlatformCertificate(t *testing.T) {
	now := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	makeInputs := func(source, artifact string) (computerversion.CodeClosure, computerversion.ArtifactProgram) {
		closure, err := computerversion.NewCodeClosure(strings.Repeat(source, 40), []computerversion.CodeArtifact{{Name: "bundle", SHA256: strings.Repeat(artifact, 64), URI: "artifact+sha256://" + strings.Repeat(artifact, 64) + "/bundle"}}, now)
		if err != nil {
			t.Fatal(err)
		}
		program, err := computerversion.NewArtifactProgram([]computerversion.ArtifactProgramEntry{{Kind: "capsule_effect_bundle", ContentSHA256: strings.Repeat(artifact, 64), ArtifactURI: "artifact+sha256://" + strings.Repeat(artifact, 64) + "/bundle"}}, now)
		if err != nil {
			t.Fatal(err)
		}
		return closure, program
	}
	oldCode, oldProgram := makeInputs("1", "a")
	newCode, newProgram := makeInputs("2", "b")
	oldVersion := computerversion.ComputerVersion{CodeRef: oldCode.Ref, ArtifactProgramRef: oldProgram.Ref}
	newVersion := computerversion.ComputerVersion{CodeRef: newCode.Ref, ArtifactProgramRef: newProgram.Ref}
	catalog := &projectionInputCatalog{codes: map[computerversion.CodeRef]computerversion.CodeClosure{oldCode.Ref: oldCode}, programs: map[computerversion.ArtifactProgramRef]computerversion.ArtifactProgram{oldProgram.Ref: oldProgram}}
	ledger := routeledger.NewMemoryLedger()
	authority, err := newMemoryRouteAuthority(ledger, catalog)
	if err != nil {
		t.Fatal(err)
	}
	slotID, _ := routeledger.RouteSlotID("owner", "primary")
	bootstrapApproval, _ := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidenceApproval, slotID, oldVersion, json.RawMessage(`{"bootstrap":"approval"}`), now)
	bootstrapCertificate, _ := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidencePromotionCertificate, slotID, oldVersion, json.RawMessage(`{"bootstrap":"certificate"}`), now)
	slot, _, err := ledger.TransitionWithEvidence(t.Context(), routeledger.TransitionCommand{RouteSlotID: slotID, Kind: routeledger.TransitionBootstrap, New: oldVersion, ApprovalRef: routeledger.ApprovalRef(bootstrapApproval.Ref), PromotionCertificateRef: routeledger.PromotionCertificateRef(bootstrapCertificate.Ref), IdempotencyKey: "idempotency:bootstrap"}, []routeledger.AuthorizationEvidence{bootstrapApproval, bootstrapCertificate})
	if err != nil {
		t.Fatal(err)
	}
	privateKey := ed25519.NewKeyFromSeed([]byte(strings.Repeat("p", ed25519.SeedSize)))
	signingKey := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "platform-test"}, PrivateKey: privateKey}
	verifierKey := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "verifier-control", KeyID: "verifier-test"}, PrivateKey: privateKey}
	verifierRequest := selfdevprotocol.VerifierCertificateRequest{
		Version: 1, ComputerID: "computer-active", OperationID: "operation-test",
		BundleDigest: repeatedHex('b'), VerificationEventDigest: repeatedHex('c'),
		VerifierEvidenceRefs: []string{repeatedHex('c')}, DecisionEventHead: repeatedHex('c'),
		CodeRef: string(newVersion.CodeRef), ArtifactProgramRef: string(newVersion.ArtifactProgramRef),
		ReleaseDigest: repeatedHex('e'), Decision: "pass",
	}
	verifierCertificate, err := selfdevprotocol.NewVerifierCertificate(verifierRequest, verifierKey, now)
	if err != nil {
		t.Fatal(err)
	}
	verifierResponse := selfdevprotocol.VerifierCertificateResponse{
		Request: verifierRequest, Certificate: verifierCertificate,
		PublicKey: base64.RawStdEncoding.EncodeToString(privateKey.Public().(ed25519.PublicKey)),
	}
	verifierJSON, _ := computerevent.CanonicalJSON(verifierCertificate)
	checkpointRequest := selfdevprotocol.CheckpointRequest{ComputerID: "computer-active", IdempotencyKey: "checkpoint", ComputerVersion: newVersion, AcceptedEventHead: repeatedHex('c'), EffectiveEventHead: repeatedHex('c'), EffectiveStateCommitment: repeatedHex('d'), EventHeadReceiptID: "receipt-event", ReleaseDigest: repeatedHex('e'), ReconstructionDigest: repeatedHex('f'), MaterializationReceiptDigest: repeatedHex('1'), VerifierCertificateDigest: computerevent.DigestBytes(verifierJSON), VerifierCertificate: verifierResponse, ReducerVersion: 1}
	checkpoint, _, err := selfdevprotocol.CheckpointFromRequest(checkpointRequest)
	if err != nil {
		t.Fatal(err)
	}
	checkpointCommitment, _ := selfdevprotocol.Digest(checkpointRequest)
	checkpointReceipt, _ := selfdevprotocol.NewAuthorityReceipt(selfdevprotocol.ReceiptKindCheckpoint, checkpointRequest.ComputerID, checkpointCommitment, checkpoint.Digest, "corpusd", signingKey, now)
	checkpointResponse := selfdevprotocol.CheckpointResponse{Checkpoint: checkpoint, Receipt: checkpointReceipt}
	checkpointReceiptDigest, _ := selfdevprotocol.Digest(checkpointReceipt)
	accepted := selfdevprotocol.AcceptedEventAuthorizationEvidence{Version: 1, ComputerID: checkpointRequest.ComputerID, AcceptedOrRollbackEventDigest: checkpointRequest.AcceptedEventHead, EventHeadReceiptID: checkpointRequest.EventHeadReceiptID, EffectiveEventHead: checkpointRequest.EffectiveEventHead, OldComputerVersion: oldVersion, NewComputerVersion: newVersion, DecisionActor: "owner", DecisionScope: "computer:self_development:approve"}
	acceptedJSON, _ := computerevent.CanonicalJSON(accepted)
	approval, _ := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidenceApproval, slotID, newVersion, acceptedJSON, now)
	join := selfdevprotocol.PromotionJoinEvidence{Version: 1, ComputerID: checkpointRequest.ComputerID, EventHeadReceiptID: checkpointRequest.EventHeadReceiptID, CheckpointReceiptDigest: checkpointReceiptDigest, MaterializationReceiptDigest: checkpointRequest.MaterializationReceiptDigest, VerifierCertificateDigest: checkpointRequest.VerifierCertificateDigest, OldComputerVersion: oldVersion, NewComputerVersion: newVersion}
	joinJSON, _ := computerevent.CanonicalJSON(join)
	promotion, _ := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidencePromotionCertificate, slotID, newVersion, joinJSON, now)
	command := routeledger.TransitionCommand{RouteSlotID: slotID, Kind: routeledger.TransitionPromote, Old: oldVersion, New: newVersion, ExpectedGeneration: slot.Generation, ApprovalRef: routeledger.ApprovalRef(approval.Ref), PromotionCertificateRef: routeledger.PromotionCertificateRef(promotion.Ref), IdempotencyKey: "idempotency:selfdev-route"}
	projection := selfdevprotocol.RouteProjectionRequest{ComputerID: checkpointRequest.ComputerID, IdempotencyKey: "route-certificate", Checkpoint: checkpointResponse, CanonicalEventHead: checkpointRequest.AcceptedEventHead, EventHeadReceiptID: checkpointRequest.EventHeadReceiptID, CodeClosure: newCode, ArtifactProgram: newProgram, ApprovalEvidence: approval, PromotionEvidence: promotion, Command: command, DecisionActor: "owner", DecisionScope: "computer:self_development:approve", ExpiresAt: now.Add(2 * time.Minute).Format(time.RFC3339Nano)}
	certificate, artifact, err := selfdevprotocol.RouteProjectionFromRequest(projection, now)
	if err != nil {
		t.Fatal(err)
	}
	projectionCommitment, _ := selfdevprotocol.Digest(projection)
	certificateReceipt, _ := selfdevprotocol.NewAuthorityReceipt(selfdevprotocol.ReceiptKindRouteProjection, projection.ComputerID, projectionCommitment, computerevent.DigestBytes(artifact), "corpusd", signingKey, now)
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Caller") != "true" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"signer_domain": "platform-control", "key_id": "platform-test", "public_key": base64.RawStdEncoding.EncodeToString(privateKey.Public().(ed25519.PublicKey))})
	}))
	defer corpusd.Close()
	registry := NewOwnershipRegistry("http://sandbox")
	registry.SetCorpusdURL(corpusd.URL)
	ownership := &VMOwnership{VMID: "vm-realization", ComputerID: checkpointRequest.ComputerID, UserID: "owner", DesktopID: "primary", State: VMStateActive}
	registry.ownerships[ownershipKey("owner", "primary")] = ownership
	registry.vmByID[ownership.VMID] = ownership
	if got := stableComputerID(ownership.UserID, ownership.DesktopID, ownership.ComputerID); got != projection.ComputerID {
		t.Fatalf("stable route computer identity = %q, want %q", got, projection.ComputerID)
	}
	request := selfdevprotocol.ApplyRouteProjectionRequest{Projection: projection, Authorization: selfdevprotocol.RouteProjectionResponse{Certificate: certificate, Receipt: certificateReceipt}}
	resolution, err := authority.ApplySelfDevelopmentProjection(t.Context(), registry, request, now)
	if err != nil || resolution.Slot.Current != newVersion || resolution.Slot.Generation != slot.Generation+1 {
		t.Fatalf("self-development projection result=%+v err=%v", resolution, err)
	}
	replayed, err := authority.ApplySelfDevelopmentProjection(t.Context(), registry, request, now.Add(10*time.Minute))
	if err != nil || replayed.TransitionReceipt == nil || replayed.TransitionReceipt.ID != resolution.TransitionReceipt.ID {
		t.Fatalf("expired exact replay result=%+v err=%v", replayed, err)
	}
	tampered := request
	tampered.Authorization.Certificate.EffectiveEventHead = repeatedHex('9')
	if _, err := authority.ApplySelfDevelopmentProjection(t.Context(), registry, tampered, now); err == nil {
		t.Fatal("tampered route certificate was accepted")
	}
}

func TestHandleListClassifiesConstructedOwnershipFromRouteEvidence(t *testing.T) {
	now := time.Date(2026, 7, 23, 20, 0, 0, 0, time.UTC)
	code, err := computerversion.NewCodeClosure(strings.Repeat("1", 40), []computerversion.CodeArtifact{{
		Name: "bundle", SHA256: repeatedHex('a'), URI: "artifact+sha256://" + repeatedHex('a') + "/bundle",
	}}, now)
	if err != nil {
		t.Fatal(err)
	}
	program, err := computerversion.NewArtifactProgram([]computerversion.ArtifactProgramEntry{{
		Kind: "capsule_effect_bundle", ContentSHA256: repeatedHex('a'), ArtifactURI: "artifact+sha256://" + repeatedHex('a') + "/bundle",
	}}, now)
	if err != nil {
		t.Fatal(err)
	}
	version := computerversion.ComputerVersion{CodeRef: code.Ref, ArtifactProgramRef: program.Ref}
	catalog := &projectionInputCatalog{
		codes:    map[computerversion.CodeRef]computerversion.CodeClosure{code.Ref: code},
		programs: map[computerversion.ArtifactProgramRef]computerversion.ArtifactProgram{program.Ref: program},
	}
	ledger := routeledger.NewMemoryLedger()
	authority, err := newMemoryRouteAuthority(ledger, catalog)
	if err != nil {
		t.Fatal(err)
	}
	registry := NewOwnershipRegistry("http://127.0.0.1:8085")
	ownership, err := registry.ResolveOrAssignDesktop("owner", PrimaryDesktopID)
	if err != nil {
		t.Fatal(err)
	}
	slotID, _ := routeledger.RouteSlotID(ownership.UserID, ownership.DesktopID)
	ownerApproval, _ := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidenceApproval, slotID, version, json.RawMessage(`{"decision":"approved","signature":"owner"}`), now)
	verification := map[string]any{
		"verification_receipt_id": "verification:sha256:" + repeatedHex('e'),
		"verifier":                "independent-production-realization-verifier",
		"computer_version":        version,
		"disk_receipt_id":         "disk-instantiation:sha256:" + repeatedHex('d'),
		"vm_id":                   ownership.VMID,
	}
	certificatePayload, _ := json.Marshal(map[string]any{
		"kind":          "verified_route_bootstrap",
		"route_slot_id": slotID,
		"verification":  verification,
		"approval_ref":  ownerApproval.Ref,
	})
	certificate, _ := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidencePromotionCertificate, slotID, version, certificatePayload, now)
	gatePayload, _ := json.Marshal(map[string]any{
		"candidate_id":       "route-bootstrap:sha256:" + repeatedHex('c'),
		"owner_approval_ref": ownerApproval.Ref,
		"verification":       verification,
	})
	gate, _ := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidenceApproval, slotID, version, gatePayload, now)
	if _, _, err := ledger.TransitionWithEvidence(t.Context(), routeledger.TransitionCommand{
		RouteSlotID: slotID, Kind: routeledger.TransitionBootstrap, New: version,
		ApprovalRef: routeledger.ApprovalRef(gate.Ref), PromotionCertificateRef: routeledger.PromotionCertificateRef(certificate.Ref),
		IdempotencyKey: "idempotency:constructed-list",
	}, []routeledger.AuthorizationEvidence{gate, ownerApproval, certificate}); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(registry)
	handler.SetRouteAuthority(authority)
	if err := handler.AuthorizeComputerVersionRoute(t.Context(), ownership.UserID, ownership.DesktopID); err != nil {
		t.Fatalf("joined constructed ownership was not authorized: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/internal/vmctl/list", nil)
	request.Header.Set("X-Internal-Caller", "true")
	response := httptest.NewRecorder()
	handler.HandleList(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", response.Code, response.Body.String())
	}
	var result struct {
		Ownerships []ownershipResponse `json:"ownerships"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Ownerships) != 1 {
		t.Fatalf("ownerships=%d, want 1", len(result.Ownerships))
	}
	got := result.Ownerships[0]
	if got.ComputerID != ownership.ComputerID || got.SnapshotKind != "constructed-computer-version" ||
		got.ConstructionVersion == nil || *got.ConstructionVersion != version ||
		got.ConstructionDiskReceiptID != "disk-instantiation:sha256:"+repeatedHex('d') {
		t.Fatalf("constructed ownership response=%+v", got)
	}
	request = httptest.NewRequest(http.MethodPost, "/internal/vmctl/resolve", strings.NewReader(`{"user_id":"owner","desktop_id":"primary"}`))
	request.Header.Set("X-Internal-Caller", "true")
	response = httptest.NewRecorder()
	handler.HandleResolve(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("joined constructed resolve status=%d body=%s", response.Code, response.Body.String())
	}
	unknown, err := registry.ResolveOrAssignDesktop("ordinary-owner", PrimaryDesktopID)
	if err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodGet, "/internal/vmctl/list", nil)
	request.Header.Set("X-Internal-Caller", "true")
	response = httptest.NewRecorder()
	handler.HandleList(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("mixed no-route list status=%d body=%s", response.Code, response.Body.String())
	}
	result.Ownerships = nil
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	var ordinary ownershipResponse
	for _, listed := range result.Ownerships {
		if listed.VMID == unknown.VMID {
			ordinary = listed
			break
		}
	}
	if ordinary.VMID != unknown.VMID || ordinary.SnapshotKind != "" || ordinary.ConstructionVersion != nil || ordinary.ConstructionDiskReceiptID != "" {
		t.Fatalf("no-route mutable ownership response=%+v", ordinary)
	}
	unknownSlotID, _ := routeledger.RouteSlotID(unknown.UserID, unknown.DesktopID)
	unknownApproval, _ := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidenceApproval, unknownSlotID, version, json.RawMessage(`{"bootstrap":"approval"}`), now)
	unknownCertificate, _ := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidencePromotionCertificate, unknownSlotID, version, json.RawMessage(`{"bootstrap":"certificate"}`), now)
	if _, _, err := ledger.TransitionWithEvidence(t.Context(), routeledger.TransitionCommand{
		RouteSlotID: unknownSlotID, Kind: routeledger.TransitionBootstrap, New: version,
		ApprovalRef: routeledger.ApprovalRef(unknownApproval.Ref), PromotionCertificateRef: routeledger.PromotionCertificateRef(unknownCertificate.Ref),
		IdempotencyKey: "idempotency:unknown-list",
	}, []routeledger.AuthorizationEvidence{unknownApproval, unknownCertificate}); err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodGet, "/internal/vmctl/list", nil)
	request.Header.Set("X-Internal-Caller", "true")
	response = httptest.NewRecorder()
	handler.HandleList(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("unknown evidence list status=%d body=%s, want fail-closed 503", response.Code, response.Body.String())
	}
}

func TestHandleListFailsClosedWithoutRequiredRouteAuthority(t *testing.T) {
	registry := NewOwnershipRegistry("http://127.0.0.1:8085")
	if _, err := registry.ResolveOrAssignDesktop("owner", PrimaryDesktopID); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(registry)
	handler.RequireRouteAuthority()
	request := httptest.NewRequest(http.MethodGet, "/internal/vmctl/list", nil)
	request.Header.Set("X-Internal-Caller", "true")
	response := httptest.NewRecorder()
	handler.HandleList(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("list status=%d body=%s, want fail-closed 503", response.Code, response.Body.String())
	}
}

func TestResolveComputerVersionRouteReturnsTypedCanonicalAbsence(t *testing.T) {
	ledger := routeledger.NewMemoryLedger()
	catalog := &projectionInputCatalog{
		codes:    map[computerversion.CodeRef]computerversion.CodeClosure{},
		programs: map[computerversion.ArtifactProgramRef]computerversion.ArtifactProgram{},
	}
	authority, err := newMemoryRouteAuthority(ledger, catalog)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(NewOwnershipRegistry("http://127.0.0.1:8085"))
	handler.SetRouteAuthority(authority)
	server := httptest.NewServer(http.HandlerFunc(handler.HandleResolveComputerVersionRoute))
	defer server.Close()

	slotID, err := routeledger.RouteSlotID("ordinary-owner", PrimaryDesktopID)
	if err != nil {
		t.Fatal(err)
	}
	resolution, err := NewClient(server.URL).ResolveComputerVersionRouteOrAbsent(t.Context(), slotID)
	if err != nil {
		t.Fatal(err)
	}
	if !resolution.RouteAbsent {
		t.Fatalf("route resolution=%+v, want canonical absence", resolution)
	}
	if _, err := NewClient(server.URL).ResolveComputerVersionRoute(t.Context(), slotID); err == nil {
		t.Fatal("strict immutable-route client accepted canonical absence")
	}
	tampered := resolution
	tampered.Slot.ID = slotID
	if err := validateRouteResolution(slotID, tampered); err == nil {
		t.Fatal("route-absent response carrying route authority was accepted")
	}
	if err := handler.AuthorizeComputerVersionRoute(t.Context(), "ordinary-owner", PrimaryDesktopID); err != nil {
		t.Fatalf("ordinary route absence was refused: %v", err)
	}
	if err := handler.AuthorizeComputerVersionRoute(t.Context(), UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID); err == nil {
		t.Fatal("shared route guard accepted platform route absence")
	}
	platformRequest := httptest.NewRequest(http.MethodPost, "/internal/vmctl/resolve", strings.NewReader(
		`{"user_id":"`+UniversalWirePlatformOwnerID+`","desktop_id":"`+UniversalWirePlatformDesktopID+`"}`,
	))
	platformRequest.Header.Set("X-Internal-Caller", "true")
	platformResponse := httptest.NewRecorder()
	handler.HandleResolve(platformResponse, platformRequest)
	if platformResponse.Code != http.StatusConflict {
		t.Fatalf("route-absent platform resolve status=%d body=%s, want 409", platformResponse.Code, platformResponse.Body.String())
	}
	if got := handler.registry.GetOwnershipForDesktop(UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID); got != nil {
		t.Fatalf("route-absent platform resolve allocated %+v", got)
	}
	malformed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"route_absent":true} {}`))
	}))
	defer malformed.Close()
	if _, err := NewClient(malformed.URL).ResolveComputerVersionRouteOrAbsent(t.Context(), slotID); err == nil {
		t.Fatal("route client accepted trailing JSON content")
	}
}

func TestHandleResolveRefusesRoutedComputerWithoutRealizedOwnership(t *testing.T) {
	now := time.Date(2026, 7, 23, 20, 0, 0, 0, time.UTC)
	code, err := computerversion.NewCodeClosure(strings.Repeat("a", 40), []computerversion.CodeArtifact{{
		Name: "bundle", SHA256: repeatedHex('b'), URI: "artifact+sha256://" + repeatedHex('b') + "/bundle",
	}}, now)
	if err != nil {
		t.Fatal(err)
	}
	program, err := computerversion.NewArtifactProgram([]computerversion.ArtifactProgramEntry{{
		Kind: "capsule_effect_bundle", ContentSHA256: repeatedHex('b'), ArtifactURI: "artifact+sha256://" + repeatedHex('b') + "/bundle",
	}}, now)
	if err != nil {
		t.Fatal(err)
	}
	version := computerversion.ComputerVersion{CodeRef: code.Ref, ArtifactProgramRef: program.Ref}
	catalog := &projectionInputCatalog{
		codes:    map[computerversion.CodeRef]computerversion.CodeClosure{code.Ref: code},
		programs: map[computerversion.ArtifactProgramRef]computerversion.ArtifactProgram{program.Ref: program},
	}
	ledger := routeledger.NewMemoryLedger()
	authority, err := newMemoryRouteAuthority(ledger, catalog)
	if err != nil {
		t.Fatal(err)
	}
	slotID, err := routeledger.RouteSlotID("routed-owner", PrimaryDesktopID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ledger.Transition(t.Context(), routeledger.TransitionCommand{
		RouteSlotID: slotID, Kind: routeledger.TransitionBootstrap, New: version,
		ApprovalRef:             routeledger.ApprovalRef("approval:sha256:" + repeatedHex('c')),
		PromotionCertificateRef: routeledger.PromotionCertificateRef("certificate:sha256:" + repeatedHex('d')),
		IdempotencyKey:          "idempotency:routed-without-ownership",
	}); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(NewOwnershipRegistry("http://127.0.0.1:8085"))
	handler.SetRouteAuthority(authority)
	request := httptest.NewRequest(http.MethodPost, "/internal/vmctl/resolve", strings.NewReader(`{"user_id":"routed-owner","desktop_id":"primary"}`))
	request.Header.Set("X-Internal-Caller", "true")
	response := httptest.NewRecorder()
	handler.HandleResolve(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("routed missing-ownership resolve status=%d body=%s, want 409", response.Code, response.Body.String())
	}
	if got := handler.registry.GetOwnershipForDesktop("routed-owner", PrimaryDesktopID); got != nil {
		t.Fatalf("routed missing-ownership resolve allocated %+v", got)
	}
	if err := handler.AuthorizeComputerVersionRoute(t.Context(), "routed-owner", PrimaryDesktopID); err == nil {
		t.Fatal("shared route guard accepted routed computer without realized ownership")
	}
	mismatched, err := handler.registry.ResolveOrAssignDesktop("routed-owner", PrimaryDesktopID)
	if err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodPost, "/internal/vmctl/resolve", strings.NewReader(`{"user_id":"routed-owner","desktop_id":"primary"}`))
	request.Header.Set("X-Internal-Caller", "true")
	response = httptest.NewRecorder()
	handler.HandleResolve(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("nonjoining routed ownership status=%d body=%s, want 409", response.Code, response.Body.String())
	}
	if err := handler.AuthorizeComputerVersionRoute(t.Context(), "routed-owner", PrimaryDesktopID); err == nil {
		t.Fatal("shared route guard accepted nonjoining routed ownership")
	}
	if got := handler.registry.GetOwnershipForDesktop("routed-owner", PrimaryDesktopID); got == nil || got.VMID != mismatched.VMID {
		t.Fatalf("nonjoining routed ownership mutated: got=%+v want vm_id=%s", got, mismatched.VMID)
	}
}

func repeatedHex(value byte) string {
	return strings.Repeat(string(value), 64)
}

func TestPostComputerVersionControlRejectsUnknownAndTrailingContent(t *testing.T) {
	for name, body := range map[string]string{
		"unknown":  `{"route_absent":true,"unexpected":true}`,
		"trailing": `{"route_absent":true} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			var resolution RouteResolution
			if err := NewClient(server.URL).postComputerVersionControl(
				t.Context(), server.URL, map[string]string{"request": "test"}, &resolution,
			); err == nil {
				t.Fatalf("accepted malformed control response %q", body)
			}
		})
	}
}
