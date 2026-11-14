package computerevent

// The compact form of a stored EventHeadReceipt
// (docs/design/append-receipt-trim-2026-10-31.md). corpusd stores each
// append's signed receipt as canonical JSON in
// the append-receipt table's receipt column, about 1.5 KB a row,
// and most of it repeats the row's own columns (computer, sequence, kind,
// event digest, request commitment, the event's pin digest, the receipt id)
// or constants (version, kind, issuer), spelled as hex text under JSON keys.
// The compact form keeps only what no column holds, in binary: the previous
// head, the next heads and commitments, the payload pin digests, issued_at
// and the signatures. Expand rebuilds the exact canonical bytes from it and
// the row; nothing is lost and the signature still verifies.
//
// Ways it could fail, written before the code:
//
//  1. the rebuilt bytes differ from the signed original (a field spelled
//     differently, a number typed differently, a key the encoder does not
//     know): Compact refuses to return a compact form unless Expand of it
//     gives back the original bytes exactly, so such a row stays full JSON;
//  2. a compact value is damaged or read against the wrong row: it carries a
//     check of the original bytes (the first 16 bytes of their SHA-256), and
//     Expand fails rather than return anything else;
//  3. a reader reads the column without expanding: the compact text does not
//     start with "{", so a JSON decode fails loudly instead of misreading
//     (the platform also lints for raw readers of the column);
//  4. a full-JSON row is "expanded": Expand returns it unchanged.

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
)

// CompactHeadReceiptPrefix starts every compact stored receipt; a full one
// starts with "{".
const CompactHeadReceiptPrefix = "c1:"

const (
	compactFormatV1 byte = 1
	// compactFormatJSONB is the same fields as V1; it expands to the text
	// PostgreSQL's jsonb output wrote for rows stored before the column was
	// TEXT (keys by length then bytes, ", " and ": " separators), not to the
	// canonical form. docs/problems/old-receipts-do-not-compact-2026-11-14.md.
	compactFormatJSONB byte = 2
	compactCheckLen         = 16
	hexTagEmpty        byte = 0
	hexTagSHA256       byte = 1
	hexTagRaw          byte = 2
	eventHeadKind           = "EventHeadReceipt"
	eventHeadIssuer         = "corpusd"
)

// HeadReceiptRow is the part of an append-receipt row the compact form
// leans on: every value here is a column of the row.
type HeadReceiptRow struct {
	ComputerID            string
	Sequence              uint64
	EventKind             string
	EventDigest           string
	RequestCommitment     string
	EventPinReceiptDigest string
	ReceiptID             string
}

// ErrCompactHeadReceipt is a compact stored receipt that does not expand.
var ErrCompactHeadReceipt = errors.New("compact event head receipt")

// IsCompactHeadReceipt reports whether a stored value is in the compact form.
func IsCompactHeadReceipt(stored string) bool {
	return len(stored) >= len(CompactHeadReceiptPrefix) && stored[:len(CompactHeadReceiptPrefix)] == CompactHeadReceiptPrefix
}

// CompactHeadReceipt returns the compact form of a stored full receipt, and
// false when the receipt cannot be stored compactly without loss (the caller
// keeps the full JSON).
func CompactHeadReceipt(row HeadReceiptRow, receiptJSON []byte) (string, bool) {
	if len(receiptJSON) == 0 || receiptJSON[0] != '{' {
		return "", false
	}
	var receipt Receipt
	if err := receipt.UnmarshalJSON(receiptJSON); err != nil {
		return "", false
	}
	if receipt.ReceiptVersion < 0 || receipt.ReceiptKind != eventHeadKind || receipt.Issuer != eventHeadIssuer || receipt.ReceiptID != row.ReceiptID {
		return "", false
	}
	if len(receipt.RequiredSigners) == 0 || len(receipt.RequiredSigners) != len(receipt.SignatureSet) {
		return "", false
	}
	// The canonical form first; a receipt that is not canonical text (the
	// jsonb-spelled rows) takes the second format. Each is returned only when
	// its expansion is the original bytes exactly.
	for _, format := range []byte{compactFormatV1, compactFormatJSONB} {
		blob, ok := compactBlob(receipt, receiptJSON, format)
		if !ok {
			return "", false
		}
		compact := CompactHeadReceiptPrefix + base64.RawStdEncoding.EncodeToString(blob)
		if len(compact) >= len(receiptJSON) {
			return "", false
		}
		expanded, err := ExpandHeadReceipt(row, compact)
		if err == nil && bytes.Equal(expanded, receiptJSON) {
			return compact, true
		}
	}
	return "", false
}

func compactBlob(receipt Receipt, receiptJSON []byte, format byte) ([]byte, bool) {
	str := func(name string) (string, bool) {
		v, ok := receipt.KindFields[name].(string)
		return v, ok
	}
	var b bytes.Buffer
	b.WriteByte(format)
	b.Write(binary.AppendUvarint(nil, uint64(receipt.ReceiptVersion)))
	for _, name := range []string{"previous_head", "desired_event_head", "effective_event_head", "pending_transition_ref", "desired_state_commitment", "effective_state_commitment"} {
		v, ok := str(name)
		if !ok {
			return nil, false
		}
		writeHexField(&b, v)
	}
	pins, ok := receiptPinList(receipt.KindFields["pin_receipt_digests"])
	if !ok || len(pins) == 0 {
		return nil, false
	}
	b.Write(binary.AppendUvarint(nil, uint64(len(pins)-1)))
	for _, pin := range pins[1:] {
		writeHexField(&b, pin)
	}
	writeString(&b, receipt.IssuedAt)
	b.Write(binary.AppendUvarint(nil, uint64(len(receipt.SignatureSet))))
	for i, sig := range receipt.SignatureSet {
		if receipt.RequiredSigners[i].SignerDomain != sig.SignerDomain || receipt.RequiredSigners[i].KeyID != sig.KeyID {
			return nil, false
		}
		raw, err := base64.RawStdEncoding.Strict().DecodeString(sig.Signature)
		if err != nil {
			return nil, false
		}
		writeString(&b, sig.SignerDomain)
		writeString(&b, sig.KeyID)
		writeString(&b, string(raw))
	}
	check := sha256.Sum256(receiptJSON)
	b.Write(check[:compactCheckLen])
	return b.Bytes(), true
}

// ExpandHeadReceipt returns the canonical receipt JSON a stored value holds:
// a full value unchanged, a compact one rebuilt from it and the row.
func ExpandHeadReceipt(row HeadReceiptRow, stored string) ([]byte, error) {
	if !IsCompactHeadReceipt(stored) {
		return []byte(stored), nil
	}
	blob, err := base64.RawStdEncoding.Strict().DecodeString(stored[len(CompactHeadReceiptPrefix):])
	if err != nil {
		return nil, fmt.Errorf("%w: decode: %v", ErrCompactHeadReceipt, err)
	}
	if len(blob) < 1+compactCheckLen {
		return nil, fmt.Errorf("%w: truncated", ErrCompactHeadReceipt)
	}
	check := blob[len(blob)-compactCheckLen:]
	r := &compactReader{buf: blob[:len(blob)-compactCheckLen]}
	format := r.byte()
	if r.err == nil && format != compactFormatV1 && format != compactFormatJSONB {
		return nil, fmt.Errorf("%w: unknown format %d", ErrCompactHeadReceipt, format)
	}
	version := r.uvarint()
	heads := make([]string, 6)
	for i := range heads {
		heads[i] = r.hexField()
	}
	pinCount := r.uvarint()
	if r.err == nil && pinCount > uint64(len(r.buf)) {
		r.err = errors.New("pin count exceeds the value")
	}
	pins := []string{row.EventPinReceiptDigest}
	for i := uint64(0); i < pinCount && r.err == nil; i++ {
		pins = append(pins, r.hexField())
	}
	issuedAt := r.string()
	signerCount := r.uvarint()
	if r.err == nil && (signerCount == 0 || signerCount > uint64(len(r.buf))) {
		r.err = errors.New("bad signer count")
	}
	var signers []SignerRef
	var signatures []Signature
	for i := uint64(0); i < signerCount && r.err == nil; i++ {
		domain, keyID, sig := r.string(), r.string(), r.string()
		signers = append(signers, SignerRef{SignerDomain: domain, KeyID: keyID})
		signatures = append(signatures, Signature{SignerDomain: domain, KeyID: keyID, Signature: base64.RawStdEncoding.EncodeToString([]byte(sig))})
	}
	if r.err == nil && len(r.buf) != 0 {
		r.err = errors.New("trailing bytes")
	}
	if r.err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCompactHeadReceipt, r.err)
	}
	if version > 1<<31 {
		return nil, fmt.Errorf("%w: receipt version out of range", ErrCompactHeadReceipt)
	}
	receipt := Receipt{
		ReceiptVersion:  int(version),
		ReceiptKind:     eventHeadKind,
		ReceiptID:       row.ReceiptID,
		Issuer:          eventHeadIssuer,
		IssuedAt:        issuedAt,
		RequiredSigners: signers,
		SignatureSet:    signatures,
		KindFields: map[string]any{
			"computer_id":                row.ComputerID,
			"previous_head":              heads[0],
			"event_digest":               row.EventDigest,
			"sequence":                   row.Sequence,
			"event_kind":                 row.EventKind,
			"request_commitment":         row.RequestCommitment,
			"pin_receipt_digests":        pins,
			"desired_event_head":         heads[1],
			"effective_event_head":       heads[2],
			"pending_transition_ref":     heads[3],
			"desired_state_commitment":   heads[4],
			"effective_state_commitment": heads[5],
		},
	}
	payload, err := receipt.canonicalPayload()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCompactHeadReceipt, err)
	}
	digest := sha256.Sum256(payload)
	receipt.CanonicalPayloadSHA256 = hex.EncodeToString(digest[:])
	var out []byte
	if format == compactFormatJSONB {
		out, err = receipt.jsonbText()
	} else {
		out, err = receipt.CanonicalBytes()
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCompactHeadReceipt, err)
	}
	got := sha256.Sum256(out)
	if !bytes.Equal(got[:compactCheckLen], check) {
		return nil, fmt.Errorf("%w: check mismatch for %s sequence %d", ErrCompactHeadReceipt, row.ComputerID, row.Sequence)
	}
	return out, nil
}

func receiptPinList(value any) ([]string, bool) {
	switch raw := value.(type) {
	case []string:
		return raw, true
	case []any:
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			v, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, v)
		}
		return out, true
	}
	return nil, false
}

func writeHexField(b *bytes.Buffer, v string) {
	switch {
	case v == "":
		b.WriteByte(hexTagEmpty)
	case isSHA256(v):
		raw, _ := hex.DecodeString(v)
		b.WriteByte(hexTagSHA256)
		b.Write(raw)
	default:
		b.WriteByte(hexTagRaw)
		writeString(b, v)
	}
}

func writeString(b *bytes.Buffer, v string) {
	b.Write(binary.AppendUvarint(nil, uint64(len(v))))
	b.WriteString(v)
}

type compactReader struct {
	buf []byte
	err error
}

func (r *compactReader) byte() byte {
	if r.err != nil {
		return 0
	}
	if len(r.buf) == 0 {
		r.err = errors.New("truncated")
		return 0
	}
	v := r.buf[0]
	r.buf = r.buf[1:]
	return v
}

func (r *compactReader) uvarint() uint64 {
	if r.err != nil {
		return 0
	}
	v, n := binary.Uvarint(r.buf)
	if n <= 0 {
		r.err = errors.New("bad varint")
		return 0
	}
	r.buf = r.buf[n:]
	return v
}

func (r *compactReader) take(n uint64) []byte {
	if r.err != nil {
		return nil
	}
	if n > uint64(len(r.buf)) {
		r.err = errors.New("truncated")
		return nil
	}
	v := r.buf[:n]
	r.buf = r.buf[n:]
	return v
}

func (r *compactReader) string() string {
	return string(r.take(r.uvarint()))
}

func (r *compactReader) hexField() string {
	switch tag := r.byte(); tag {
	case hexTagEmpty:
		return ""
	case hexTagSHA256:
		return hex.EncodeToString(r.take(sha256.Size))
	case hexTagRaw:
		return r.string()
	default:
		if r.err == nil {
			r.err = fmt.Errorf("unknown field tag %d", tag)
		}
		return ""
	}
}
