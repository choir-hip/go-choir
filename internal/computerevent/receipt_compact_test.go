package computerevent

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"
)

func compactTestDigest(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])
}

// compactTestReceipt signs an EventHeadReceipt the way corpusd's CAS does
// (platform eventHeadReceiptFields) and returns its row and stored bytes.
func compactTestReceipt(t *testing.T, previous, pending string, payloadPins []string, issuedAt time.Time) (HeadReceiptRow, []byte, ed25519.PublicKey) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	row := HeadReceiptRow{
		ComputerID: "computer-59c902c5d46caecc0a180a8557164e72", Sequence: 21193,
		EventKind: string(EventProjectionBatchRecorded), EventDigest: compactTestDigest("event"),
		RequestCommitment: compactTestDigest("request"), EventPinReceiptDigest: compactTestDigest("pin"),
	}
	receipt, err := NewSignedReceipt("EventHeadReceipt", "corpusd", map[string]any{
		"computer_id":                row.ComputerID,
		"previous_head":              previous,
		"event_digest":               row.EventDigest,
		"sequence":                   row.Sequence,
		"event_kind":                 EventProjectionBatchRecorded,
		"request_commitment":         row.RequestCommitment,
		"pin_receipt_digests":        append([]string{row.EventPinReceiptDigest}, payloadPins...),
		"desired_event_head":         compactTestDigest("desired"),
		"effective_event_head":       compactTestDigest("desired"),
		"pending_transition_ref":     pending,
		"desired_state_commitment":   compactTestDigest("state"),
		"effective_state_commitment": compactTestDigest("state"),
	}, []SigningKey{{SignerRef: SignerRef{SignerDomain: "platform-control", KeyID: "0123456789abcdef"}, PrivateKey: private}}, issuedAt)
	if err != nil {
		t.Fatal(err)
	}
	row.ReceiptID = receipt.ReceiptID
	raw, err := receipt.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	return row, raw, public
}

// Failure 1: the rebuilt bytes must be the signed original, and the
// signature must still verify on them.
func TestCompactHeadReceiptRoundTripsExactly(t *testing.T) {
	cases := []struct {
		name     string
		previous string
		pending  string
		pins     []string
		at       time.Time
	}{
		{"projection batch, three payload pins", compactTestDigest("prev"), "", []string{compactTestDigest("p1"), compactTestDigest("p2"), compactTestDigest("p3")}, time.Date(2026, 10, 31, 12, 0, 0, 123456789, time.UTC)},
		{"genesis, zero previous, no payload pins, whole second", strings.Repeat("0", 64), "", nil, time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)},
		{"empty previous head", "", "", nil, time.Date(2026, 10, 13, 0, 0, 0, 1000, time.UTC)},
		{"pending transition ref, non-hex", compactTestDigest("prev"), "transition:abc", []string{"not-a-digest"}, time.Now().UTC()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row, raw, public := compactTestReceipt(t, tc.previous, tc.pending, tc.pins, tc.at)
			compact, ok := CompactHeadReceipt(row, raw)
			if !ok {
				t.Fatal("compaction refused a receipt the CAS writes")
			}
			if !IsCompactHeadReceipt(compact) || len(compact) >= len(raw) {
				t.Fatalf("compact %d bytes from %d", len(compact), len(raw))
			}
			t.Logf("full %d bytes, compact %d bytes", len(raw), len(compact))
			expanded, err := ExpandHeadReceipt(row, compact)
			if err != nil {
				t.Fatal(err)
			}
			if string(expanded) != string(raw) {
				t.Fatalf("expanded differs:\n%s\n%s", expanded, raw)
			}
			var receipt Receipt
			if err := receipt.UnmarshalJSON(expanded); err != nil {
				t.Fatal(err)
			}
			if err := receipt.Verify(staticKeyResolver{key: public}); err != nil {
				t.Fatalf("expanded receipt does not verify: %v", err)
			}
		})
	}
}

// Failure 4: a full value passes through unchanged.
func TestExpandHeadReceiptPassesFullJSON(t *testing.T) {
	row, raw, _ := compactTestReceipt(t, compactTestDigest("prev"), "", nil, time.Now().UTC())
	got, err := ExpandHeadReceipt(row, string(raw))
	if err != nil || string(got) != string(raw) {
		t.Fatalf("full passthrough = %q, %v", got, err)
	}
}

// Failure 1: anything the encoder cannot rebuild stays full.
func TestCompactHeadReceiptRefusesWhatItCannotRebuild(t *testing.T) {
	row, raw, _ := compactTestReceipt(t, compactTestDigest("prev"), "", nil, time.Now().UTC())
	mutate := func(f func(*HeadReceiptRow)) HeadReceiptRow { r := row; f(&r); return r }
	for name, r := range map[string]HeadReceiptRow{
		"other receipt id":   mutate(func(r *HeadReceiptRow) { r.ReceiptID = "x" }),
		"other sequence":     mutate(func(r *HeadReceiptRow) { r.Sequence++ }),
		"other event digest": mutate(func(r *HeadReceiptRow) { r.EventDigest = compactTestDigest("other") }),
		"other kind":         mutate(func(r *HeadReceiptRow) { r.EventKind = "file_root_committed" }),
		"other pin":          mutate(func(r *HeadReceiptRow) { r.EventPinReceiptDigest = compactTestDigest("other") }),
		"other computer":     mutate(func(r *HeadReceiptRow) { r.ComputerID = "computer-x" }),
		"other commitment":   mutate(func(r *HeadReceiptRow) { r.RequestCommitment = compactTestDigest("other") }),
	} {
		if _, ok := CompactHeadReceipt(r, raw); ok {
			t.Errorf("%s: compacted a receipt whose row disagrees", name)
		}
	}
	// An unknown kind field (a receipt shape the encoder does not know).
	extra := strings.Replace(string(raw), `"computer_id":`, `"zz_extra":"1","computer_id":`, 1)
	if _, ok := CompactHeadReceipt(row, []byte(extra)); ok {
		t.Error("compacted a receipt with an extra kind field")
	}
	if _, ok := CompactHeadReceipt(row, []byte("not json")); ok {
		t.Error("compacted non-JSON")
	}
}

// Failure 2: a damaged compact value, or one read against another row, is an
// error, never other bytes.
func TestExpandHeadReceiptRefusesDamage(t *testing.T) {
	row, raw, _ := compactTestReceipt(t, compactTestDigest("prev"), "", []string{compactTestDigest("p1")}, time.Now().UTC())
	compact, ok := CompactHeadReceipt(row, raw)
	if !ok {
		t.Fatal("refused")
	}
	other := row
	other.Sequence++
	if _, err := ExpandHeadReceipt(other, compact); !errors.Is(err, ErrCompactHeadReceipt) {
		t.Fatalf("wrong row = %v", err)
	}
	body := []byte(compact)
	for i := len(CompactHeadReceiptPrefix); i < len(body); i += 7 {
		damaged := append([]byte(nil), body...)
		if damaged[i] == 'A' {
			damaged[i] = 'B'
		} else {
			damaged[i] = 'A'
		}
		if got, err := ExpandHeadReceipt(row, string(damaged)); err == nil && string(got) != string(raw) {
			t.Fatalf("damage at %d expanded to other bytes", i)
		}
	}
	for _, bad := range []string{CompactHeadReceiptPrefix, CompactHeadReceiptPrefix + "!!!", compact[:len(compact)-10]} {
		if _, err := ExpandHeadReceipt(row, bad); err == nil {
			t.Fatalf("expanded %q", bad)
		}
	}
}
