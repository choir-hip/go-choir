package computerevent

import (
	"bytes"
	"context"
	"testing"
)

// DeriveSecret: stable per key and label, distinct across labels and keys,
// never the key itself, and bound to the keyring's computer.
func TestDeriveSecretIsDomainSeparated(t *testing.T) {
	ctx := context.Background()
	key := bytes.Repeat([]byte{9}, 32)
	c, err := NewPrivateArtifactCipher("computer-a", key)
	if err != nil {
		t.Fatal(err)
	}
	a1, err := c.DeriveSecret(ctx, "computer-a", "calibration-salt-v2")
	if err != nil || len(a1) != 32 {
		t.Fatalf("derive: %v", err)
	}
	a2, _ := c.DeriveSecret(ctx, "computer-a", "calibration-salt-v2")
	b, _ := c.DeriveSecret(ctx, "computer-a", "another-use")
	if !bytes.Equal(a1, a2) || bytes.Equal(a1, b) || bytes.Equal(a1, key) {
		t.Fatal("derived secret not stable, not separated, or the key")
	}
	other, _ := NewPrivateArtifactCipher("computer-a", bytes.Repeat([]byte{8}, 32))
	if o, _ := other.DeriveSecret(ctx, "computer-a", "calibration-salt-v2"); bytes.Equal(o, a1) {
		t.Fatal("two keys derived one secret")
	}
	if _, err := c.DeriveSecret(ctx, "computer-b", "calibration-salt-v2"); err == nil {
		t.Fatal("derived for another computer")
	}
	if _, err := c.DeriveSecret(ctx, "computer-a", " "); err == nil {
		t.Fatal("derived without a label")
	}
}
