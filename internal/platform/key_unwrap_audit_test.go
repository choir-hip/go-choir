package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/keyescrow"
)

// An operator reveal is audited or it does not happen (the frame's hard gate
// "every unwrap is audited"; review of the cutover, 2026-10-30). Before, the
// handler opened the key, flipped the request to revealed, then appended the
// transparency entry in a second transaction, so a failed append left a
// revealed request with no entry and the approval spent. Ways this could fail:
//  1. the transparency append fails and the request still reads revealed;
//  2. the transparency append fails and the key is returned;
//  3. a reveal succeeds with no transparency entry naming the request;
//  4. a second reveal of the same request succeeds or adds a second entry.
//
// The append is broken here the way the cutover broke it: a row sits where
// the identity sequence will put the next one.
func TestKeyUnwrapRevealIsAuditedOrNotRevealed(t *testing.T) {
	ctx := context.Background()
	store, root := openTestPlatformStore(t)
	handler := NewHandler(NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "signing-key")))
	privateKey, publicKey, err := keyescrow.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	if err := handler.ConfigureKeyEscrow(privateKey, "alice:token-a,bob:token-b,charlie:token-c"); err != nil {
		t.Fatal(err)
	}
	dek := bytes.Repeat([]byte{0x2a}, 32)
	wrapped, err := keyescrow.SealDEK(publicKey, "computer-audit", dek)
	if err != nil {
		t.Fatal(err)
	}
	wrappedJSON, _ := json.Marshal(wrapped)
	putBody, _ := json.Marshal(keyEscrowPutRequest{ComputerID: wrapped.ComputerID, Protector: wrapped.Protector, WrappedKey: string(wrappedJSON), KeyDigest: wrapped.KeyDigest})
	if rec := callKeyEscrowHandler(t, handler.HandleKeyEscrow, http.MethodPut, "/internal/computers/keys/escrow", putBody, map[string]string{"X-Internal-Caller": "true"}); rec.Code != http.StatusOK {
		t.Fatalf("escrow put = %d %s", rec.Code, rec.Body)
	}
	requestBody, _ := json.Marshal(keyUnwrapRequestInput{ComputerID: wrapped.ComputerID, RequestedBy: "requester", Reason: "recovery", IdempotencyKey: "audit-test"})
	rec := callKeyEscrowHandler(t, handler.HandleKeyUnwrapRequests, http.MethodPost, "/internal/computers/keys/unwrap-requests", requestBody, map[string]string{"X-Choir-Operator": "alice", "X-Choir-Operator-Token": "token-a"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("unwrap request = %d %s", rec.Code, rec.Body)
	}
	var request KeyUnwrapRequest
	if err := json.NewDecoder(rec.Body).Decode(&request); err != nil {
		t.Fatal(err)
	}
	action := "/internal/computers/keys/unwrap-requests/" + request.RequestID
	for _, op := range []struct{ name, token string }{{"bob", "token-b"}, {"charlie", "token-c"}} {
		body, _ := json.Marshal(keyUnwrapApprovalInput{Approver: op.name})
		if rec := callKeyEscrowHandler(t, handler.HandleKeyUnwrapRequestAction, http.MethodPost, action+"/approvals", body, map[string]string{"X-Choir-Operator": op.name, "X-Choir-Operator-Token": op.token}); rec.Code != http.StatusOK {
			t.Fatalf("approval by %s = %d %s", op.name, rec.Code, rec.Body)
		}
	}
	reveal := func() int {
		return callKeyEscrowHandler(t, handler.HandleKeyUnwrapRequestAction, http.MethodPost, action+"/reveal", nil, map[string]string{"X-Choir-Operator": "charlie", "X-Choir-Operator-Token": "token-c"}).Code
	}
	entries := func() int {
		var n int
		if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM computer_key_escrow_transparency WHERE payload_json LIKE '%"type":"dek_reveal"%' AND payload_json LIKE '%`+request.RequestID+`%'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	status := func() string {
		got, _, err := store.GetKeyUnwrapRequest(ctx, request.RequestID)
		if err != nil {
			t.Fatal(err)
		}
		return got.Status
	}

	// Block the next identity value.
	if _, err := store.db.ExecContext(ctx, `INSERT INTO computer_key_escrow_transparency (seq, prev_hash, entry_hash, payload_json, created_at)
		OVERRIDING SYSTEM VALUE
		SELECT COALESCE(MAX(seq), 0) + 1, '', 'blocker', '{}', now() FROM computer_key_escrow_transparency`); err != nil {
		t.Fatal(err)
	}
	if code := reveal(); code == http.StatusOK {
		t.Fatal("a reveal returned the key while its audit append failed")
	}
	if got := status(); got != "approved" {
		t.Fatalf("after a failed audit append the request is %q, want approved", got)
	}
	if n := entries(); n != 0 {
		t.Fatalf("%d transparency entries after a failed append", n)
	}

	// Unblock: the same approved request reveals once, with one entry.
	if _, err := store.db.ExecContext(ctx, `DELETE FROM computer_key_escrow_transparency WHERE entry_hash = 'blocker'`); err != nil {
		t.Fatal(err)
	}
	if code := reveal(); code != http.StatusOK {
		t.Fatalf("reveal after the append recovered = %d", code)
	}
	if got, n := status(), entries(); got != "revealed" || n != 1 {
		t.Fatalf("after reveal: status %q, %d entries; want revealed and 1", got, n)
	}
	if code := reveal(); code == http.StatusOK {
		t.Fatal("a second reveal of one request succeeded")
	}
	if n := entries(); n != 1 {
		t.Fatalf("%d entries after a refused second reveal", n)
	}
}
