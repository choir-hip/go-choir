package computerevent

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const ReceiptVersionV1 = 1

var receiptDomain = []byte("choir-receipt-v1\x00")

type SignerRef struct {
	SignerDomain string `json:"signer_domain"`
	KeyID        string `json:"key_id"`
}

type Signature struct {
	SignerDomain string `json:"signer_domain"`
	KeyID        string `json:"key_id"`
	Signature    string `json:"signature"`
}

type Receipt struct {
	ReceiptVersion         int            `json:"receipt_version"`
	ReceiptKind            string         `json:"receipt_kind"`
	ReceiptID              string         `json:"receipt_id"`
	Issuer                 string         `json:"issuer"`
	IssuedAt               string         `json:"issued_at"`
	RequiredSigners        []SignerRef    `json:"required_signers"`
	CanonicalPayloadSHA256 string         `json:"canonical_payload_sha256"`
	SignatureSet           []Signature    `json:"signature_set"`
	KindFields             map[string]any `json:"-"`
}

type SigningKey struct {
	SignerRef
	PrivateKey ed25519.PrivateKey
}

type KeyResolver interface {
	ResolveReceiptKey(domain, computerID, keyID string, sequence uint64, issuedAt time.Time) (ed25519.PublicKey, error)
}

func NewSignedReceipt(kind, issuer string, fields map[string]any, keys []SigningKey, issuedAt time.Time) (Receipt, error) {
	id, err := uuid.NewV7FromReader(randReader{})
	if err != nil {
		return Receipt{}, fmt.Errorf("receipt: uuidv7: %w", err)
	}
	if issuedAt.Location() != time.UTC {
		issuedAt = issuedAt.UTC()
	}
	receipt := Receipt{
		ReceiptVersion:  ReceiptVersionV1,
		ReceiptKind:     kind,
		ReceiptID:       id.String(),
		Issuer:          issuer,
		IssuedAt:        issuedAt.Format(time.RFC3339Nano),
		RequiredSigners: make([]SignerRef, len(keys)),
		SignatureSet:    make([]Signature, len(keys)),
		KindFields:      cloneFields(fields),
	}
	for i, key := range keys {
		if len(key.PrivateKey) != ed25519.PrivateKeySize || key.SignerDomain == "" || key.KeyID == "" {
			return Receipt{}, fmt.Errorf("receipt: invalid signing key %d", i)
		}
		receipt.RequiredSigners[i] = key.SignerRef
	}
	payload, err := receipt.canonicalPayload()
	if err != nil {
		return Receipt{}, err
	}
	digest := sha256.Sum256(payload)
	receipt.CanonicalPayloadSHA256 = hex.EncodeToString(digest[:])
	preimage := receiptSignaturePreimage(digest[:])
	for i, key := range keys {
		receipt.SignatureSet[i] = Signature{
			SignerDomain: key.SignerDomain,
			KeyID:        key.KeyID,
			Signature:    base64.RawStdEncoding.EncodeToString(ed25519.Sign(key.PrivateKey, preimage)),
		}
	}
	return receipt, nil
}

func (r Receipt) MarshalJSON() ([]byte, error) {
	values, err := r.values(true)
	if err != nil {
		return nil, err
	}
	return CanonicalJSON(values)
}

func (r *Receipt) UnmarshalJSON(data []byte) error {
	var values map[string]json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil {
		return fmt.Errorf("receipt: decode object: %w", err)
	}
	decode := func(name string, target any) error {
		raw, ok := values[name]
		if !ok {
			return fmt.Errorf("receipt: missing %s", name)
		}
		if err := json.Unmarshal(raw, target); err != nil {
			return fmt.Errorf("receipt: decode %s: %w", name, err)
		}
		delete(values, name)
		return nil
	}
	var decoded Receipt
	if err := decode("receipt_version", &decoded.ReceiptVersion); err != nil {
		return err
	}
	if err := decode("receipt_kind", &decoded.ReceiptKind); err != nil {
		return err
	}
	if err := decode("receipt_id", &decoded.ReceiptID); err != nil {
		return err
	}
	if err := decode("issuer", &decoded.Issuer); err != nil {
		return err
	}
	if err := decode("issued_at", &decoded.IssuedAt); err != nil {
		return err
	}
	if err := decode("required_signers", &decoded.RequiredSigners); err != nil {
		return err
	}
	if err := decode("canonical_payload_sha256", &decoded.CanonicalPayloadSHA256); err != nil {
		return err
	}
	if err := decode("signature_set", &decoded.SignatureSet); err != nil {
		return err
	}
	decoded.KindFields = make(map[string]any, len(values))
	for name, raw := range values {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		var value any
		if err := decoder.Decode(&value); err != nil {
			return fmt.Errorf("receipt: decode kind field %s: %w", name, err)
		}
		decoded.KindFields[name] = value
	}
	*r = decoded
	return nil
}

func (r Receipt) CanonicalBytes() ([]byte, error) {
	return r.MarshalJSON()
}

func (r Receipt) RequireKindFields(names ...string) error {
	if len(r.KindFields) != len(names) {
		return fmt.Errorf("receipt: kind field count mismatch")
	}
	for _, name := range names {
		if _, ok := r.KindFields[name]; !ok {
			return fmt.Errorf("receipt: missing kind field %q", name)
		}
	}
	return nil
}

func (r Receipt) Verify(resolver KeyResolver) error {
	if resolver == nil {
		return fmt.Errorf("receipt: key resolver is required")
	}
	if r.ReceiptVersion != ReceiptVersionV1 || r.ReceiptKind == "" || r.Issuer == "" {
		return fmt.Errorf("receipt: invalid common fields")
	}
	id, err := uuid.Parse(r.ReceiptID)
	if err != nil || id.Version() != 7 {
		return fmt.Errorf("receipt: receipt_id must be UUIDv7")
	}
	issuedAt, err := time.Parse(time.RFC3339Nano, r.IssuedAt)
	if err != nil || issuedAt.Location() != time.UTC || issuedAt.Format(time.RFC3339Nano) != r.IssuedAt {
		return fmt.Errorf("receipt: issued_at must be canonical UTC RFC3339")
	}
	if len(r.RequiredSigners) == 0 || len(r.RequiredSigners) != len(r.SignatureSet) {
		return fmt.Errorf("receipt: signature set does not match required signers")
	}
	payload, err := r.canonicalPayload()
	if err != nil {
		return err
	}
	digest := sha256.Sum256(payload)
	if hex.EncodeToString(digest[:]) != r.CanonicalPayloadSHA256 {
		return fmt.Errorf("receipt: canonical payload digest mismatch")
	}
	computerID, _ := r.KindFields["computer_id"].(string)
	sequence, err := receiptSequence(r.KindFields["sequence"])
	if err != nil {
		return err
	}
	preimage := receiptSignaturePreimage(digest[:])
	seen := make(map[SignerRef]struct{}, len(r.RequiredSigners))
	for i, required := range r.RequiredSigners {
		actual := r.SignatureSet[i]
		if required.SignerDomain == "" || required.KeyID == "" || actual.SignerDomain != required.SignerDomain || actual.KeyID != required.KeyID {
			return fmt.Errorf("receipt: signer %d does not match required signer", i)
		}
		if _, duplicate := seen[required]; duplicate {
			return fmt.Errorf("receipt: duplicate required signer %q/%q", required.SignerDomain, required.KeyID)
		}
		seen[required] = struct{}{}
		sig, err := base64.RawStdEncoding.DecodeString(actual.Signature)
		if err != nil || len(sig) != ed25519.SignatureSize {
			return fmt.Errorf("receipt: invalid signature encoding for %q/%q", required.SignerDomain, required.KeyID)
		}
		publicKey, err := resolver.ResolveReceiptKey(required.SignerDomain, computerID, required.KeyID, sequence, issuedAt)
		if err != nil {
			return fmt.Errorf("receipt: resolve signer %q/%q: %w", required.SignerDomain, required.KeyID, err)
		}
		if !ed25519.Verify(publicKey, preimage, sig) {
			return fmt.Errorf("receipt: invalid signature for %q/%q", required.SignerDomain, required.KeyID)
		}
	}
	return nil
}

func (r Receipt) canonicalPayload() ([]byte, error) {
	values, err := r.values(false)
	if err != nil {
		return nil, err
	}
	return CanonicalJSON(values)
}

func (r Receipt) values(includeAttestations bool) (map[string]any, error) {
	values := cloneFields(r.KindFields)
	for _, reserved := range []string{"receipt_version", "receipt_kind", "receipt_id", "issuer", "issued_at", "required_signers", "canonical_payload_sha256", "signature_set"} {
		if _, exists := values[reserved]; exists {
			return nil, fmt.Errorf("receipt: kind field %q is reserved", reserved)
		}
	}
	values["receipt_version"] = r.ReceiptVersion
	values["receipt_kind"] = r.ReceiptKind
	values["receipt_id"] = r.ReceiptID
	values["issuer"] = r.Issuer
	values["issued_at"] = r.IssuedAt
	values["required_signers"] = nonNilSignerRefs(r.RequiredSigners)
	if includeAttestations {
		values["canonical_payload_sha256"] = r.CanonicalPayloadSHA256
		values["signature_set"] = nonNilSignatures(r.SignatureSet)
	}
	return values, nil
}

func receiptSignaturePreimage(digest []byte) []byte {
	preimage := make([]byte, 0, len(receiptDomain)+len(digest))
	preimage = append(preimage, receiptDomain...)
	preimage = append(preimage, digest...)
	return preimage
}

func receiptSequence(value any) (uint64, error) {
	if value == nil {
		return 0, nil
	}
	switch sequence := value.(type) {
	case uint64:
		return sequence, nil
	case int:
		if sequence < 0 {
			return 0, fmt.Errorf("receipt: negative sequence")
		}
		return uint64(sequence), nil
	case json.Number:
		parsed, err := sequence.Int64()
		if err != nil || parsed < 0 {
			return 0, fmt.Errorf("receipt: invalid sequence")
		}
		return uint64(parsed), nil
	default:
		return 0, fmt.Errorf("receipt: unsupported sequence type %T", value)
	}
}

func cloneFields(fields map[string]any) map[string]any {
	cloned := make(map[string]any, len(fields)+8)
	for key, value := range fields {
		cloned[key] = value
	}
	return cloned
}

func nonNilSignerRefs(values []SignerRef) []SignerRef {
	if values == nil {
		return []SignerRef{}
	}
	return values
}

func nonNilSignatures(values []Signature) []Signature {
	if values == nil {
		return []Signature{}
	}
	return values
}

// randReader isolates the randomness dependency for UUIDv7 construction.
type randReader struct{}

func (randReader) Read(p []byte) (int, error) {
	return rand.Read(p)
}
