package filecas

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"testing"
	"time"
)

func TestChunkBytesEdges(t *testing.T) {
	cases := []struct {
		name string
		data string
		want []string
	}{
		{name: "empty", want: []string{}},
		{name: "exact multiple", data: "abcdef", want: []string{"abc", "def"}},
		{name: "remainder", data: "abcdefg", want: []string{"abc", "def", "g"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ChunkBytes([]byte(tc.data), 3)
			if len(got) != len(tc.want) {
				t.Fatalf("chunk count = %d, want %d", len(got), len(tc.want))
			}
			for i := range got {
				if string(got[i]) != tc.want[i] {
					t.Fatalf("chunk %d = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestManifestDeterminism(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 123, time.UTC)
	entries := []FileEntry{{Path: "z", Mode: 0o600, Size: 1}, {Path: "a", Mode: 0o644, Size: 2}}
	first, err := BuildManifest("computer-1", entries, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildManifest("computer-1", []FileEntry{entries[1], entries[0]}, now)
	if err != nil {
		t.Fatal(err)
	}
	if first.Root != second.Root || first.Files[0].Path != "a" {
		t.Fatalf("roots/order = %q/%q, first=%+v", first.Root, second.Root, first.Files)
	}
	if err := first.VerifyRoot(); err != nil {
		t.Fatal(err)
	}
}

func TestSealOpenChunk(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	sealed, digest, err := SealChunk(key, "computer-1", []byte("private chunk"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := OpenChunk(key, "computer-1", digest, sealed)
	if err != nil || string(plain) != "private chunk" {
		t.Fatalf("open = %q, %v", plain, err)
	}
	tampered := append([]byte(nil), sealed...)
	tampered[len(tampered)-1] ^= 1
	if _, err := OpenChunk(key, "computer-1", digest, tampered); err == nil {
		t.Fatal("tampered chunk accepted")
	}
	if _, err := OpenChunk(key, "computer-other", digest, sealed); err == nil {
		t.Fatal("wrong computer accepted")
	}
}

// Failure mode (artifact-store-grows-12gb-a-day-2026-10-14): a random nonce
// gave every re-seal of an unchanged chunk a new CAS digest, so each guest boot
// stored its whole Files tree again. Sealing must be a function of (key,
// computer, plaintext) so the platform's immutable PUT deduplicates.
func TestSealChunkIsDeterministicPerComputer(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	first, firstDigest, err := SealChunk(key, "computer-1", []byte("private chunk"))
	if err != nil {
		t.Fatal(err)
	}
	again, againDigest, err := SealChunk(key, "computer-1", []byte("private chunk"))
	if err != nil {
		t.Fatal(err)
	}
	if firstDigest != againDigest || !bytes.Equal(first, again) {
		t.Fatal("re-sealing an unchanged chunk produced a new ciphertext")
	}
	_, otherChunk, err := SealChunk(key, "computer-1", []byte("private chunk 2"))
	if err != nil {
		t.Fatal(err)
	}
	_, otherComputer, err := SealChunk(key, "computer-2", []byte("private chunk"))
	if err != nil {
		t.Fatal(err)
	}
	_, otherKey, err := SealChunk(bytes.Repeat([]byte{8}, 32), "computer-1", []byte("private chunk"))
	if err != nil {
		t.Fatal(err)
	}
	if otherChunk == firstDigest || otherComputer == firstDigest || otherKey == firstDigest {
		t.Fatal("distinct plaintext, computer or key shared a digest")
	}
	// Distinct plaintexts must never share a nonce (nonce reuse under one key
	// breaks the AEAD): the nonce is the 24-byte prefix.
	a, _, _ := SealChunk(key, "computer-1", []byte("aaaa"))
	b, _, _ := SealChunk(key, "computer-1", []byte("aaab"))
	if bytes.Equal(a[:24], b[:24]) {
		t.Fatal("distinct plaintexts shared a nonce")
	}
	plain, err := OpenChunk(key, "computer-1", firstDigest, first)
	if err != nil || string(plain) != "private chunk" {
		t.Fatalf("open = %q, %v", plain, err)
	}
}

// Chunks sealed before the change carry random nonces; they must still open.
func TestOpenChunkAcceptsRandomNonceChunks(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	nonce := bytes.Repeat([]byte{0x5a}, 24)
	sealed, digest, err := sealChunkWithNonce(key, "computer-1", nonce, []byte("legacy chunk"), []byte("computer-1"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := OpenChunk(key, "computer-1", digest, sealed)
	if err != nil || string(plain) != "legacy chunk" {
		t.Fatalf("legacy open = %q, %v", plain, err)
	}
}

// The file object's keyed digest key (docs/design/file-object-2026-10-24.md
// section 3): derived from the DEK and computer, separate from every chunk key
// and nonce key, so the platform host that holds the tape cannot test a
// plaintext against a recorded digest.
func TestFileDigestKeyIsPerComputerAndSeparateFromChunkKeys(t *testing.T) {
	dek := bytes.Repeat([]byte{7}, 32)
	a, err := FileDigestKey(dek, "computer-a")
	if err != nil || len(a) != 32 {
		t.Fatalf("key = %x, %v", a, err)
	}
	again, _ := FileDigestKey(dek, "computer-a")
	other, _ := FileDigestKey(dek, "computer-b")
	otherDEK, _ := FileDigestKey(bytes.Repeat([]byte{8}, 32), "computer-a")
	if !bytes.Equal(a, again) || bytes.Equal(a, other) || bytes.Equal(a, otherDEK) {
		t.Fatal("the digest key must be deterministic per (DEK, computer) and differ across either")
	}
	// Not the chunk nonce key: HMAC(nonce key, chunk) is the sealed-chunk nonce.
	nonce, err := chunkNonce(dek, "computer-a", []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, a)
	mac.Write([]byte("x"))
	if bytes.Equal(mac.Sum(nil)[:len(nonce)], nonce) {
		t.Fatal("the digest key equals the chunk nonce key")
	}
	if _, err := FileDigestKey(dek[:5], "computer-a"); err == nil {
		t.Fatal("a short DEK was accepted")
	}
	if _, err := FileDigestKey(dek, " "); err == nil {
		t.Fatal("an empty computer ID was accepted")
	}
}
