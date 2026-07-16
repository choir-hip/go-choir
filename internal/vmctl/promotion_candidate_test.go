package vmctl

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/diskinstantiation"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/store"
)

type acceptingPromotionArtifactVerifier struct{}

func (acceptingPromotionArtifactVerifier) VerifyArtifact(context.Context, string, string) error {
	return nil
}

func TestSQLRouteCASRequiresPinnedSignedExecutionEnvelope(t *testing.T) {
	now := time.Date(2026, 7, 16, 14, 0, 0, 0, time.UTC)
	productStore, err := store.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = productStore.Close() }()
	catalog := computerversion.NewSQLInputCatalog(productStore.DB(), acceptingPromotionArtifactVerifier{})
	if err := catalog.EnsureSchema(t.Context()); err != nil {
		t.Fatal(err)
	}
	codeDigest := repeatHex("7")
	closure, err := computerversion.NewCodeClosure(strings.Repeat("8", 40), []computerversion.CodeArtifact{{Name: "sandbox", SHA256: codeDigest, URI: "artifact+sha256://" + codeDigest + "/sql/sandbox"}}, now)
	if err != nil {
		t.Fatal(err)
	}
	programDigest := repeatHex("9")
	program, err := computerversion.NewArtifactProgram([]computerversion.ArtifactProgramEntry{{Kind: "test", ContentSHA256: programDigest, ArtifactURI: "artifact+sha256://" + programDigest + "/sql/program"}}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.PinCode(t.Context(), closure); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.PinArtifactProgram(t.Context(), program); err != nil {
		t.Fatal(err)
	}
	version := computerversion.ComputerVersion{CodeRef: closure.Ref, ArtifactProgramRef: program.Ref}
	ledger := routeledger.NewSQLLedger(productStore.DB())
	if err := ledger.EnsureSchema(t.Context()); err != nil {
		t.Fatal(err)
	}
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.ConfigurePromotionAuthority(t.Context(), publicKey); err != nil {
		t.Fatal(err)
	}
	otherPublicKey, _, _ := ed25519.GenerateKey(nil)
	if err := ledger.ConfigurePromotionAuthority(t.Context(), otherPublicKey); err == nil {
		t.Fatal("SQL route authority accepted replacement promotion key")
	}
	authority, err := NewRouteAuthority(ledger, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if err := authority.SetPromotionAuthorityPublicKey(publicKey); err != nil {
		t.Fatal(err)
	}
	slotID, _ := routeledger.RouteSlotID("owner", "primary")
	verification := validVerificationReceipt(t, version, now)
	approval := OwnerPromotionApproval{RouteSlotID: slotID, OwnerID: "owner", ComputerVersion: version, ConstructionSHA256: verification.ConstructionSHA256, Decision: "approve", KeyID: "owner-key", ApprovedAt: now}
	payload, _ := approval.SigningPayload()
	approval.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, payload))
	frozen, err := authority.prepareBootstrap(slotID, verification, approval, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	acceptance := G3PromotionAcceptance{CandidateID: frozen.ID, RouteSlotID: slotID, OwnerID: "owner", ComputerVersion: version, VerificationRef: verification.ID, CertificateRef: frozen.CertificateEvidence.Ref, BootstrapPlanSHA256: transitionPlanSHA256(frozen.Bootstrap), Decision: "accept", KeyID: "g3-key", AcceptedAt: now.Add(2 * time.Second)}
	payload, _ = acceptance.SigningPayload()
	acceptance.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, payload))
	resolution, err := authority.applyFrozenBootstrap(t.Context(), frozen, acceptance)
	if err != nil {
		t.Fatal(err)
	}
	if resolution.Slot.Generation != 1 || resolution.Slot.Current != version || resolution.TransitionReceipt == nil {
		t.Fatalf("signed SQL bootstrap result = %+v", resolution)
	}
	gate, err := ledger.ResolveAuthorizationEvidence(t.Context(), string(resolution.TransitionReceipt.ApprovalRef))
	if err != nil {
		t.Fatal(err)
	}
	var forgedExecution AuthorizedRouteExecution
	if err := json.Unmarshal(gate.Payload, &forgedExecution); err != nil {
		t.Fatal(err)
	}
	forgedExecution.Plan.IdempotencyKey = "idempotency:forged-post-g3-plan"
	forgedPayload, _ := json.Marshal(forgedExecution)
	forgedGate, err := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidenceApproval, slotID, version, forgedPayload, acceptance.AcceptedAt)
	if err != nil {
		t.Fatal(err)
	}
	forgedCommand := forgedExecution.command(forgedGate)
	if _, _, err := ledger.ApplySignedTransition(t.Context(), forgedCommand, []routeledger.AuthorizationEvidence{forgedGate, frozen.ApprovalEvidence, frozen.CertificateEvidence}); err == nil || !strings.Contains(err.Error(), "does not authorize the transition plan") {
		t.Fatalf("post-G3 plan substitution error = %v", err)
	}
	unchanged, _, err := ledger.Resolve(t.Context(), slotID)
	if err != nil || unchanged.Generation != 1 || unchanged.Current != version {
		t.Fatalf("forged execution mutated route: %+v err=%v", unchanged, err)
	}
}

func TestBuildFrozenRoutePromotionCandidatePlansPromoteAndRollbackWithoutMutation(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	active := computerversion.ComputerVersion{CodeRef: "code:active", ArtifactProgramRef: "artifact:active"}
	candidateVersion := computerversion.ComputerVersion{CodeRef: "code:candidate", ArtifactProgramRef: "artifact:candidate"}
	slotID, err := routeledger.RouteSlotID("owner", "primary")
	if err != nil {
		t.Fatal(err)
	}
	ledger := routeledger.NewMemoryLedger()
	bootstrap := routeledger.TransitionCommand{
		RouteSlotID: slotID, Kind: routeledger.TransitionBootstrap, New: active,
		ApprovalRef: routeledger.ApprovalRef("approval:sha256:" + repeatHex("1")), PromotionCertificateRef: routeledger.PromotionCertificateRef("certificate:sha256:" + repeatHex("2")),
		IdempotencyKey: "idempotency:bootstrap",
	}
	slot, bootstrapReceipt, err := ledger.Transition(context.Background(), bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	verification := validVerificationReceipt(t, candidateVersion, now)
	approval, err := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidenceApproval, slotID, candidateVersion, json.RawMessage(`{"owner":"approved"}`), now)
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := buildFrozenRoutePromotionCandidate(RouteResolution{Slot: slot, TransitionReceipt: &bootstrapReceipt}, verification, approval, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := frozen.Validate(); err != nil {
		t.Fatalf("frozen candidate validation: %v", err)
	}
	crossDesktop := verification
	crossDesktop.Identity.DesktopID = "other"
	crossDesktop.Identity.CandidateID = "other"
	crossDesktop.ID = ""
	crossPayload, err := json.Marshal(crossDesktop)
	if err != nil {
		t.Fatal(err)
	}
	crossDigest := sha256.Sum256(crossPayload)
	crossDesktop.ID = "verification:sha256:" + hex.EncodeToString(crossDigest[:])
	if _, err := buildFrozenRoutePromotionCandidate(RouteResolution{Slot: slot, TransitionReceipt: &bootstrapReceipt}, crossDesktop, approval, now); err == nil || !strings.Contains(err.Error(), "verification identity does not match route slot") {
		t.Fatalf("cross-desktop promotion candidate error = %v", err)
	}
	substitutedCommand := frozen
	substitutedCommand.Promote.RouteSlotID = "computer:owner:other"
	substitutedCommand.ID = ""
	substitutedPayload, err := frozenPromotionPayload(substitutedCommand)
	if err != nil {
		t.Fatal(err)
	}
	substitutedDigest := sha256.Sum256(substitutedPayload)
	substitutedCommand.ID = "route-promotion:sha256:" + hex.EncodeToString(substitutedDigest[:])
	if err := substitutedCommand.Validate(); err == nil || !strings.Contains(err.Error(), "transition commands were substituted") {
		t.Fatalf("route-substituted promotion candidate error = %v", err)
	}
	if frozen.Promote.ExpectedGeneration != slot.Generation || frozen.Rollback.ExpectedGeneration != slot.Generation+1 || frozen.Rollback.RollbackTargetReceiptID != bootstrapReceipt.ID {
		t.Fatalf("incorrect bounded CAS plan: promote=%+v rollback=%+v", frozen.Promote, frozen.Rollback)
	}
	unchanged, _, err := ledger.Resolve(context.Background(), slotID)
	if err != nil || unchanged.Generation != slot.Generation || unchanged.Current != active {
		t.Fatalf("candidate preparation mutated route: %+v err=%v", unchanged, err)
	}
	promoted, _, err := ledger.Transition(context.Background(), frozen.Promote.command(routeledger.ApprovalRef(frozen.ApprovalEvidence.Ref)))
	if err != nil || promoted.Current != candidateVersion || promoted.Generation != slot.Generation+1 {
		t.Fatalf("promote command failed: %+v err=%v", promoted, err)
	}
	rolledBack, _, err := ledger.Transition(context.Background(), frozen.Rollback.command(bootstrapReceipt.ApprovalRef))
	if err != nil || rolledBack.Current != active || rolledBack.Generation != slot.Generation+2 {
		t.Fatalf("rollback command failed: %+v err=%v", rolledBack, err)
	}
	forged := frozen
	forged.Verification.ObservationSHA256 = repeatHex("f")
	if err := forged.Validate(); err == nil {
		t.Fatal("accepted frozen candidate with forged verification receipt")
	}
}

type promotionInputFixture struct {
	codes    map[computerversion.CodeRef]computerversion.CodeClosure
	programs map[computerversion.ArtifactProgramRef]computerversion.ArtifactProgram
}

func (f promotionInputFixture) ResolveCode(_ context.Context, ref computerversion.CodeRef) (computerversion.CodeClosure, error) {
	value, ok := f.codes[ref]
	if !ok {
		return computerversion.CodeClosure{}, computerversion.ErrInputNotFound
	}
	return value, nil
}
func (f promotionInputFixture) ResolveArtifactProgram(_ context.Context, ref computerversion.ArtifactProgramRef) (computerversion.ArtifactProgram, error) {
	value, ok := f.programs[ref]
	if !ok {
		return computerversion.ArtifactProgram{}, computerversion.ErrInputNotFound
	}
	return value, nil
}

func signedOwnerApprovalEvidence(t *testing.T, privateKey ed25519.PrivateKey, slotID string, verification computerversion.RealizationVerificationReceipt) routeledger.AuthorizationEvidence {
	t.Helper()
	approval := OwnerPromotionApproval{RouteSlotID: slotID, OwnerID: verification.Identity.OwnerID, ComputerVersion: verification.Version, ConstructionSHA256: verification.ConstructionSHA256, Decision: "approve", KeyID: "owner-key", ApprovedAt: verification.VerifiedAt}
	payload, err := approval.SigningPayload()
	if err != nil {
		t.Fatal(err)
	}
	approval.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, payload))
	payload, err = json.Marshal(approval)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidenceApproval, slotID, verification.Version, payload, approval.ApprovedAt)
	if err != nil {
		t.Fatal(err)
	}
	return evidence
}

func TestSignedFrozenBootstrapIsOnlyFirstRouteCASPath(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 30, 0, 0, time.UTC)
	digest := strings.Repeat("c", 64)
	code, err := computerversion.NewCodeClosure(strings.Repeat("3", 40), []computerversion.CodeArtifact{{Name: "sandbox-runtime.tar", SHA256: digest, URI: "nix-store+sha256://" + digest + "/nix/store/runtime"}}, now)
	if err != nil {
		t.Fatal(err)
	}
	program, err := computerversion.NewArtifactProgram([]computerversion.ArtifactProgramEntry{{Kind: "base_journal", ContentSHA256: digest, ArtifactURI: "artifact+sha256://" + digest + "/state"}}, now)
	if err != nil {
		t.Fatal(err)
	}
	version := computerversion.ComputerVersion{CodeRef: code.Ref, ArtifactProgramRef: program.Ref}
	slotID, _ := routeledger.RouteSlotID("owner", "primary")
	verification := validVerificationReceipt(t, version, now)
	privateKey := ed25519.NewKeyFromSeed([]byte(strings.Repeat("j", ed25519.SeedSize)))
	ownerEvidence := signedOwnerApprovalEvidence(t, privateKey, slotID, verification)
	frozen, err := buildFrozenRouteBootstrapCandidate(slotID, verification, ownerEvidence, now)
	if err != nil {
		t.Fatal(err)
	}
	ledger := routeledger.NewMemoryLedger()
	authority, err := newMemoryRouteAuthority(ledger, promotionInputFixture{codes: map[computerversion.CodeRef]computerversion.CodeClosure{version.CodeRef: code}, programs: map[computerversion.ArtifactProgramRef]computerversion.ArtifactProgram{version.ArtifactProgramRef: program}})
	if err != nil {
		t.Fatal(err)
	}
	if err := authority.SetPromotionAuthorityPublicKey(privateKey.Public().(ed25519.PublicKey)); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.Transition(t.Context(), frozen.Bootstrap.command(routeledger.ApprovalRef(frozen.ApprovalEvidence.Ref))); err == nil {
		t.Fatal("raw bootstrap bypassed signed frozen path")
	}
	unsignedEvidence, err := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidenceApproval, slotID, version, json.RawMessage(`{"signed":"owner"}`), now)
	if err != nil {
		t.Fatal(err)
	}
	unsignedFrozen, err := buildFrozenRouteBootstrapCandidate(slotID, verification, unsignedEvidence, now)
	if err != nil {
		t.Fatal(err)
	}
	unsignedAcceptance := G3PromotionAcceptance{CandidateID: unsignedFrozen.ID, RouteSlotID: slotID, OwnerID: "owner", ComputerVersion: version, VerificationRef: unsignedFrozen.Verification.ID, CertificateRef: unsignedFrozen.CertificateEvidence.Ref, Decision: "accept", KeyID: "g3-owner-key", AcceptedAt: now.Add(time.Minute), BootstrapPlanSHA256: transitionPlanSHA256(unsignedFrozen.Bootstrap)}
	unsignedPayload, _ := unsignedAcceptance.SigningPayload()
	unsignedAcceptance.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, unsignedPayload))
	if _, err := authority.applyFrozenBootstrap(t.Context(), unsignedFrozen, unsignedAcceptance); err == nil {
		t.Fatal("unsigned owner approval reached bootstrap")
	}
	handler := NewHandler(NewOwnershipRegistry("http://sandbox"))
	handler.SetRouteAuthority(authority)
	requestPayload, err := json.Marshal(applyFrozenBootstrapRequest{Candidate: unsignedFrozen, Acceptance: unsignedAcceptance})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/internal/vmctl/computer-version-routes/apply-bootstrap", bytes.NewReader(requestPayload))
	request.Header.Set("X-Internal-Caller", "true")
	response := httptest.NewRecorder()
	handler.HandleApplyFrozenComputerVersionBootstrap(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("unsigned HTTP bootstrap status=%d body=%s", response.Code, response.Body.String())
	}
	if _, _, err := ledger.Resolve(t.Context(), slotID); !errors.Is(err, routeledger.ErrSlotNotFound) {
		t.Fatalf("unsigned HTTP bootstrap mutated route: %v", err)
	}
	forgedCertificate, err := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidencePromotionCertificate, slotID, version, json.RawMessage(`{"kind":"substituted"}`), now)
	if err != nil {
		t.Fatal(err)
	}
	forgedCandidate := frozen
	forgedCandidate.CertificateEvidence = forgedCertificate
	forgedCandidate.Bootstrap.PromotionCertificateRef = routeledger.PromotionCertificateRef(forgedCertificate.Ref)
	forgedCandidate.ID = ""
	forgedPayload, _ := frozenBootstrapPayload(forgedCandidate)
	forgedDigest := sha256.Sum256(forgedPayload)
	forgedCandidate.ID = "route-bootstrap:sha256:" + hex.EncodeToString(forgedDigest[:])
	if err := forgedCandidate.Validate(); err == nil || !strings.Contains(err.Error(), "certificate evidence payload mismatch") {
		t.Fatalf("substituted bootstrap certificate error = %v", err)
	}
	acceptance := G3PromotionAcceptance{CandidateID: frozen.ID, RouteSlotID: slotID, OwnerID: "owner", ComputerVersion: version, VerificationRef: frozen.Verification.ID, CertificateRef: frozen.CertificateEvidence.Ref, Decision: "accept", KeyID: "g3-owner-key", AcceptedAt: now.Add(time.Minute), BootstrapPlanSHA256: transitionPlanSHA256(frozen.Bootstrap)}
	payload, _ := acceptance.SigningPayload()
	wrongKey := ed25519.NewKeyFromSeed([]byte(strings.Repeat("x", ed25519.SeedSize)))
	wrongAcceptance := acceptance
	wrongAcceptance.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(wrongKey, payload))
	if _, err := authority.applyFrozenBootstrap(t.Context(), frozen, wrongAcceptance); err == nil {
		t.Fatal("wrong-key G3 acceptance reached bootstrap")
	}
	earlyAcceptance := acceptance
	earlyAcceptance.AcceptedAt = frozen.PreparedAt
	earlyPayload, _ := earlyAcceptance.SigningPayload()
	earlyAcceptance.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, earlyPayload))
	if _, err := authority.applyFrozenBootstrap(t.Context(), frozen, earlyAcceptance); err == nil {
		t.Fatal("pre-freeze G3 acceptance reached bootstrap")
	}
	acceptance.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, payload))
	resolution, err := authority.applyFrozenBootstrap(t.Context(), frozen, acceptance)
	if err != nil || resolution.Slot.Generation != 1 || resolution.Slot.Current != version {
		t.Fatalf("signed bootstrap: %+v err=%v", resolution, err)
	}
	gateEvidence, err := ledger.ResolveAuthorizationEvidence(t.Context(), string(resolution.TransitionReceipt.ApprovalRef))
	if err != nil {
		t.Fatal(err)
	}
	var execution AuthorizedRouteExecution
	if err := json.Unmarshal(gateEvidence.Payload, &execution); err != nil {
		t.Fatal(err)
	}
	if execution.Action != string(routeledger.TransitionBootstrap) || !transitionReceiptMatchesCommand(*resolution.TransitionReceipt, execution.command(gateEvidence)) {
		t.Fatalf("bootstrap receipt did not join authorized execution: execution=%+v receipt=%+v", execution, resolution.TransitionReceipt)
	}
	if _, err := authority.applyFrozenBootstrap(t.Context(), frozen, acceptance); !errors.Is(err, routeledger.ErrStaleTransition) {
		t.Fatalf("replayed bootstrap error = %v", err)
	}
}

func TestSignedFrozenPromotionIsOnlyVmctlPromoteAndRollbackPath(t *testing.T) {
	now := time.Date(2026, 7, 16, 13, 0, 0, 0, time.UTC)
	makeInputs := func(source, artifactByte string) (computerversion.CodeClosure, computerversion.ArtifactProgram) {
		digest := strings.Repeat(artifactByte, 64)
		code, err := computerversion.NewCodeClosure(strings.Repeat(source, 40), []computerversion.CodeArtifact{{Name: "sandbox-runtime.tar", SHA256: digest, URI: "nix-store+sha256://" + digest + "/nix/store/runtime"}}, now)
		if err != nil {
			t.Fatal(err)
		}
		program, err := computerversion.NewArtifactProgram([]computerversion.ArtifactProgramEntry{{Kind: "base_journal", ContentSHA256: digest, ArtifactURI: "artifact+sha256://" + digest + "/state"}}, now)
		if err != nil {
			t.Fatal(err)
		}
		return code, program
	}
	activeCode, activeProgram := makeInputs("1", "a")
	candidateCode, candidateProgram := makeInputs("2", "b")
	active := computerversion.ComputerVersion{CodeRef: activeCode.Ref, ArtifactProgramRef: activeProgram.Ref}
	candidateVersion := computerversion.ComputerVersion{CodeRef: candidateCode.Ref, ArtifactProgramRef: candidateProgram.Ref}
	slotID, _ := routeledger.RouteSlotID("owner", "primary")
	ledger := routeledger.NewMemoryLedger()
	baseApproval, err := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidenceApproval, slotID, active, json.RawMessage(`{"base":"approval"}`), now)
	if err != nil {
		t.Fatal(err)
	}
	baseCertificate, err := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidencePromotionCertificate, slotID, active, json.RawMessage(`{"base":"certificate"}`), now)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap := routeledger.TransitionCommand{RouteSlotID: slotID, Kind: routeledger.TransitionBootstrap, New: active, ApprovalRef: routeledger.ApprovalRef(baseApproval.Ref), PromotionCertificateRef: routeledger.PromotionCertificateRef(baseCertificate.Ref), IdempotencyKey: "idempotency:signed-bootstrap"}
	slot, receipt, err := ledger.TransitionWithEvidence(t.Context(), bootstrap, []routeledger.AuthorizationEvidence{baseApproval, baseCertificate})
	if err != nil {
		t.Fatal(err)
	}
	verification := validVerificationReceipt(t, candidateVersion, now)
	privateKey := ed25519.NewKeyFromSeed([]byte(strings.Repeat("k", ed25519.SeedSize)))
	ownerEvidence := signedOwnerApprovalEvidence(t, privateKey, slotID, verification)
	frozen, err := buildFrozenRoutePromotionCandidate(RouteResolution{Slot: slot, TransitionReceipt: &receipt}, verification, ownerEvidence, now)
	if err != nil {
		t.Fatal(err)
	}
	authority, err := newMemoryRouteAuthority(ledger, promotionInputFixture{codes: map[computerversion.CodeRef]computerversion.CodeClosure{active.CodeRef: activeCode, candidateVersion.CodeRef: candidateCode}, programs: map[computerversion.ArtifactProgramRef]computerversion.ArtifactProgram{active.ArtifactProgramRef: activeProgram, candidateVersion.ArtifactProgramRef: candidateProgram}})
	if err != nil {
		t.Fatal(err)
	}
	if err := authority.SetPromotionAuthorityPublicKey(privateKey.Public().(ed25519.PublicKey)); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.Transition(t.Context(), frozen.Promote.command(routeledger.ApprovalRef(frozen.ApprovalEvidence.Ref))); err == nil {
		t.Fatal("raw promote bypassed signed frozen path")
	}
	acceptance := G3PromotionAcceptance{CandidateID: frozen.ID, RouteSlotID: slotID, OwnerID: "owner", ComputerVersion: candidateVersion, VerificationRef: frozen.Verification.ID, CertificateRef: frozen.CertificateEvidence.Ref, Decision: "accept", KeyID: "g3-owner-key", AcceptedAt: now.Add(time.Minute), PromotePlanSHA256: transitionPlanSHA256(frozen.Promote), RollbackPlanSHA256: transitionPlanSHA256(frozen.Rollback)}
	payload, _ := acceptance.SigningPayload()
	acceptance.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, payload))
	approvalValue, err := decodeOwnerApproval(frozen.ApprovalEvidence)
	if err != nil {
		t.Fatal(err)
	}
	_, staleRollbackGate, err := newAuthorizedRouteExecution(frozen.ID, string(routeledger.TransitionRollback), approvalValue, frozen.Verification, frozen.CertificateEvidence.Ref, acceptance, frozen.PreparedAt, frozen.Rollback)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.applyFrozenPromotion(t.Context(), frozen, acceptance, true); !errors.Is(err, routeledger.ErrStaleTransition) {
		t.Fatalf("pre-promote rollback error = %v", err)
	}
	if _, err := ledger.ResolveAuthorizationEvidence(t.Context(), staleRollbackGate.Ref); err == nil {
		t.Fatal("stale rollback pinned evidence before refusal")
	}
	promoted, err := authority.applyFrozenPromotion(t.Context(), frozen, acceptance, false)
	if err != nil || promoted.Slot.Current != candidateVersion {
		t.Fatalf("signed promote: %+v err=%v", promoted, err)
	}
	promoteGate, err := ledger.ResolveAuthorizationEvidence(t.Context(), string(promoted.TransitionReceipt.ApprovalRef))
	if err != nil {
		t.Fatal(err)
	}
	var promoteExecution AuthorizedRouteExecution
	if err := json.Unmarshal(promoteGate.Payload, &promoteExecution); err != nil {
		t.Fatal(err)
	}
	if promoteExecution.Action != string(routeledger.TransitionPromote) || !transitionReceiptMatchesCommand(*promoted.TransitionReceipt, promoteExecution.command(promoteGate)) {
		t.Fatalf("promotion receipt did not join authorized execution: execution=%+v receipt=%+v", promoteExecution, promoted.TransitionReceipt)
	}
	rolledBack, err := authority.applyFrozenPromotion(t.Context(), frozen, acceptance, true)
	if err != nil || rolledBack.Slot.Current != active {
		t.Fatalf("signed rollback: %+v err=%v", rolledBack, err)
	}
}

func validVerificationReceipt(t *testing.T, version computerversion.ComputerVersion, now time.Time) computerversion.RealizationVerificationReceipt {
	t.Helper()
	identity := computerversion.ConstructionIdentity{RealizationID: "candidate-realization", ComputerKind: "candidate", OwnerID: "owner", DesktopID: "primary", CandidateID: "primary"}
	plan := diskinstantiation.Plan{RealizationID: identity.RealizationID, DeviceID: "data", LogicalBytes: 32 << 30, Filesystem: diskinstantiation.FilesystemContract{Type: diskinstantiation.FilesystemExt4, Label: "choir-data", BlockSizeBytes: 4096}, Allocation: diskinstantiation.AllocationContract{Mode: diskinstantiation.AllocationSparse, MaxAllocatedBytes: 2 << 30, MinimumAvailableBytes: 2 << 30}}
	disk, err := diskinstantiation.FinalizeReceipt(diskinstantiation.Receipt{Backend: diskinstantiation.Ext4BackendName, RealizationID: identity.RealizationID, DeviceID: "data", DevicePath: "/vm-state/candidate-realization/data.img", Geometry: diskinstantiation.GeometryReceipt{FilesystemType: diskinstantiation.FilesystemExt4, FilesystemLabel: "choir-data", PartitionLayout: diskinstantiation.PartitionLayoutNone, DeviceLogicalBytes: 32 << 30, FilesystemBytes: 32 << 30, FilesystemBlockSize: 4096, FilesystemBlocks: (32 << 30) / 4096, AllocatedBytes: 128 << 20}, CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	receipt := computerversion.RealizationVerificationReceipt{
		Verifier: computerversion.IndependentRealizationVerifierName, Version: version, Identity: identity, DiskPlan: plan, Disk: disk, RealizationID: identity.RealizationID,
		ConstructionSHA256: repeatHex("a"), ObservationSHA256: repeatHex("b"), DiskReceiptID: disk.ID,
		RuntimeGeometry: diskinstantiation.RuntimeGeometryReceipt{FilesystemBytes: 32 << 30, FilesystemBlockSize: 4096, AvailableBytes: 3 << 30},
		VMID:            identity.RealizationID, Epoch: 7, VerifiedAt: now,
	}
	payload, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(payload)
	receipt.ID = "verification:sha256:" + hex.EncodeToString(digest[:])
	return receipt
}

func repeatHex(value string) string {
	out := ""
	for len(out) < 64 {
		out += value
	}
	return out[:64]
}
