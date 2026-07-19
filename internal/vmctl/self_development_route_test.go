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

func repeatedHex(value byte) string {
	return strings.Repeat(string(value), 64)
}
