package platform

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// The three lookups of checkpoints.go that name the current head's receipt
// find its row by (computer, sequence) and check the receipt id against it
// (receipt id index: none). Every refusal reads as it did with a lookup by id.

func moveHeadForward(t *testing.T, authority *CheckpointAuthority, computerID string) (newHead string) {
	t.Helper()
	newHead = platformTestDigest('7')
	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := authority.cas.store.db.ExecContext(t.Context(), `INSERT INTO computer_event_append_receipts (computer_id, idempotency_key, request_commitment, sequence, event_kind, event_digest, event_pin_receipt_digest, event_head_receipt_id, event_head_receipt_json, created_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		computerID, "head-13", platformTestDigest('c'), 13, "key_revoked", newHead, platformTestDigest('d'), "receipt-head-13",
		`{"receipt_version":1,"receipt_kind":"EventHead","receipt_id":"receipt-head-13","issuer":"corpusd","issued_at":"2026-08-14T00:00:01Z","required_signers":[],"canonical_payload_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","signature_set":[]}`, now); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.cas.store.db.ExecContext(t.Context(), `UPDATE computer_event_heads SET sequence=13, canonical_event_head=? WHERE computer_id=?`, newHead, computerID); err != nil {
		t.Fatal(err)
	}
	return newHead
}

func TestHeadReceiptLookupsRefuseAsTheyDidByID(t *testing.T) {
	t.Run("unknown receipt id", func(t *testing.T) {
		authority, request, _, _ := ownerRecoveryPlatformFixture(t)
		request.EventHeadReceiptID = "receipt-nobody"
		_, err := authority.Publish(t.Context(), request)
		if err == nil || !strings.Contains(err.Error(), "accepted event receipt is unavailable") || !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("err = %v, want unavailable wrapping sql.ErrNoRows", err)
		}
	})
	t.Run("receipt of an earlier event", func(t *testing.T) {
		authority, request, _, _ := ownerRecoveryPlatformFixture(t)
		now := time.Now().UTC().Truncate(time.Microsecond)
		if _, err := authority.cas.store.db.ExecContext(t.Context(), `INSERT INTO computer_event_append_receipts (computer_id, idempotency_key, request_commitment, sequence, event_kind, event_digest, event_pin_receipt_digest, event_head_receipt_id, event_head_receipt_json, created_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
			request.ComputerID, "head-11", platformTestDigest('c'), 11, "key_revoked", platformTestDigest('8'), platformTestDigest('d'), "receipt-head-11",
			`{"receipt_version":1,"receipt_kind":"EventHead","receipt_id":"receipt-head-11","issuer":"corpusd","issued_at":"2026-08-14T00:00:00Z","required_signers":[],"canonical_payload_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","signature_set":[]}`, now); err != nil {
			t.Fatal(err)
		}
		request.EventHeadReceiptID = "receipt-head-11"
		_, err := authority.Publish(t.Context(), request)
		if err == nil || !strings.Contains(err.Error(), "accepted event receipt is unavailable") || !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("err = %v, want unavailable wrapping sql.ErrNoRows", err)
		}
	})
	t.Run("the head moved between the request and the check", func(t *testing.T) {
		authority, request, _, _ := ownerRecoveryPlatformFixture(t)
		newHead := moveHeadForward(t, authority, request.ComputerID)
		request.AcceptedEventHead = newHead // the requester re-read the head, but names the old receipt
		_, err := authority.Publish(t.Context(), request)
		if err == nil || !strings.Contains(err.Error(), "accepted event receipt is unavailable") {
			t.Fatalf("err = %v, want the old head's receipt refused as unavailable", err)
		}
		// The new head's receipt is found at the new sequence.
		request.EventHeadReceiptID = "receipt-head-13"
		request.IdempotencyKey = "owner-recovery-" + newHead
		if _, err := authority.Publish(t.Context(), request); err != nil {
			t.Fatalf("publish naming the new head's receipt: %v", err)
		}
	})
	t.Run("the receipt body names another id", func(t *testing.T) {
		authority, request, _, _ := ownerRecoveryPlatformFixture(t)
		if _, err := authority.cas.store.db.ExecContext(t.Context(), `UPDATE computer_event_append_receipts SET event_head_receipt_json=replace(event_head_receipt_json, '"receipt_id":"receipt-head-12"', '"receipt_id":"receipt-other"') WHERE computer_id=? AND sequence=12`, request.ComputerID); err != nil {
			t.Fatal(err)
		}
		_, err := authority.Publish(t.Context(), request)
		if err == nil || !strings.Contains(err.Error(), "accepted event receipt binding failed") {
			t.Fatalf("err = %v, want binding failed", err)
		}
	})
	t.Run("the row at the head does not bind the accepted head", func(t *testing.T) {
		authority, request, _, _ := ownerRecoveryPlatformFixture(t)
		if _, err := authority.cas.store.db.ExecContext(t.Context(), `UPDATE computer_event_append_receipts SET event_digest=? WHERE computer_id=? AND sequence=12`, platformTestDigest('9'), request.ComputerID); err != nil {
			t.Fatal(err)
		}
		_, err := authority.Publish(t.Context(), request)
		if err == nil || !strings.Contains(err.Error(), "non-verifier head receipt does not bind the accepted head") {
			t.Fatalf("err = %v, want does not bind", err)
		}
	})
}

func TestRouteProjectionHeadReceiptLookupRefusesAsItDidByID(t *testing.T) {
	authority, request, _, canonicalHead := ownerRecoveryPlatformFixture(t)
	published, err := authority.Publish(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	projection := func(receiptID string) selfdevprotocol.RouteProjectionRequest {
		return selfdevprotocol.RouteProjectionRequest{
			ComputerID: request.ComputerID, IdempotencyKey: "route-" + receiptID, Checkpoint: published,
			CanonicalEventHead: canonicalHead, EventHeadReceiptID: receiptID,
			ExpiresAt: time.Now().UTC().Add(time.Minute).Format(time.RFC3339Nano),
		}
	}
	const unavailable = "route projection authority: current event head receipt is unavailable"
	// The right receipt passes this lookup; the request then fails later on the
	// evidence this fixture does not carry.
	if _, err := authority.PublishRouteProjection(t.Context(), projection("receipt-head-12")); err == nil || err.Error() == unavailable {
		t.Fatalf("the head's own receipt: err = %v, want a later refusal", err)
	}
	if _, err := authority.PublishRouteProjection(t.Context(), projection("receipt-nobody")); err == nil || err.Error() != unavailable {
		t.Fatalf("unknown receipt: err = %v", err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := authority.cas.store.db.ExecContext(t.Context(), `INSERT INTO computer_event_append_receipts (computer_id, idempotency_key, request_commitment, sequence, event_kind, event_digest, event_pin_receipt_digest, event_head_receipt_id, event_head_receipt_json, created_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		request.ComputerID, "head-11", platformTestDigest('c'), 11, "key_revoked", platformTestDigest('8'), platformTestDigest('d'), "receipt-head-11", "{}", now); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.PublishRouteProjection(t.Context(), projection("receipt-head-11")); err == nil || err.Error() != unavailable {
		t.Fatalf("receipt of another sequence: err = %v", err)
	}
	if _, err := authority.cas.store.db.ExecContext(t.Context(), `UPDATE computer_event_append_receipts SET event_digest=? WHERE computer_id=? AND sequence=12`, platformTestDigest('9'), request.ComputerID); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.PublishRouteProjection(t.Context(), projection("receipt-head-12")); err == nil || err.Error() != unavailable {
		t.Fatalf("digest that is not the canonical head: err = %v", err)
	}
	// The head moves on: the old head's receipt is not the current head's.
	if _, err := authority.cas.store.db.ExecContext(t.Context(), `UPDATE computer_event_append_receipts SET event_digest=? WHERE computer_id=? AND sequence=12`, canonicalHead, request.ComputerID); err != nil {
		t.Fatal(err)
	}
	newHead := moveHeadForward(t, authority, request.ComputerID)
	moved := projection("receipt-head-12")
	moved.CanonicalEventHead = newHead
	if _, err := authority.PublishRouteProjection(t.Context(), moved); err == nil || err.Error() != unavailable {
		t.Fatalf("moved head: err = %v", err)
	}
}
