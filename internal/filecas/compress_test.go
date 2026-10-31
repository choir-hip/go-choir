package filecas

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// Failure modes: old raw-sealed chunk must open; incompressible chunk must be
// byte-identical to the old format; compressible chunk must round-trip and
// shrink; sealing is deterministic; wrong key/computer refused in both forms;
// tampered compressed chunk refused; decompression bomb capped.

func compressibleChunk() []byte {
	return bytes.Repeat([]byte("sqlite page row data 0123456789 "), 4096)
}

func incompressibleChunk(n int) []byte {
	out := make([]byte, 0, n)
	var ctr uint64
	for len(out) < n {
		sum := sha256.Sum256([]byte{byte(ctr), byte(ctr >> 8), byte(ctr >> 16), 7})
		out = append(out, sum[:]...)
		ctr++
	}
	return out[:n]
}

func TestCompressedChunkRoundTripsSmallerAndDeterministic(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	plain := compressibleChunk()
	sealed, digest, err := SealChunk(key, "computer-1", plain)
	if err != nil {
		t.Fatal(err)
	}
	if len(sealed) >= len(plain)/4 {
		t.Fatalf("sealed %d bytes for %d plaintext, want much smaller", len(sealed), len(plain))
	}
	got, err := OpenChunk(key, "computer-1", digest, sealed)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("round trip failed: %v", err)
	}
	again, againDigest, _ := SealChunk(key, "computer-1", plain)
	if againDigest != digest || !bytes.Equal(again, sealed) {
		t.Fatal("compressed sealing is not deterministic")
	}
}

func TestIncompressibleChunkSealsInOldFormat(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	plain := incompressibleChunk(64 << 10)
	sealed, digest, err := SealChunk(key, "computer-1", plain)
	if err != nil {
		t.Fatal(err)
	}
	nonce, _ := chunkNonce(key, "computer-1", plain)
	old, oldDigest, err := sealChunkWithNonce(key, "computer-1", nonce, plain, []byte("computer-1"))
	if err != nil {
		t.Fatal(err)
	}
	if digest != oldDigest || !bytes.Equal(sealed, old) {
		t.Fatal("incompressible chunk differs from the old raw format")
	}
	got, err := OpenChunk(key, "computer-1", digest, sealed)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("open: %v", err)
	}
}

func TestOldRawSealedCompressibleChunkStillOpens(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	plain := compressibleChunk()
	nonce, _ := chunkNonce(key, "computer-1", plain)
	old, digest, _ := sealChunkWithNonce(key, "computer-1", nonce, plain, []byte("computer-1"))
	got, err := OpenChunk(key, "computer-1", digest, old)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("old chunk: %v", err)
	}
}

func TestChunkRefusals(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	other := bytes.Repeat([]byte{8}, 32)
	sealed, digest, _ := SealChunk(key, "computer-1", compressibleChunk())
	if _, err := OpenChunk(other, "computer-1", digest, sealed); err == nil {
		t.Fatal("wrong key opened a compressed chunk")
	}
	if _, err := OpenChunk(key, "computer-2", digest, sealed); err == nil {
		t.Fatal("wrong computer opened a compressed chunk")
	}
	tampered := append([]byte(nil), sealed...)
	tampered[len(tampered)-1] ^= 1
	sum := sha256.Sum256(tampered)
	if _, err := OpenChunk(key, "computer-1", hex.EncodeToString(sum[:]), tampered); err == nil {
		t.Fatal("tampered compressed chunk opened")
	}
	raw, rawDigest, _ := SealChunk(key, "computer-1", incompressibleChunk(4096))
	if _, err := OpenChunk(other, "computer-1", rawDigest, raw); err == nil {
		t.Fatal("wrong key opened a raw chunk")
	}
	if _, err := OpenChunk(key, "computer-2", rawDigest, raw); err == nil {
		t.Fatal("wrong computer opened a raw chunk")
	}
}

func TestDecompressionBombIsCapped(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	bomb := make([]byte, 80<<20) // zeros compress to a few bytes; the cap is 64 MiB
	comp := zstdEncoder.EncodeAll(bomb, nil)
	nonce, _ := chunkNonce(key, "computer-1", comp)
	sealed, digest, err := sealChunkWithNonce(key, "computer-1", nonce, comp, compressedAAD("computer-1"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OpenChunk(key, "computer-1", digest, sealed); err == nil {
		t.Fatal("bomb expanded past the cap")
	}
}
