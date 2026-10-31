package bundle

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// The receipts of a bundle (event head receipts and pin receipts) are signed
// by the platform-control key of the host whose corpusd appended the event.
// A bundle is authentic to an importing host only when every receipt is
// signed by a key that host already trusts. The trust set never comes from
// the bundle: a bundle that carried its own keys would authenticate itself
// (docs/design/bundle-authenticity-2026-10-31.md, section 3).
//
// An importing host's trust set is the union of
//
//   - the repository-pinned staging platform-control key
//     (computerevent.PlatformControlTrustKeyID), unless the caller drops it;
//   - the trust files the operator names (choir-bundle --trust <file>), each
//     provisioned out of band, for example with `choir-bundle trust-key` run
//     on the exporting host and copied over an authenticated channel.
//
// One key id has one meaning wherever two entries could both apply: loading
// a second entry for an id with different constraints (a retirement cutoff
// beside the unscoped pinned entry) is refused, and resolution refuses if
// any applicable entry refuses. To retire the pinned key, pass
// --no-pinned-trust and list the key with its cutoff in a trust file.
//
// A key id is the first 16 hex characters of the sha256 of the public key
// (internal/platform signing.go newSigningKey); an entry whose id does not
// match its key is refused at load, so an id can never be rebound to another
// key.

// SignerDomain is the receipt signer domain of corpusd's platform-control key.
const SignerDomain = "platform-control"

// ReceiptIssuer is the issuer of every receipt a bundle carries.
const ReceiptIssuer = "corpusd"

// TrustedKey is one platform-control public key the importing host trusts.
//
// A retired key (planned rotation) keeps its entry with a cutoff: receipts
// at or after FirstInvalidSequence of ComputerID, or issued at or after
// FirstInvalidTime, are refused. A compromised key is removed from the trust
// set instead: whoever holds it can sign a whole forged chain with any
// sequence or time, so no cutoff contains it.
type TrustedKey struct {
	SignerDomain string `json:"signer_domain"`
	KeyID        string `json:"key_id"`
	// PublicKey is the ed25519 public key, base64 (standard or raw).
	PublicKey string `json:"public_key"`
	// Label names where the key lives (a host), for operators.
	Label string `json:"label,omitempty"`
	// ComputerID, when set, scopes the key to one computer.
	ComputerID string `json:"computer_id,omitempty"`
	// FirstInvalidSequence needs ComputerID: sequences are per computer.
	FirstInvalidSequence *uint64    `json:"first_invalid_sequence,omitempty"`
	FirstInvalidTime     *time.Time `json:"first_invalid_time,omitempty"`

	public ed25519.PublicKey
}

// TrustFile is a trust file's JSON.
type TrustFile struct {
	Keys []TrustedKey `json:"keys"`
}

// TrustSet resolves receipt signer keys for verification. The zero value
// trusts nothing.
type TrustSet struct {
	keys map[string][]TrustedKey // signer domain + "/" + key id
}

// ErrUntrustedKey is wrapped when a receipt names a key the set does not hold.
var ErrUntrustedKey = errors.New("bundle: receipt signer is not trusted")

// KeyIDFor is the platform key id of a public key.
func KeyIDFor(public ed25519.PublicKey) string {
	sum := sha256.Sum256(public)
	return hex.EncodeToString(sum[:])[:16]
}

// TrustEntry returns the trust entry of a platform-control public key.
func TrustEntry(public ed25519.PublicKey, label string) TrustedKey {
	return TrustedKey{SignerDomain: SignerDomain, KeyID: KeyIDFor(public), PublicKey: base64.StdEncoding.EncodeToString(public), Label: label}
}

// NewTrustSet builds a set from entries, checking each.
func NewTrustSet(entries ...TrustedKey) (*TrustSet, error) {
	t := &TrustSet{keys: map[string][]TrustedKey{}}
	for _, e := range entries {
		if err := t.Add(e); err != nil {
			return nil, err
		}
	}
	return t, nil
}

// PinnedTrust is the repository-pinned staging platform-control key.
func PinnedTrust() TrustedKey {
	return TrustedKey{SignerDomain: SignerDomain, KeyID: computerevent.PlatformControlTrustKeyID, PublicKey: computerevent.PlatformControlTrustPublicKey, Label: "repository-pinned staging platform-control key"}
}

// Add checks e and adds it.
func (t *TrustSet) Add(e TrustedKey) error {
	if t.keys == nil {
		t.keys = map[string][]TrustedKey{}
	}
	if e.SignerDomain == "" {
		e.SignerDomain = SignerDomain
	}
	raw, err := decodeKey(e.PublicKey)
	if err != nil {
		return fmt.Errorf("bundle: trusted key %q: %w", e.KeyID, err)
	}
	if KeyIDFor(raw) != e.KeyID {
		return fmt.Errorf("bundle: trusted key id %q does not name its public key (want %s)", e.KeyID, KeyIDFor(raw))
	}
	if e.FirstInvalidSequence != nil && e.ComputerID == "" {
		return fmt.Errorf("bundle: trusted key %s: a sequence cutoff needs a computer id", e.KeyID)
	}
	e.public = raw
	k := e.SignerDomain + "/" + e.KeyID
	// One key id, one meaning wherever two entries could both apply: a
	// retirement cutoff in a later entry must not be voided by an earlier
	// unscoped one (retiring the pinned key needs --no-pinned-trust).
	for _, have := range t.keys[k] {
		if (have.ComputerID == "" || e.ComputerID == "" || have.ComputerID == e.ComputerID) && !sameConstraints(have, e) {
			return fmt.Errorf("bundle: trusted key %s is already trusted with other constraints for an overlapping scope; to retire the repository-pinned key, pass --no-pinned-trust and list it in the trust file", e.KeyID)
		}
	}
	t.keys[k] = append(t.keys[k], e)
	return nil
}

func sameConstraints(a, b TrustedKey) bool {
	eqSeq := (a.FirstInvalidSequence == nil && b.FirstInvalidSequence == nil) || (a.FirstInvalidSequence != nil && b.FirstInvalidSequence != nil && *a.FirstInvalidSequence == *b.FirstInvalidSequence)
	eqTime := (a.FirstInvalidTime == nil && b.FirstInvalidTime == nil) || (a.FirstInvalidTime != nil && b.FirstInvalidTime != nil && a.FirstInvalidTime.Equal(*b.FirstInvalidTime))
	return a.ComputerID == b.ComputerID && eqSeq && eqTime
}

// Len is the number of entries.
func (t *TrustSet) Len() int {
	if t == nil {
		return 0
	}
	n := 0
	for _, v := range t.keys {
		n += len(v)
	}
	return n
}

func decodeKey(s string) (ed25519.PublicKey, error) {
	s = strings.TrimSpace(s)
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding} {
		if raw, err := enc.DecodeString(s); err == nil {
			if len(raw) != ed25519.PublicKeySize {
				return nil, fmt.Errorf("public key is %d bytes, want %d", len(raw), ed25519.PublicKeySize)
			}
			return ed25519.PublicKey(raw), nil
		}
	}
	return nil, fmt.Errorf("public key is not base64")
}

// LoadTrustFile reads a trust file into t.
func (t *TrustSet) LoadTrustFile(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("bundle: read trust file: %w", err)
	}
	var f TrustFile
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return fmt.Errorf("bundle: decode trust file %s: %w", path, err)
	}
	if dec.More() {
		return fmt.Errorf("bundle: trust file %s has data after its first JSON document", path)
	}
	if len(f.Keys) == 0 {
		return fmt.Errorf("bundle: trust file %s names no keys", path)
	}
	for _, e := range f.Keys {
		if err := t.Add(e); err != nil {
			return fmt.Errorf("bundle: trust file %s: %w", path, err)
		}
	}
	return nil
}

// ResolveReceiptKey implements computerevent.KeyResolver. sequence is the
// tape sequence the receipt belongs to (the verifier passes the event's
// sequence for its pin receipts, which carry none).
func (t *TrustSet) ResolveReceiptKey(domain, computerID, keyID string, sequence uint64, issuedAt time.Time) (ed25519.PublicKey, error) {
	if t == nil {
		return nil, ErrUntrustedKey
	}
	if domain != SignerDomain {
		return nil, fmt.Errorf("%w: signer domain %q", ErrUntrustedKey, domain)
	}
	entries := t.keys[domain+"/"+keyID]
	var refusal error
	var found ed25519.PublicKey
	for _, e := range entries {
		if e.ComputerID != "" && e.ComputerID != computerID {
			continue
		}
		if e.FirstInvalidSequence != nil && sequence >= *e.FirstInvalidSequence {
			refusal = fmt.Errorf("%w: key %s is retired from sequence %d of %s (receipt at %d)", ErrUntrustedKey, keyID, *e.FirstInvalidSequence, e.ComputerID, sequence)
			continue
		}
		if e.FirstInvalidTime != nil && !issuedAt.Before(*e.FirstInvalidTime) {
			refusal = fmt.Errorf("%w: key %s is retired from %s (receipt issued %s)", ErrUntrustedKey, keyID, e.FirstInvalidTime.UTC().Format(time.RFC3339), issuedAt.UTC().Format(time.RFC3339Nano))
			continue
		}
		if found == nil {
			found = e.public
		}
	}
	if refusal != nil {
		return nil, refusal
	}
	if found != nil {
		return found, nil
	}
	return nil, fmt.Errorf("%w: %s/%s", ErrUntrustedKey, domain, keyID)
}

// sequenceResolver pins the sequence a pin receipt is checked at to its
// event's.
type sequenceResolver struct {
	trust    *TrustSet
	sequence uint64
}

func (r sequenceResolver) ResolveReceiptKey(domain, computerID, keyID string, _ uint64, issuedAt time.Time) (ed25519.PublicKey, error) {
	return r.trust.ResolveReceiptKey(domain, computerID, keyID, r.sequence, issuedAt)
}
