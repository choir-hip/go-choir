package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// The operator path: census, dry run, apply, census, with their exit codes
// and the refusals that keep the tool scoped and copy-only.
func TestTaperepackToolPath(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	key := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform", KeyID: "t"}, PrivateKey: priv}
	root, work := t.TempDir(), t.TempDir()
	for _, n := range []string{"1", "2", "3"} {
		legacyPin(t, root, "computer-a", key, n)
	}
	legacyPin(t, root, "computer-b", key, "b1")
	base := []string{"--root", root, "--work-dir", work, "--quiet"}

	// Nothing is copied yet: the census runs and exits 3.
	code, out, errs := tool(append([]string{"census"}, base...)...)
	if code != 3 {
		t.Fatalf("census before the copy: exit %d, stderr %q", code, errs)
	}
	var rep struct {
		Receipt string `json:"receipt"`
		Mode    string `json:"mode"`
		Files   map[string]struct {
			Files, Uncovered, Covered int64
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil || rep.Mode != "census" || rep.Files["payload"].Files != 4 || rep.Files["payload"].Uncovered != 4 || len(rep.Receipt) != 64 {
		t.Fatalf("census output: %v %q", err, out)
	}

	// Dry run by default; --apply refuses without a computer.
	if code, _, errs = tool(append([]string{"copy", "--computer", "computer-a"}, base...)...); code != 0 {
		t.Fatalf("dry run: exit %d %q", code, errs)
	}
	if _, err := os.Stat(filepath.Join(root, "tape-seg")); err == nil {
		t.Fatal("the dry run wrote tape-seg")
	}
	if code, _, _ = tool(append([]string{"copy", "--apply"}, base...)...); code == 0 {
		t.Fatal("--apply without --computer ran")
	}
	if _, err := os.Stat(filepath.Join(root, "tape-seg")); err == nil {
		t.Fatal("a refused apply wrote tape-seg")
	}
	if code, _, errs = tool(append([]string{"copy", "--apply", "--computer", "computer-a"}, base...)...); code != 0 {
		t.Fatalf("apply: exit %d %q", code, errs)
	}
	if _, err := os.Stat(filepath.Join(root, "tape-seg", "computer-b")); err == nil {
		t.Fatal("computer-b was copied without being named")
	}
	// B's files are uncovered, so the census still exits 3; limited to A it passes.
	if code, _, _ = tool(append([]string{"census", "--computer", "computer-a"}, base...)...); code != 0 {
		t.Fatalf("census of A after the copy: exit %d", code)
	}
	if code, _, _ = tool(append([]string{"census"}, base...)...); code != 3 {
		t.Fatalf("census with B uncopied: exit %d", code)
	}
	// The work directory may not be inside the artifacts root.
	if code, _, _ = tool("census", "--root", root, "--work-dir", filepath.Join(root, "w"), "--quiet"); code == 0 {
		t.Fatal("a work dir inside the root was accepted")
	}
	// A legacy file is never touched.
	if _, err := os.Stat(filepath.Join(root, "sha256", "computer-event-payload")); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := tool("bogus"); code != 2 {
		t.Fatalf("unknown subcommand: exit %d", code)
	}
}
