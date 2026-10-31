package bundle

import (
	"crypto/ed25519"
	"errors"
	"testing"
	"time"
)

// Review finding 3: a retirement cutoff for a key id another entry already
// trusts must not be silently voided by the first entry.
func TestTrustDuplicateKeyRetirementNotVoided(t *testing.T) {
	pub := newSigningKey(t).PrivateKey.Public().(ed25519.PublicKey)
	open := TrustEntry(pub, "pinned")
	cut := TrustEntry(pub, "operator")
	n := uint64(5)
	cut.ComputerID, cut.FirstInvalidSequence = "c1", &n

	// Loading both is refused: the cutoff would never apply.
	if _, err := NewTrustSet(open, cut); err == nil {
		t.Fatal("an unscoped entry and a retirement cutoff for the same key id loaded together")
	}
	// The retirement alone refuses at and after the cutoff.
	set, err := NewTrustSet(cut)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := set.ResolveReceiptKey(SignerDomain, "c1", cut.KeyID, 5, time.Now()); !errors.Is(err, ErrUntrustedKey) {
		t.Fatalf("receipt at the cutoff resolved: %v", err)
	}
	if _, err := set.ResolveReceiptKey(SignerDomain, "c1", cut.KeyID, 4, time.Now()); err != nil {
		t.Fatalf("receipt before the cutoff: %v", err)
	}
	// Scoped entries for different computers coexist.
	other := TrustEntry(pub, "other")
	other.ComputerID = "c2"
	if _, err := NewTrustSet(cut, other); err != nil {
		t.Fatalf("disjoint scopes refused: %v", err)
	}
	// The same entry twice is harmless.
	if _, err := NewTrustSet(open, open); err != nil {
		t.Fatalf("identical duplicate refused: %v", err)
	}
}

// Resolution refuses when any applicable entry refuses, whatever the order.
func TestTrustResolveAnyRefusalWins(t *testing.T) {
	pub := newSigningKey(t).PrivateKey.Public().(ed25519.PublicKey)
	n := uint64(5)
	cut := TrustEntry(pub, "operator")
	cut.ComputerID, cut.FirstInvalidSequence = "c1", &n
	open := TrustEntry(pub, "pinned")
	set := &TrustSet{keys: map[string][]TrustedKey{}}
	for _, e := range []TrustedKey{open, cut} { // bypass Add's conflict check
		e.public = pub
		set.keys[SignerDomain+"/"+e.KeyID] = append(set.keys[SignerDomain+"/"+e.KeyID], e)
	}
	if _, err := set.ResolveReceiptKey(SignerDomain, "c1", cut.KeyID, 9, time.Now()); !errors.Is(err, ErrUntrustedKey) {
		t.Fatalf("a retired entry was voided by an unscoped one: %v", err)
	}
}
