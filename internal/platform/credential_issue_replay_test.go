package platform

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

// vmctl retries credential issuance with the same idempotency key when a
// request times out under host load. The issuer used to fold its own clock
// (expires_at) into the request commitment, so a retry after a committed
// first attempt was refused as an idempotency conflict and the new computer
// went to `failed` (docs/problems/texture-zombie-activations-revising-forever-
// 2026-10-09.md, signup failure). Failure modes pinned: a same-key retry
// refused; a retry minting a different envelope (second bearer); a same-key
// request for another realization accepted.
func TestCredentialIssueReplaysSameKeyRetry(t *testing.T) {
	store, root := openTestPlatformStore(t)
	service := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	handler := NewHandler(service)
	issue := func(realizationID string) (int, computerCredentialIssueResponse) {
		t.Helper()
		body, _ := json.Marshal(computerCredentialIssueRequest{ComputerID: "computer-retry", RealizationID: realizationID, IdempotencyKey: "guest-credential:retry:1"})
		request := httptest.NewRequest(http.MethodPost, "/internal/computers/credentials/issue", bytes.NewReader(body))
		request.Header.Set("X-Internal-Caller", "true")
		response := httptest.NewRecorder()
		handler.HandleComputerCredentialIssue(response, request)
		var out computerCredentialIssueResponse
		_ = json.Unmarshal(response.Body.Bytes(), &out)
		return response.Code, out
	}
	firstCode, first := issue("realization-retry")
	if firstCode != http.StatusCreated {
		t.Fatalf("first issue status=%d", firstCode)
	}
	time.Sleep(5 * time.Millisecond) // the retry arrives later on the issuer's clock
	retryCode, retry := issue("realization-retry")
	if retryCode != http.StatusCreated {
		t.Fatalf("same-key retry status=%d, want replay", retryCode)
	}
	firstJSON, _ := json.Marshal(first.Envelope)
	retryJSON, _ := json.Marshal(retry.Envelope)
	if !bytes.Equal(firstJSON, retryJSON) {
		t.Fatalf("retry minted a different envelope:\nfirst=%s\nretry=%s", firstJSON, retryJSON)
	}
	if otherCode, _ := issue("realization-other"); otherCode == http.StatusCreated {
		t.Fatal("same key accepted for a different realization")
	}
}
