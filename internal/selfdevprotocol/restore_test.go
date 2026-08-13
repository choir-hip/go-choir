package selfdevprotocol

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
)

func TestCheckpointFromRequestRequiresVMLocalWitnessAndFrontendIdentity(t *testing.T) {
	checkpoint := testCheckpointRequest(t)
	if _, _, err := CheckpointFromRequest(checkpoint); err != nil {
		t.Fatalf("valid restore bindings refused: %v", err)
	}

	missingWitness := checkpoint
	missingWitness.VMLocalContentWitness = VMLocalContentWitness{}
	if _, _, err := CheckpointFromRequest(missingWitness); err == nil {
		t.Fatal("checkpoint accepted a request with no VM-local content witness")
	}

	missingFrontend := checkpoint
	missingFrontend.FrontendIdentity = FrontendIdentity{}
	if _, _, err := CheckpointFromRequest(missingFrontend); err == nil {
		t.Fatal("checkpoint accepted a request with an underivable SPA")
	}
}

func TestCheckpointFromRequestRefusesPlatformOrCycleWitness(t *testing.T) {
	for _, database := range []string{RestoreScopePlatform, RestoreScopeCycle} {
		checkpoint := testCheckpointRequest(t)
		checkpoint.VMLocalContentWitness.Database = database
		if _, _, err := CheckpointFromRequest(checkpoint); err == nil {
			t.Fatalf("checkpoint accepted a %s witness", database)
		}
	}
}

func TestCheckpointFromRequestRefusesIncompleteWitness(t *testing.T) {
	checkpoint := testCheckpointRequest(t)
	checkpoint.VMLocalContentWitness.DerivabilityDigest = ""
	if _, _, err := CheckpointFromRequest(checkpoint); err == nil {
		t.Fatal("checkpoint accepted underivable VM-local rows")
	}

	checkpoint = testCheckpointRequest(t)
	checkpoint.VMLocalContentWitness.Tables = map[string]string{}
	if _, _, err := CheckpointFromRequest(checkpoint); err == nil {
		t.Fatal("checkpoint accepted a witness with no table hashes")
	}

	checkpoint = testCheckpointRequest(t)
	checkpoint.FrontendIdentity.Derivation = ""
	if _, _, err := CheckpointFromRequest(checkpoint); err == nil {
		t.Fatal("checkpoint accepted an underivable SPA derivation")
	}

	checkpoint = testCheckpointRequest(t)
	checkpoint.FrontendIdentity.Derivation = FrontendDerivationRelease
	checkpoint.FrontendIdentity.ReleaseDigest = strings.Repeat("0", 64)
	if _, _, err := CheckpointFromRequest(checkpoint); err == nil {
		t.Fatal("checkpoint accepted a frontend identity that does not join the release")
	}
}

func TestWitnessFromObservationSetsRefusesLiveOnlyRowsAndOutOfScopeStores(t *testing.T) {
	live := observationSet("choir", "agents", "a", "b")
	replay := live
	equivalent := computerversion.EquivalenceResult{Status: computerversion.EquivalenceEquivalent}
	witness, err := WitnessFromObservationSets(live, replay, equivalent)
	if err != nil {
		t.Fatalf("equivalent VM-local observations refused: %v", err)
	}
	if err := witness.Validate(); err != nil {
		t.Fatal(err)
	}

	mismatched := computerversion.EquivalenceResult{Status: computerversion.EquivalenceNotEquivalent, Differences: []computerversion.Difference{{
		Kind: computerversion.ObservationDoltHead, Key: "dolt:choir:table:agents", Left: "live-only", Right: "", Reason: "live-only row",
	}}}
	if _, err := WitnessFromObservationSets(live, observationSet("choir", "agents", "a", "c"), mismatched); err == nil {
		t.Fatal("witness accepted live-only behavior-bearing rows")
	}

	platform := observationSet("platform", "computers", "a", "b")
	if _, err := WitnessFromObservationSets(platform, platform, equivalent); err == nil {
		t.Fatal("witness accepted platform store observations")
	}
}

func TestRestoreFromRequestRefusesPlatformAndCycleOperands(t *testing.T) {
	checkpoint, _, err := CheckpointFromRequest(testCheckpointRequest(t))
	if err != nil {
		t.Fatal(err)
	}
	valid := RestoreRequest{
		ComputerID: checkpoint.Request.ComputerID, CheckpointDigest: checkpoint.Digest,
		OperandScopes: []string{RestoreScopeVMLocal, RestoreScopeFrontend},
	}
	if err := RestoreFromRequest(valid, checkpoint); err != nil {
		t.Fatalf("in-scope restore operand refused: %v", err)
	}
	for _, scope := range []string{RestoreScopePlatform, RestoreScopeCycle} {
		request := valid
		request.OperandScopes = []string{RestoreScopeVMLocal, RestoreScopeFrontend, scope}
		if err := RestoreFromRequest(request, checkpoint); err == nil {
			t.Fatalf("restore accepted a %s operand", scope)
		}
	}
	missingFrontend := valid
	missingFrontend.OperandScopes = []string{RestoreScopeVMLocal}
	if err := RestoreFromRequest(missingFrontend, checkpoint); err == nil {
		t.Fatal("restore accepted an operand without the computer-surface frontend")
	}
}

func testCheckpointRequest(t *testing.T) CheckpointRequest {
	t.Helper()
	digest := func(value byte) string { return strings.Repeat(string(value), 64) }
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "verifier-control", KeyID: "verifier-test"}, PrivateKey: privateKey}
	request := VerifierCertificateRequest{
		Version: 1, ComputerID: "computer-test", OperationID: "operation-test",
		BundleDigest: digest('a'), VerificationEventDigest: digest('b'), VerifierEvidenceRefs: []string{digest('b')},
		DecisionEventHead: digest('c'), CodeRef: "code:sha256:" + digest('d'), ArtifactProgramRef: "artifact-program:sha256:" + digest('e'),
		ReleaseDigest: digest('f'), Decision: "pass",
	}
	certificate, err := NewVerifierCertificate(request, key, time.Date(2026, 7, 19, 6, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	response := VerifierCertificateResponse{Request: request, Certificate: certificate, PublicKey: base64.RawStdEncoding.EncodeToString(publicKey)}
	certificateJSON, _ := computerevent.CanonicalJSON(certificate)
	return CheckpointRequest{
		ComputerID: request.ComputerID, IdempotencyKey: "checkpoint-test",
		ComputerVersion:   computerversion.ComputerVersion{CodeRef: computerversion.CodeRef(request.CodeRef), ArtifactProgramRef: computerversion.ArtifactProgramRef(request.ArtifactProgramRef)},
		AcceptedEventHead: digest('1'), EffectiveEventHead: digest('1'), EffectiveStateCommitment: digest('2'), EventHeadReceiptID: "receipt-test",
		ReleaseDigest: request.ReleaseDigest, ReconstructionDigest: digest('3'), MaterializationReceiptDigest: digest('4'),
		VerifierCertificateDigest: computerevent.DigestBytes(certificateJSON), VerifierCertificate: response, ReducerVersion: 1,
		VMLocalContentWitness: VMLocalContentWitness{
			Database: "choir", ContentRoot: digest('7'), DoltHead: digest('8'), DerivabilityDigest: digest('9'),
			Schema: map[string]string{"agents": digest('a')},
			Tables: map[string]string{"agents": digest('b')},
		},
		FrontendIdentity: FrontendIdentity{Digest: digest('c'), Derivation: FrontendDerivationExplicit},
	}
}

func observationSet(database, table, schemaSeed, tableSeed string) computerversion.ObservationSet {
	digest := func(seed string) string { return strings.Repeat(seed[:1], 64) }
	return computerversion.ObservationSet{
		Name:     "test-dolt",
		Version:  computerversion.ComputerVersion{CodeRef: "code:test", ArtifactProgramRef: "artifact:test"},
		Required: []computerversion.ObservationKind{computerversion.ObservationDoltHead},
		Observations: []computerversion.Observation{
			{Kind: computerversion.ObservationDoltHead, Key: "dolt:" + database + ":head", Value: "sha256:" + digest("e")},
			{Kind: computerversion.ObservationDoltHead, Key: "dolt:" + database + ":content_root", Value: "sha256:" + digest("d")},
			{Kind: computerversion.ObservationDoltHead, Key: "dolt:" + database + ":schema:" + table, Value: "sha256:" + digest(schemaSeed)},
			{Kind: computerversion.ObservationDoltHead, Key: "dolt:" + database + ":table:" + table, Value: "sha256:" + digest(tableSeed)},
		},
	}
}

func TestFrontendIdentityFromReleaseFilesRefusesMissingSPA(t *testing.T) {
	digest := func(value byte) string { return strings.Repeat(string(value), 64) }
	if _, err := FrontendIdentityFromReleaseFiles(digest('f'), []ReleaseFile{{Path: "bin/choir", SHA256: digest('1')}}); err == nil {
		t.Fatal("frontend identity accepted a release with no computer-surface SPA")
	}
	identity, err := FrontendIdentityFromReleaseFiles(digest('f'), []ReleaseFile{
		{Path: "bin/choir", SHA256: digest('1')},
		{Path: "frontend/index.html", SHA256: digest('2')},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := identity.Validate(digest('f')); err != nil {
		t.Fatal(err)
	}
	if identity.Derivation != FrontendDerivationRelease || identity.ReleaseDigest != digest('f') {
		t.Fatalf("unexpected identity %#v", identity)
	}
}
