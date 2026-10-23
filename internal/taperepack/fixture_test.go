package taperepack

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func init() { fsyncWork = false }

const (
	compA = "computer-a"
	compB = "computer-b"
)

type fixture struct {
	t    *testing.T
	root string
	key  computerevent.SigningKey
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{t: t, root: t.TempDir(), key: computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform", KeyID: "test"}, PrivateKey: priv}}
}

func hexSum(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func (f *fixture) write(rel string, body []byte) {
	f.t.Helper()
	path := filepath.Join(f.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o640); err != nil {
		f.t.Fatal(err)
	}
}

// pin writes a legacy artifact and its receipt the way the legacy path did
// and returns (artifact digest, receipt digest).
func (f *fixture) pin(computer, namespace, mediaType string, body []byte) (string, string) {
	f.t.Helper()
	digest := hexSum(body)
	f.write("sha256/"+namespace+"/"+digest, body)
	return digest, f.receipt(computer, namespace, mediaType, digest, len(body))
}

func (f *fixture) receipt(computer, namespace, mediaType, digest string, length int) string {
	f.t.Helper()
	raw := f.receiptRaw(computer, namespace, mediaType, digest, length)
	rd := hexSum(raw)
	f.write("sha256/pin-receipts/"+rd+".json", raw)
	return rd
}

// receiptRaw is the signed receipt's bytes, written nowhere.
func (f *fixture) receiptRaw(computer, namespace, mediaType, digest string, length int) []byte {
	f.t.Helper()
	fields := map[string]any{
		"computer_id": computer, "artifact_digest": digest, "media_type": mediaType,
		"length": length, "privacy_class": "owner", "pin_namespace": namespace,
		"request_commitment": strings.Repeat("a", 64),
	}
	r, err := computerevent.NewSignedReceipt("PinReceipt", "corpusd", fields, []computerevent.SigningKey{f.key}, time.Now())
	if err != nil {
		f.t.Fatal(err)
	}
	raw, err := r.CanonicalBytes()
	if err != nil {
		f.t.Fatal(err)
	}
	return raw
}

func eventBody(computer, kind, nonce string) []byte {
	return []byte(`{"computer_id":"` + computer + `","event_kind":"` + kind + `","nonce":"` + nonce + `"}`)
}

func (f *fixture) batchEvent(computer, nonce string) (string, string) {
	return f.pin(computer, "computer-event", "application/vnd.choir.computer-event+json", eventBody(computer, string(computerevent.EventProjectionBatchRecorded), nonce))
}

func (f *fixture) batchPayload(computer, nonce string) (string, string) {
	return f.pin(computer, "computer-event-payload", computerevent.ProjectionBatchMediaType, []byte(`{"batch":"`+nonce+`"}`))
}

// tree records every file under root with its size, mode and mtime.
func tree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		sum := ""
		if info.Mode().IsRegular() {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sum = hexSum(b)
		}
		out[rel] = info.Mode().String() + " " + info.ModTime().String() + " " + sum
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func subtree(all map[string]string, prefix string) map[string]string {
	out := map[string]string{}
	for k, v := range all {
		if k == prefix || strings.HasPrefix(k, prefix+string(filepath.Separator)) {
			out[k] = v
		}
	}
	return out
}

func sameTree(t *testing.T, label string, before, after map[string]string) {
	t.Helper()
	if len(before) != len(after) {
		t.Fatalf("%s: %d entries before, %d after", label, len(before), len(after))
	}
	for k, v := range before {
		if after[k] != v {
			t.Fatalf("%s: %s changed: %q -> %q", label, k, v, after[k])
		}
	}
}
