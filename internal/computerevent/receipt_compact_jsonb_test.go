package computerevent

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// jsonbSpelling writes a receipt the way the old rows are stored: the shape
// read from a row of the paper's sequence 3,000,000 (keys by length then
// bytes, ", " and ": " separators). It is written out by hand, not through
// the encoder under test.
func jsonbSpelling(row HeadReceiptRow, canonical []byte) string {
	var r Receipt
	if err := r.UnmarshalJSON(canonical); err != nil {
		panic(err)
	}
	f := func(name string) string { return fmt.Sprint(r.KindFields[name]) }
	pins := r.KindFields["pin_receipt_digests"].([]any)
	quoted := make([]string, len(pins))
	for i, p := range pins {
		quoted[i] = fmt.Sprintf("%q", p)
	}
	sig, signer := r.SignatureSet[0], r.RequiredSigners[0]
	return fmt.Sprintf(`{"issuer": %q, "sequence": %d, "issued_at": %q, "event_kind": %q, "receipt_id": %q, "computer_id": %q, "event_digest": %q, "receipt_kind": %q, "previous_head": %q, `+
		`"signature_set": [{"key_id": %q, "signature": %q, "signer_domain": %q}], "receipt_version": %d, "required_signers": [{"key_id": %q, "signer_domain": %q}], `+
		`"desired_event_head": %q, "request_commitment": %q, "pin_receipt_digests": [%s], "effective_event_head": %q, "pending_transition_ref": %q, `+
		`"canonical_payload_sha256": %q, "desired_state_commitment": %q, "effective_state_commitment": %q}`,
		r.Issuer, row.Sequence, r.IssuedAt, row.EventKind, r.ReceiptID, row.ComputerID, row.EventDigest, r.ReceiptKind, f("previous_head"),
		sig.KeyID, sig.Signature, sig.SignerDomain, r.ReceiptVersion, signer.KeyID, signer.SignerDomain,
		f("desired_event_head"), row.RequestCommitment, strings.Join(quoted, ", "), f("effective_event_head"), f("pending_transition_ref"),
		r.CanonicalPayloadSHA256, f("desired_state_commitment"), f("effective_state_commitment"))
}

// Format 2 is read by every reader and written only with the allowance
// (CompactStoredHeadReceipt, allowJSONB). The rows written before about 2026-10-30 hold the jsonb spelling, not the
// canonical bytes, so the canonical codec's round trip failed for them and
// they stayed full. The compact form must give back those exact bytes.
func TestCompactHeadReceiptKeepsJSONBSpelledRowsExactly(t *testing.T) {
	for _, tc := range []struct {
		name string
		pins []string
		at   time.Time
	}{
		{"one pin, nanoseconds", nil, time.Date(2026, 10, 23, 16, 43, 50, 123456789, time.UTC)},
		{"three pins, whole second", []string{compactTestDigest("p1"), compactTestDigest("p2")}, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row, canonical, public := compactTestReceipt(t, compactTestDigest("prev"), "", tc.pins, tc.at)
			old := jsonbSpelling(row, canonical)
			if old == string(canonical) {
				t.Fatal("fixture is already canonical")
			}
			// Not written without the allowance: the canonical writer and the
			// explicit refusal both leave the row full.
			if _, ok := CompactHeadReceipt(row, []byte(old)); ok {
				t.Fatal("CompactHeadReceipt wrote format 2")
			}
			if _, ok := CompactStoredHeadReceipt(row, []byte(old), false); ok {
				t.Fatal("format 2 written without the allowance")
			}
			compact, ok := CompactStoredHeadReceipt(row, []byte(old), true)
			if !ok {
				t.Fatal("a jsonb-spelled receipt stayed full")
			}
			if len(compact) >= len(old)/2 {
				t.Fatalf("compact %d bytes from %d", len(compact), len(old))
			}
			got, err := ExpandHeadReceipt(row, compact)
			if err != nil || string(got) != old {
				t.Fatalf("expand = %q, %v; want the stored bytes", got, err)
			}
			// Same receipt, so it still verifies once parsed.
			var receipt Receipt
			if err := receipt.UnmarshalJSON(got); err != nil {
				t.Fatal(err)
			}
			if err := receipt.Verify(staticKeyResolver{key: public}); err != nil {
				t.Fatalf("expanded receipt does not verify: %v", err)
			}
			// Another spelling of the same receipt (extra space) is refused,
			// not respelled.
			if c, ok := CompactStoredHeadReceipt(row, []byte(strings.Replace(old, `"issuer": `, `"issuer":  `, 1)), true); ok {
				if g, err := ExpandHeadReceipt(row, c); err != nil || string(g) == old {
					t.Fatalf("another spelling expanded to %q, %v", g, err)
				}
			}
		})
	}
}

// The canonical form still wins for canonical text.
func TestCompactHeadReceiptStillPrefersCanonicalFormat(t *testing.T) {
	row, raw, _ := compactTestReceipt(t, compactTestDigest("prev"), "", nil, time.Now().UTC())
	compact, ok := CompactHeadReceipt(row, raw)
	if !ok || !IsCompactHeadReceipt(compact) {
		t.Fatal("canonical receipt refused")
	}
	if got, err := ExpandHeadReceipt(row, compact); err != nil || string(got) != string(raw) {
		t.Fatalf("canonical expand = %q, %v", got, err)
	}
}
