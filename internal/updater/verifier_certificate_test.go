package updater

import (
	"crypto/ed25519"
	"crypto/rand"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

func TestUpdaterVerifierCertificateIsDurableAcrossRetry(t *testing.T) {
	_, updaterPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, verifierPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "updater")
	engine, err := New(root, "computer-test", "realization-test", &fakeServiceManager{}, fakeHealthProber{}, computerevent.SigningKey{
		SignerRef: computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "updater-test"}, PrivateKey: updaterPrivate,
	})
	if err != nil {
		t.Fatal(err)
	}
	key := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "verifier-control", KeyID: "verifier-test"}, PrivateKey: verifierPrivate}
	digest := strings.Repeat("a", 64)
	request := selfdevprotocol.VerifierCertificateRequest{
		Version: 1, ComputerID: "computer-test", OperationID: "operation-test", BundleDigest: digest,
		VerificationEventDigest: digest, VerifierEvidenceRefs: []string{digest}, DecisionEventHead: digest,
		CodeRef: "code:sha256:" + digest, ArtifactProgramRef: "artifact-program:sha256:" + digest,
		ReleaseDigest: digest, Decision: "pass",
	}
	first, err := engine.SignVerifierCertificate(request, key, time.Date(2026, 7, 19, 2, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	retry, err := engine.SignVerifierCertificate(request, key, time.Date(2026, 7, 19, 3, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	firstBytes, firstErr := first.Certificate.CanonicalBytes()
	retryBytes, retryErr := retry.Certificate.CanonicalBytes()
	if firstErr != nil || retryErr != nil || string(firstBytes) != string(retryBytes) || first.Certificate.ReceiptID != retry.Certificate.ReceiptID {
		t.Fatalf("durable verifier certificate changed: %v/%v first=%s retry=%s", firstErr, retryErr, firstBytes, retryBytes)
	}
}
