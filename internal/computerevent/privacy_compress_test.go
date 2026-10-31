package computerevent

// Failure modes covered (written before the code):
//  1. a legacy envelope (no compression field) must still open;
//  2. a small or incompressible payload must produce exactly today's envelope
//     (no "compression" key) so digests of those artifacts do not change;
//  3. a compressible payload must round-trip and be smaller, with
//     PlaintextLength staying the uncompressed size;
//  4. flipping/removing/adding the compression marker must fail authentication;
//  5. wrong computer/event must be refused for compressed envelopes too;
//  6. a decompression bomb must be refused by the output cap.

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"golang.org/x/crypto/chacha20poly1305"
)

// The release holds compressed writes back; these tests exercise them.
func init() { compressPrivateWrites = true }

func realisticJSON(n int) []byte {
	var b strings.Builder
	b.WriteString(`{"tool_results":[`)
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"call_id":"call-%06d","tool":"read_file","path":"internal/agentcore/runtime.go","status":"ok","output":"func (r *Runtime) Step(ctx context.Context) error { return r.turn(ctx, %d) }","elapsed_ms":%d}`, i, i, 10+i%37)
	}
	b.WriteString(`]}`)
	return []byte(b.String())
}

func TestCompressedPrivatePayloadRoundTripsSmaller(t *testing.T) {
	c := externalTestCipher(t, 0x31)
	pt := realisticJSON(200)
	env, md, err := c.Encrypt(context.Background(), "computer-test", "ev", "application/json", "private", pt)
	if err != nil {
		t.Fatal(err)
	}
	if md.Compression != "zstd" || md.PlaintextLength != len(pt) {
		t.Fatalf("metadata: %+v len=%d", md, len(pt))
	}
	t.Logf("realistic JSON: plaintext=%d envelope=%d ratio=%.3f", len(pt), len(env), float64(len(env))/float64(len(pt)))
	if len(env) >= len(pt)/2 {
		t.Fatalf("envelope not meaningfully smaller: %d vs %d", len(env), len(pt))
	}
	got, _, err := c.Decrypt(context.Background(), env, "computer-test", "ev")
	if err != nil || !bytes.Equal(got, pt) {
		t.Fatalf("round trip: %v", err)
	}
	if _, _, err := c.Decrypt(context.Background(), env, "computer-other", "ev"); err == nil {
		t.Fatal("wrong computer accepted")
	}
	if _, _, err := c.Decrypt(context.Background(), env, "computer-test", "ev-other"); err == nil {
		t.Fatal("wrong event accepted")
	}
}

func TestSmallAndIncompressiblePayloadsKeepLegacyEnvelope(t *testing.T) {
	c := externalTestCipher(t, 0x32)
	random := make([]byte, 4096)
	_, _ = rand.Read(random)
	for name, pt := range map[string][]byte{"small": bytes.Repeat([]byte("a"), 200), "random": random} {
		env, md, err := c.Encrypt(context.Background(), "computer-test", "ev", "application/octet-stream", "private", pt)
		if err != nil {
			t.Fatal(err)
		}
		if md.Compression != "" || bytes.Contains(env, []byte("compression")) {
			t.Fatalf("%s: envelope carries a compression marker", name)
		}
		_, _, ct, err := decodePrivateArtifactEnvelope(env)
		if err != nil || len(ct) != len(pt)+chacha20poly1305.Overhead {
			t.Fatalf("%s: not a legacy-shaped envelope (%v)", name, err)
		}
		if got, _, err := c.Decrypt(context.Background(), env, "computer-test", "ev"); err != nil || !bytes.Equal(got, pt) {
			t.Fatalf("%s: round trip: %v", name, err)
		}
	}
}

func TestLegacyEnvelopeWithoutFieldDecrypts(t *testing.T) {
	c := externalTestCipher(t, 0x33)
	pt := realisticJSON(50)
	env := sealForTest(t, c, "computer-test", "ev", pt, len(pt), "")
	got, md, err := c.Decrypt(context.Background(), env, "computer-test", "ev")
	if err != nil || !bytes.Equal(got, pt) || md.Compression != "" {
		t.Fatalf("legacy open: %v", err)
	}
}

func TestFlippingCompressionMarkerFailsAuthentication(t *testing.T) {
	c := externalTestCipher(t, 0x34)
	pt := realisticJSON(100)
	env, _, err := c.Encrypt(context.Background(), "computer-test", "ev", "application/json", "private", pt)
	if err != nil {
		t.Fatal(err)
	}
	var e privateArtifactEnvelope
	if err := json.Unmarshal(env, &e); err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"", "gzip"} {
		e2 := e
		e2.Metadata.Compression = v
		raw, _ := CanonicalJSON(e2)
		if _, _, err := c.Decrypt(context.Background(), raw, "computer-test", "ev"); err == nil {
			t.Fatalf("marker %q accepted", v)
		}
	}
	small := []byte("short legacy payload")
	legacy := sealForTest(t, c, "computer-test", "ev", small, len(small), "")
	var l privateArtifactEnvelope
	_ = json.Unmarshal(legacy, &l)
	l.Metadata.Compression = "zstd"
	raw, _ := CanonicalJSON(l)
	if _, _, err := c.Decrypt(context.Background(), raw, "computer-test", "ev"); err == nil {
		t.Fatal("marker added to legacy envelope accepted")
	}
}

func TestDecompressionBombIsCapped(t *testing.T) {
	c := externalTestCipher(t, 0x35)
	big := make([]byte, MaxPrivatePlaintextBytes+(1<<20))
	packed := privateZstdEncoder.EncodeAll(big, nil)
	env := sealForTest(t, c, "computer-test", "ev", packed, len(big), "zstd")
	if _, _, err := c.Decrypt(context.Background(), env, "computer-test", "ev"); err == nil {
		t.Fatal("oversized declared plaintext accepted")
	}
	env = sealForTest(t, c, "computer-test", "ev", packed, 10, "zstd")
	if _, _, err := c.Decrypt(context.Background(), env, "computer-test", "ev"); err == nil {
		t.Fatal("bomb with lying length accepted")
	}
}

// sealForTest builds an envelope by hand with arbitrary metadata; sealed is
// the exact byte string placed under the AEAD.
func sealForTest(t *testing.T, c *PrivateArtifactCipher, computerID, eventID string, sealed []byte, plaintextLen int, compression string) []byte {
	t.Helper()
	material, err := c.keys.current(context.Background(), computerID)
	if err != nil {
		t.Fatal(err)
	}
	aead, _ := chacha20poly1305.NewX(material.key[:])
	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	_, _ = rand.Read(nonce)
	md := PrivateArtifactMetadata{
		Version: privateArtifactVersionV1, ComputerID: computerID, EventID: eventID,
		MediaType: "application/json", PlaintextLength: plaintextLen, PrivacyClass: "private",
		KeyVersionDigest: material.digest, Nonce: base64.RawStdEncoding.EncodeToString(nonce),
		Compression: compression,
	}
	aad, _ := CanonicalJSON(md)
	ct := aead.Seal(nil, nonce, sealed, aad)
	env, err := CanonicalJSON(privateArtifactEnvelope{Metadata: md, Ciphertext: base64.RawStdEncoding.EncodeToString(ct)})
	if err != nil {
		t.Fatal(err)
	}
	return env
}
