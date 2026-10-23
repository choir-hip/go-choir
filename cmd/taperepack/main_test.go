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
	// The delete: refused without the census digest, a dry run by default, and
	// only the files with a verified copy go when applied.
	var cen struct {
		Receipt string `json:"receipt"`
	}
	code, out, _ = tool(append([]string{"census", "--computer", "computer-a"}, base...)...)
	if err := json.Unmarshal([]byte(out), &cen); err != nil || code != 0 || len(cen.Receipt) != 64 {
		t.Fatalf("census of A: %d %q", code, out)
	}
	if code, _, _ = tool(append([]string{"delete", "--apply"}, base...)...); code == 0 {
		t.Fatal("delete --apply ran without a census receipt")
	}
	if code, _, _ = tool(append([]string{"delete", "--census-receipt", strings.Repeat("0", 64), "--apply"}, base...)...); code == 0 {
		t.Fatal("delete accepted an unknown census receipt")
	}
	count := func() int {
		n := 0
		_ = filepath.WalkDir(filepath.Join(root, "sha256"), func(_ string, d os.DirEntry, _ error) error {
			if !d.IsDir() {
				n++
			}
			return nil
		})
		return n
	}
	before := count()
	if code, out, errs = tool(append([]string{"delete", "--census-receipt", cen.Receipt, "--computer", "computer-a", "--pause", "1ms"}, base...)...); code != 0 || count() != before {
		t.Fatalf("delete dry run: exit %d, %d -> %d files, %q", code, before, count(), errs)
	}
	// An apply needs the platform's keep list (here: known to be empty).
	emptyKeep := filepath.Join(t.TempDir(), "keep")
	if err := os.WriteFile(emptyKeep, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if code, out, errs = tool(append([]string{"delete", "--census-receipt", cen.Receipt, "--computer", "computer-a", "--apply", "--pause", "1ms", "--keep-digests", emptyKeep, "--keep-digests-empty-ok"}, base...)...); code != 0 {
		t.Fatalf("delete: exit %d %q", code, errs)
	}
	// A's 3 payloads and their 3 receipts are gone; B's payload and receipt stay.
	if got := count(); got != 2 {
		t.Fatalf("after the delete %d legacy files remain, want B's payload and receipt", got)
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

// M2 (review round 4): delete --apply needs --keep-digests, the digests it
// names are never unlinked, a malformed list is an error, and an empty list
// needs --keep-digests-empty-ok. All of these fail on 8722c4932, whose delete
// takes no keep file and applies.
func TestDeleteApplyNeedsTheKeepDigestsFile(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	key := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform", KeyID: "t"}, PrivateKey: priv}
	root, work := t.TempDir(), t.TempDir()
	var digests []string
	for _, n := range []string{"1", "2", "3"} {
		legacyPin(t, root, "computer-a", key, n)
		digests = append(digests, sum([]byte(`{"batch":"`+n+`"}`)))
	}
	base := []string{"--root", root, "--work-dir", work, "--quiet"}
	if code, _, errs := tool(append([]string{"copy", "--apply", "--computer", "computer-a"}, base...)...); code != 0 {
		t.Fatalf("copy: exit %d %q", code, errs)
	}
	code, out, _ := tool(append([]string{"census"}, base...)...)
	var cen struct {
		Receipt string `json:"receipt"`
	}
	if err := json.Unmarshal([]byte(out), &cen); err != nil || code != 0 {
		t.Fatalf("census: %d %q", code, out)
	}
	exists := func(d string) bool {
		_, err := os.Stat(filepath.Join(root, "sha256", "computer-event-payload", d))
		return err == nil
	}
	del := func(extra ...string) (int, string) {
		args := append([]string{"delete", "--census-receipt", cen.Receipt, "--pause", "1ms", "--apply"}, base...)
		code, out, errs := tool(append(args, extra...)...)
		return code, out + errs
	}
	all := func() bool { return exists(digests[0]) && exists(digests[1]) && exists(digests[2]) }
	keepFile := func(name, content string) string {
		p := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}

	if code, _ := del(); code == 0 || !all() {
		t.Fatalf("delete --apply without --keep-digests: exit %d, files intact %v", code, all())
	}
	if code, _ := del("--keep-digests-empty-ok"); code == 0 || !all() {
		t.Fatalf("--keep-digests-empty-ok without a file: exit %d, files intact %v", code, all())
	}
	if code, msg := del("--keep-digests", keepFile("bad", digests[0]+"\nnot-a-digest\n")); code == 0 || !all() {
		t.Fatalf("a malformed keep file: exit %d, files intact %v: %s", code, all(), msg)
	}
	if code, _ := del("--keep-digests", keepFile("empty", "")); code == 0 || !all() {
		t.Fatalf("an empty keep file without the flag: exit %d, files intact %v", code, all())
	}
	if code, _, _ := tool(append([]string{"copy", "--keep-digests", keepFile("k", digests[0]+"\n")}, base...)...); code == 0 {
		t.Fatal("copy accepted --keep-digests")
	}

	// A listed digest survives; the others of the same computer go.
	code, msg := del("--keep-digests", keepFile("keep", digests[1]+"\n"))
	if code != 0 {
		t.Fatalf("delete with a keep file: exit %d %s", code, msg)
	}
	if exists(digests[0]) || !exists(digests[1]) || exists(digests[2]) {
		t.Fatalf("after the delete: %v %v %v, want only the listed digest left", exists(digests[0]), exists(digests[1]), exists(digests[2]))
	}
	var rep struct {
		KeptByList  int64 `json:"kept_by_list"`
		KeepDigests int64 `json:"keep_digests"`
	}
	if err := json.Unmarshal([]byte(strings.SplitN(msg, "\n", 2)[0]), &rep); err != nil || rep.KeptByList != 1 || rep.KeepDigests != 1 {
		t.Fatalf("report %+v (%v): %s", rep, err, msg)
	}
}
