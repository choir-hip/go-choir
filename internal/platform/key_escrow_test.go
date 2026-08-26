package platform

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/keyescrow"
)

func TestKeyEscrowFullFlow(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	ctx := context.Background()
	privateKey, publicKey, err := keyescrow.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	dek := bytes.Repeat([]byte{0x42}, 32)
	wrapped, err := keyescrow.SealDEK(publicKey, "computer-key-escrow", dek)
	if err != nil {
		t.Fatal(err)
	}
	wrappedJSON, err := json.Marshal(wrapped)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertKeyEscrow(ctx, wrapped.ComputerID, wrapped.Protector, wrappedJSON, wrapped.KeyDigest); err != nil {
		t.Fatal(err)
	}
	gotWrapped, digest, err := store.GetKeyEscrow(ctx, wrapped.ComputerID, wrapped.Protector)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotWrapped, wrappedJSON) || digest != wrapped.KeyDigest {
		t.Fatal("stored escrow record differs")
	}
	statuses, err := store.ListKeyEscrowStatus(ctx, wrapped.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 1 || statuses[0].Protector != keyescrow.ProtectorCustodian || statuses[0].KeyDigest != wrapped.KeyDigest || statuses[0].EscrowedAt.IsZero() {
		t.Fatalf("unexpected escrow status: %#v", statuses)
	}

	request, err := store.CreateKeyUnwrapRequest(ctx, wrapped.ComputerID, "requester", "recovery", "idempotency-key")
	if err != nil {
		t.Fatal(err)
	}
	idempotent, err := store.CreateKeyUnwrapRequest(ctx, wrapped.ComputerID, "different", "different", "idempotency-key")
	if err != nil {
		t.Fatal(err)
	}
	if idempotent.RequestID != request.RequestID || idempotent.RequestedBy != request.RequestedBy {
		t.Fatal("idempotent request did not return existing request")
	}
	if _, err := store.ApproveKeyUnwrapRequest(ctx, request.RequestID, "requester"); !errors.Is(err, ErrSelfApproval) {
		t.Fatalf("self approval error = %v, want ErrSelfApproval", err)
	}
	if approved, err := store.ApproveKeyUnwrapRequest(ctx, request.RequestID, "operator-one"); err != nil || approved.Status != "pending" {
		t.Fatalf("first approval = %#v, %v", approved, err)
	}
	if approved, err := store.ApproveKeyUnwrapRequest(ctx, request.RequestID, "operator-two"); err != nil || approved.Status != "approved" {
		t.Fatalf("second approval = %#v, %v", approved, err)
	}
	if _, err := store.ApproveKeyUnwrapRequest(ctx, request.RequestID, "operator-two"); !errors.Is(err, ErrKeyUnwrapNotPending) {
		t.Fatalf("duplicate approval error = %v, want pending conflict", err)
	}
	request, approvers, err := store.GetKeyUnwrapRequest(ctx, request.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if request.Status != "approved" || len(approvers) != 2 {
		t.Fatalf("request after approval = %#v approvers=%v", request, approvers)
	}
	record, err := keyescrow.ParseWrappedKey(gotWrapped)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := keyescrow.OpenDEK(privateKey, record, request.ComputerID)
	if err != nil || !bytes.Equal(opened, dek) {
		t.Fatalf("opened DEK = %x, %v", opened, err)
	}
	if err := store.MarkKeyUnwrapRevealed(ctx, request.RequestID); err != nil {
		t.Fatal(err)
	}
	revealed, _, err := store.GetKeyUnwrapRequest(ctx, request.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if revealed.Status != "revealed" || revealed.RevealedAt == nil {
		t.Fatalf("request was not revealed: %#v", revealed)
	}

	payloadOne := []byte(`{"type":"dek_reveal","request_id":"` + request.RequestID + `"}`)
	seqOne, hashOne, err := store.AppendKeyEscrowTransparency(ctx, payloadOne)
	if err != nil {
		t.Fatal(err)
	}
	payloadTwo := []byte(`{"type":"audit"}`)
	seqTwo, hashTwo, err := store.AppendKeyEscrowTransparency(ctx, payloadTwo)
	if err != nil {
		t.Fatal(err)
	}
	if seqOne != 1 || seqTwo != 2 {
		t.Fatalf("unexpected transparency sequences: %d, %d", seqOne, seqTwo)
	}
	firstDigest := sha256.Sum256(payloadOne)
	secondPreimage := append([]byte(hex.EncodeToString(firstDigest[:])), payloadTwo...)
	secondDigest := sha256.Sum256(secondPreimage)
	if hashOne != hex.EncodeToString(firstDigest[:]) || hashTwo != hex.EncodeToString(secondDigest[:]) {
		t.Fatal("transparency hashes do not form the expected chain")
	}
	seq, head, err := store.KeyEscrowTransparencyHead(ctx)
	if err != nil || seq != seqTwo || head != hashTwo {
		t.Fatalf("transparency head = %d, %q, %v", seq, head, err)
	}
}

func TestKeyEscrowHTTPFlow(t *testing.T) {
	store, root := openTestPlatformStore(t)
	service := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "signing-key"))
	handler := NewHandler(service)
	privateKey, _, err := keyescrow.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	if err := handler.ConfigureKeyEscrow(privateKey, "alice:token-a,bob:token-b,charlie:token-c"); err != nil {
		t.Fatal(err)
	}

	publicResponse := callKeyEscrowHandler(t, handler.HandleKeyEscrowPublicKey, http.MethodGet, "/internal/computers/keys/escrow-public-key", nil, map[string]string{"X-Internal-Caller": "true"})
	if publicResponse.Code != http.StatusOK {
		t.Fatalf("public key status = %d", publicResponse.Code)
	}
	var publicBody struct {
		PublicKey string `json:"public_key"`
	}
	if err := json.NewDecoder(publicResponse.Body).Decode(&publicBody); err != nil {
		t.Fatal(err)
	}
	publicBytes, err := base64.RawStdEncoding.DecodeString(publicBody.PublicKey)
	if err != nil || len(publicBytes) != 32 {
		t.Fatalf("invalid public key response: %v", err)
	}
	var publicKey keyescrow.PublicKey
	copy(publicKey[:], publicBytes)
	dek := bytes.Repeat([]byte{0x18}, 32)
	wrapped, err := keyescrow.SealDEK(publicKey, "computer-http-escrow", dek)
	if err != nil {
		t.Fatal(err)
	}
	wrappedJSON, _ := json.Marshal(wrapped)
	putBody, _ := json.Marshal(keyEscrowPutRequest{ComputerID: wrapped.ComputerID, Protector: wrapped.Protector, WrappedKey: string(wrappedJSON), KeyDigest: wrapped.KeyDigest})
	putResponse := callKeyEscrowHandler(t, handler.HandleKeyEscrow, http.MethodPut, "/internal/computers/keys/escrow", putBody, map[string]string{"X-Internal-Caller": "true"})
	if putResponse.Code != http.StatusOK {
		t.Fatalf("escrow PUT status = %d body=%s", putResponse.Code, putResponse.Body.String())
	}
	statusResponse := callKeyEscrowHandler(t, handler.HandleKeyEscrowStatus, http.MethodGet, "/internal/computers/keys/escrow/status?computer_id="+wrapped.ComputerID, nil, map[string]string{"X-Internal-Caller": "true"})
	if statusResponse.Code != http.StatusOK {
		t.Fatalf("escrow status = %d body=%s", statusResponse.Code, statusResponse.Body.String())
	}
	var statusBody struct {
		Escrows []KeyEscrowStatus `json:"escrows"`
	}
	if err := json.NewDecoder(statusResponse.Body).Decode(&statusBody); err != nil || len(statusBody.Escrows) != 1 || statusBody.Escrows[0].KeyDigest != wrapped.KeyDigest {
		t.Fatalf("invalid escrow status response: %#v, %v", statusBody, err)
	}

	requestBody, _ := json.Marshal(keyUnwrapRequestInput{ComputerID: wrapped.ComputerID, RequestedBy: "requester", Reason: "recovery", IdempotencyKey: "http-idempotency"})
	operatorHeaders := map[string]string{"X-Choir-Operator": "alice", "X-Choir-Operator-Token": "token-a"}
	requestResponse := callKeyEscrowHandler(t, handler.HandleKeyUnwrapRequests, http.MethodPost, "/internal/computers/keys/unwrap-requests", requestBody, operatorHeaders)
	if requestResponse.Code != http.StatusCreated {
		t.Fatalf("unwrap request status = %d body=%s", requestResponse.Code, requestResponse.Body.String())
	}
	var request KeyUnwrapRequest
	if err := json.NewDecoder(requestResponse.Body).Decode(&request); err != nil {
		t.Fatal(err)
	}
	// The requester is pinned to the authenticated operator (alice): she must
	// not be able to approve her own request.
	selfApprovalBody, _ := json.Marshal(keyUnwrapApprovalInput{Approver: "alice"})
	selfApprovalResponse := callKeyEscrowHandler(t, handler.HandleKeyUnwrapRequestAction, http.MethodPost, "/internal/computers/keys/unwrap-requests/"+request.RequestID+"/approvals", selfApprovalBody, operatorHeaders)
	if selfApprovalResponse.Code != http.StatusForbidden {
		t.Fatalf("self approval status = %d want 403 body=%s", selfApprovalResponse.Code, selfApprovalResponse.Body.String())
	}
	approvalBody, _ := json.Marshal(keyUnwrapApprovalInput{Approver: "bob"})
	approvalResponse := callKeyEscrowHandler(t, handler.HandleKeyUnwrapRequestAction, http.MethodPost, "/internal/computers/keys/unwrap-requests/"+request.RequestID+"/approvals", approvalBody, map[string]string{"X-Choir-Operator": "bob", "X-Choir-Operator-Token": "token-b"})
	if approvalResponse.Code != http.StatusOK {
		t.Fatalf("first approval status = %d body=%s", approvalResponse.Code, approvalResponse.Body.String())
	}
	approvalBody, _ = json.Marshal(keyUnwrapApprovalInput{Approver: "charlie"})
	approvalResponse = callKeyEscrowHandler(t, handler.HandleKeyUnwrapRequestAction, http.MethodPost, "/internal/computers/keys/unwrap-requests/"+request.RequestID+"/approvals", approvalBody, map[string]string{"X-Choir-Operator": "charlie", "X-Choir-Operator-Token": "token-c"})
	if approvalResponse.Code != http.StatusOK {
		t.Fatalf("second approval status = %d body=%s", approvalResponse.Code, approvalResponse.Body.String())
	}
	revealResponse := callKeyEscrowHandler(t, handler.HandleKeyUnwrapRequestAction, http.MethodPost, "/internal/computers/keys/unwrap-requests/"+request.RequestID+"/reveal", nil, map[string]string{"X-Choir-Operator": "charlie", "X-Choir-Operator-Token": "token-c"})
	if revealResponse.Code != http.StatusOK {
		t.Fatalf("reveal status = %d body=%s", revealResponse.Code, revealResponse.Body.String())
	}
	var revealBody struct {
		DEK       string `json:"dek"`
		KeyDigest string `json:"key_digest"`
	}
	if err := json.NewDecoder(revealResponse.Body).Decode(&revealBody); err != nil {
		t.Fatal(err)
	}
	recovered, err := base64.RawStdEncoding.DecodeString(revealBody.DEK)
	if err != nil || !bytes.Equal(recovered, dek) || revealBody.KeyDigest != wrapped.KeyDigest {
		t.Fatalf("invalid reveal response: %v", err)
	}
	headResponse := callKeyEscrowHandler(t, handler.HandleKeyEscrowTransparencyHead, http.MethodGet, "/internal/computers/keys/transparency-head", nil, map[string]string{"X-Internal-Caller": "true"})
	if headResponse.Code != http.StatusOK {
		t.Fatalf("transparency head status = %d body=%s", headResponse.Code, headResponse.Body.String())
	}
}

func TestKeyEscrowOperatorGateClosed(t *testing.T) {
	store, root := openTestPlatformStore(t)
	handler := NewHandler(NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "signing-key")))
	privateKey, _, err := keyescrow.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	if err := handler.ConfigureKeyEscrow(privateKey, "invalid"); err != nil {
		t.Fatal(err)
	}
	response := callKeyEscrowHandler(t, handler.HandleKeyUnwrapRequests, http.MethodPost, "/internal/computers/keys/unwrap-requests", []byte(`{}`), nil)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("gate status = %d", response.Code)
	}
}

func callKeyEscrowHandler(t *testing.T, handler http.HandlerFunc, method, target string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, target, bytes.NewReader(body))
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	response := httptest.NewRecorder()
	handler(response, request)
	return response
}
