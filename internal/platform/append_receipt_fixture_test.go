package platform

import (
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// insertTestAppendReceipt inserts a fixture computer_event_append_receipts
// row with a signed event head receipt that names the event pin digest first
// and then payloadPins, as the CAS writes it.
func insertTestAppendReceipt(t testing.TB, db *sql.DB, computerID, idempotencyKey, requestCommitment string, sequence any, kind, eventDigest, eventPin string, payloadPins []string, createdAt time.Time) {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pins := append([]string{eventPin}, payloadPins...)
	zero := strings.Repeat("0", 64)
	receipt, err := computerevent.NewSignedReceipt("EventHeadReceipt", "corpusd", map[string]any{
		"computer_id":                computerID,
		"previous_head":              zero,
		"event_digest":               eventDigest,
		"sequence":                   sequence,
		"event_kind":                 kind,
		"request_commitment":         requestCommitment,
		"pin_receipt_digests":        pins,
		"desired_event_head":         eventDigest,
		"effective_event_head":       zero,
		"pending_transition_ref":     "",
		"desired_state_commitment":   zero,
		"effective_state_commitment": zero,
	}, []computerevent.SigningKey{{SignerRef: computerevent.SignerRef{SignerDomain: "platform", KeyID: "fixture"}, PrivateKey: key}}, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := receipt.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO computer_event_append_receipts (computer_id,idempotency_key,request_commitment,sequence,event_kind,event_digest,event_pin_receipt_digest,event_head_receipt_id,event_head_receipt_json,created_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		computerID, idempotencyKey, requestCommitment, sequence, kind, eventDigest, eventPin, receipt.ReceiptID, string(raw), createdAt); err != nil {
		t.Fatal(err)
	}
}
