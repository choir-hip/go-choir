package receiptsigner

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	selfdevprotocol "github.com/yusefmosiah/go-choir/internal/verifierprotocol"
)

func TestGuestSignerAllowsOnlyTypedReceiptsAndPersistsRetry(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	handler := testHandler(t, ModeGuestCore, now)
	request := SignReceiptRequest{ReceiptKind: "HealthReceipt", Issuer: "choir-updater", IssuedAt: now.Format(time.RFC3339Nano), KindFields: map[string]any{
		"computer_id": "computer-1", "realization_id": "realization-1", "release_digest": strings.Repeat("a", 64),
		"probe_contract_digest": strings.Repeat("b", 64), "started_at": now.Add(-time.Minute).Format(time.RFC3339Nano),
		"completed_at": now.Format(time.RFC3339Nano), "outcome": "healthy", "observation_artifact_digests": []string{strings.Repeat("c", 64)},
	}}
	first := invoke(t, handler, "/v1/sign-receipt", request)
	second := invoke(t, handler, "/v1/sign-receipt", request)
	if first.Code != http.StatusOK || second.Code != http.StatusOK || !bytes.Equal(first.Body.Bytes(), second.Body.Bytes()) {
		t.Fatalf("durable retry mismatch: first=%d second=%d", first.Code, second.Code)
	}
	refused := invoke(t, handler, "/v1/sign-verifier-certificate", map[string]any{})
	if refused.Code != http.StatusNotFound {
		t.Fatalf("guest signer exposed verifier authority: %d", refused.Code)
	}
}

func TestVerifierSignerCannotSignUpdaterReceipt(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	handler := testHandler(t, ModeVerifier, now)
	request := selfdevprotocol.VerifierCertificateRequest{
		Version: 1, ComputerID: "computer-1", OperationID: "operation-1", BundleDigest: strings.Repeat("a", 64),
		VerificationEventDigest: strings.Repeat("b", 64), VerifierEvidenceRefs: []string{strings.Repeat("c", 64)},
		DecisionEventHead: strings.Repeat("d", 64), CodeRef: "code:sha256:" + strings.Repeat("e", 64),
		ArtifactProgramRef: "artifact-program:sha256:" + strings.Repeat("f", 64),
		ReleaseDigest:      strings.Repeat("1", 64), Decision: "pass",
	}
	response := invoke(t, handler, "/v1/sign-verifier-certificate", request)
	if response.Code != http.StatusOK {
		t.Fatalf("verifier certificate refused: %d %s", response.Code, response.Body.String())
	}
	var certificate selfdevprotocol.VerifierCertificateResponse
	if json.Unmarshal(response.Body.Bytes(), &certificate) != nil || !reflect.DeepEqual(certificate.Request, request) || selfdevprotocol.VerifyVerifierCertificate(certificate) != nil {
		t.Fatal("verifier certificate binding invalid")
	}
	refused := invoke(t, handler, "/v1/sign-receipt", SignReceiptRequest{})
	if refused.Code != http.StatusNotFound {
		t.Fatalf("verifier signer exposed updater authority: %d", refused.Code)
	}
}

func testHandler(t *testing.T, mode string, now time.Time) *Handler {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: mode, KeyID: mode + "-test"}, PrivateKey: privateKey}
	handler, err := NewHandler(mode, "computer-1", t.TempDir(), key)
	if err != nil {
		t.Fatal(err)
	}
	handler.now = func() time.Time { return now }
	return handler
}

func invoke(t *testing.T, handler http.Handler, path string, value any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := computerevent.CanonicalJSON(value)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
