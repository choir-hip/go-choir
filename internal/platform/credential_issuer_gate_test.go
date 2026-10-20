package platform

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// Credential issuance needs the issuer credential as well as the internal
// one (blast-radius design F1). Failure modes pinned: the shared internal
// credential alone mints an envelope in enforce; the right pair is refused
// (every computer would fail to boot); observe refuses anything.
func TestCredentialIssueNeedsIssuerCredential(t *testing.T) {
	const internalToken = "0123456789abcdef0123456789abcdef0123456789abcdef"
	const issuerToken = "fedcba9876543210fedcba9876543210fedcba9876543210"
	store, root := openTestPlatformStore(t)
	service := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	handler := hostauth.Middleware(http.HandlerFunc(NewHandler(service).HandleComputerCredentialIssue))
	issue := func(realizationID string, issuer string) int {
		t.Helper()
		body, _ := json.Marshal(computerCredentialIssueRequest{ComputerID: "computer-issuer-gate", RealizationID: realizationID, IdempotencyKey: "guest-credential:" + realizationID})
		request := httptest.NewRequest(http.MethodPost, "/internal/computers/credentials/issue", bytes.NewReader(body))
		request.Header.Set("X-Internal-Caller", "true")
		request.Header.Set(hostauth.Header, internalToken)
		if issuer != "" {
			request.Header.Set(hostauth.IssuerHeader, issuer)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response.Code
	}
	defer hostauth.SetForTesting(hostauth.ModeEnforce, internalToken)()

	restore := hostauth.SetIssuerForTesting(hostauth.ModeEnforce, issuerToken)
	if code := issue("realization-a", ""); code != http.StatusForbidden {
		t.Fatalf("internal credential alone: status=%d, want 403", code)
	}
	if code := issue("realization-b", internalToken); code != http.StatusForbidden {
		t.Fatalf("internal token presented as issuer: status=%d, want 403", code)
	}
	if code := issue("realization-c", issuerToken); code != http.StatusCreated {
		t.Fatalf("internal plus issuer credential: status=%d, want 201", code)
	}
	restore()

	defer hostauth.SetIssuerForTesting(hostauth.ModeObserve, issuerToken)()
	if code := issue("realization-d", ""); code != http.StatusCreated {
		t.Fatalf("observe without issuer credential: status=%d, want 201 (observe never refuses)", code)
	}
}
