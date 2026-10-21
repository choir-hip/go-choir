package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"crypto/ed25519"
	"crypto/rand"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func sum(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func writeFile(t *testing.T, root, rel string, b []byte) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o640); err != nil {
		t.Fatal(err)
	}
}

func legacyPin(t *testing.T, root, computer string, key computerevent.SigningKey, nonce string) {
	t.Helper()
	body := []byte(`{"batch":"` + nonce + `"}`)
	d := sum(body)
	writeFile(t, root, "sha256/computer-event-payload/"+d, body)
	r, err := computerevent.NewSignedReceipt("PinReceipt", "corpusd", map[string]any{
		"computer_id": computer, "artifact_digest": d, "media_type": computerevent.ProjectionBatchMediaType,
		"length": len(body), "privacy_class": "owner", "pin_namespace": "computer-event-payload",
		"pin_intent_commitment": strings.Repeat("a", 64),
	}, []computerevent.SigningKey{key}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := r.CanonicalBytes()
	writeFile(t, root, "sha256/pin-receipts/"+sum(raw)+".json", raw)
}

func tool(args ...string) (int, string, string) {
	var o, e bytes.Buffer
	code := run(args, &o, &e)
	return code, o.String(), e.String()
}

// The operator path of the drill: plan, copy, census, each with its exit
// status, and the refusals that keep the tool copy-only and scoped.
func TestTaperepackToolPath(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	key := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform", KeyID: "t"}, PrivateKey: priv}
	root := t.TempDir()
	legacyPin(t, root, "computer-a", key, "1")
	legacyPin(t, root, "computer-a", key, "2")
	scratch := t.TempDir()
	manifest := filepath.Join(scratch, "manifest.jsonl")

	if code, _, _ := tool(); code == 0 {
		t.Fatal("no subcommand accepted")
	}
	if code, _, _ := tool("plan", "--root", root, "--out", manifest); code == 0 {
		t.Fatal("plan with no --computer ran")
	}
	// The manifest may not be written under the artifacts root.
	if code, _, _ := tool("plan", "--root", root, "--computer", "computer-a", "--out", filepath.Join(root, "m.jsonl")); code == 0 {
		t.Fatal("plan wrote its manifest under the artifacts root")
	}
	code, out, errOut := tool("plan", "--root", root, "--computer", "computer-a", "--out", manifest)
	if code != 0 || !strings.Contains(out, `"eligible":2`) {
		t.Fatalf("plan: %d %q %q", code, out, errOut)
	}
	if code, out, _ := tool("census", "--root", root, "--manifest", manifest); code == 0 || !strings.Contains(out, `"missing":4`) {
		t.Fatalf("census before copy: %d %q", code, out)
	}
	if code, _, _ := tool("copy", "--root", root, "--manifest", manifest); code == 0 {
		t.Fatal("copy with no --computer ran")
	}
	code, out, errOut = tool("copy", "--root", root, "--manifest", manifest, "--computer", "computer-a")
	if code != 0 || !strings.Contains(out, `"records_written":4`) {
		t.Fatalf("copy: %d %q %q", code, out, errOut)
	}
	code, out, errOut = tool("census", "--root", root, "--manifest", manifest)
	if code != 0 || !strings.Contains(out, `"equal":4`) {
		t.Fatalf("census: %d %q %q", code, out, errOut)
	}
	// A restored segment tree, legacy compared from the live root.
	restored := t.TempDir()
	if code, _, _ := tool("census", "--root", restored, "--legacy-root", root, "--manifest", manifest); code == 0 {
		t.Fatal("census on an empty restore passed")
	}
}
