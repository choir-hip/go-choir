package bundle

// Bundle authenticity: the failures verify must refuse, written before the
// verifier (docs/design/bundle-authenticity-2026-10-31.md section 2 is the
// full list with the import-side failures, which internal/platform
// bundle_drill_test.go drives against PostgreSQL).
//
//  A1  forged signature: a head receipt whose signature bytes are not the
//      key's signature over its canonical payload.
//  A2  wrong key: a well-formed bundle signed by a key the importer does not
//      trust (an attacker's own host, or a host never enrolled).
//  A3  key id spoof: receipts naming a trusted key id, signed by another key.
//  A4  retired key used after its cutoff (sequence or time); a planned
//      rotation (old key before the cutoff, new key after) still verifies.
//  A5  snapshot root the tape does not name: no snapshot_recorded event with
//      idempotency key "snapshot:<root>" after the snapshot's head.
//  A6  truncated chain: receipts end before the manifest head.
//  A7  spliced foreign chain: another computer's receipts, or a fork of the
//      same computer, spliced after a prefix.
//  A8  genesis mismatch: the manifest's genesis is not the tape's event 1, or
//      event 1 is not genesis_imported.
//  A9  tampered payload with recomputed digests: payload, artifact ref,
//      event, pin receipts and head receipts all rehashed and re-signed by a
//      key the importer does not trust, or signed under a trusted key id.
//  A10 head receipt field tampered and its canonical payload digest
//      recomputed: the signature no longer matches.
//  A11 an empty trust set: nothing can be authenticated.
//  A12 a head receipt stored in another JSON spelling (Node B's stored rows
//      are not the canonical bytes): it must still verify, over the
//      canonical bytes.

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func mustVerify(t *testing.T, dir string, trust *TrustSet) Verified {
	t.Helper()
	v, err := VerifyBundle(dir, trust)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	return v
}

func mustNotVerify(t *testing.T, what, dir string, trust *TrustSet, want string) {
	t.Helper()
	_, err := VerifyBundle(dir, trust)
	if err == nil {
		t.Fatalf("%s: verify accepted the bundle", what)
	}
	if !errors.Is(err, ErrUnauthentic) {
		t.Fatalf("%s: error %v does not wrap ErrUnauthentic", what, err)
	}
	if want != "" && !strings.Contains(err.Error(), want) {
		t.Fatalf("%s: error %q does not say %q", what, err, want)
	}
	t.Logf("%s: %v", what, err)
}

// rewriteReceipts applies edit to every receipts.jsonl row.
func rewriteReceipts(t *testing.T, dir string, edit func(row *ReceiptRow) (keep bool)) {
	t.Helper()
	p := filepath.Join(dir, ReceiptsName)
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 1<<16), 1<<20)
	for sc.Scan() {
		var row ReceiptRow
		if err := json.Unmarshal(sc.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		if edit(&row) {
			_ = enc.Encode(row)
		}
	}
	if err := os.WriteFile(p, out.Bytes(), 0o640); err != nil {
		t.Fatal(err)
	}
}

func editReceipt(t *testing.T, row *ReceiptRow, edit func(r *computerevent.Receipt)) {
	t.Helper()
	var r computerevent.Receipt
	if err := r.UnmarshalJSON([]byte(row.EventHeadReceiptJSON)); err != nil {
		t.Fatal(err)
	}
	edit(&r)
	b, err := r.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	row.EventHeadReceiptJSON = string(b)
}

func TestBundleAuthenticity(t *testing.T) {
	keyA, keyB := newSigningKey(t), newSigningKey(t)
	trustA := trustOf(t, keyA)
	honest := synth{Events: 12, Key: keyA, Tag: "honest"}.write(t, filepath.Join(t.TempDir(), "honest"))
	v := mustVerify(t, honest.Dir, trustA)
	if v.Signatures != honest.Manifest.Counts.Receipts+honest.Manifest.Counts.PinReceipts {
		t.Fatalf("signatures checked = %d, want %d", v.Signatures, honest.Manifest.Counts.Receipts+honest.Manifest.Counts.PinReceipts)
	}
	copyOf := func(src string) string {
		dst := filepath.Join(t.TempDir(), "copy")
		copyDir(t, src, dst)
		return dst
	}

	t.Run("A1 forged signature", func(t *testing.T) {
		dir := copyOf(honest.Dir)
		rewriteReceipts(t, dir, func(row *ReceiptRow) bool {
			if row.Sequence == 5 {
				editReceipt(t, row, func(r *computerevent.Receipt) {
					sig, _ := base64.RawStdEncoding.DecodeString(r.SignatureSet[0].Signature)
					sig[7] ^= 0x01
					r.SignatureSet[0].Signature = base64.RawStdEncoding.EncodeToString(sig)
				})
			}
			return true
		})
		mustNotVerify(t, "forged signature", dir, trustA, "invalid signature")
	})

	t.Run("A2 wrong key", func(t *testing.T) {
		other := synth{Events: 12, Key: keyB, Tag: "other"}.write(t, filepath.Join(t.TempDir(), "b"))
		mustNotVerify(t, "untrusted signer", other.Dir, trustA, "not trusted")
		mustVerify(t, other.Dir, trustOf(t, keyB))
	})

	t.Run("A3 key id spoof", func(t *testing.T) {
		spoof := keyB
		spoof.KeyID = keyA.KeyID
		dir := synth{Events: 6, Key: spoof, Tag: "spoof"}.write(t, filepath.Join(t.TempDir(), "spoof")).Dir
		mustNotVerify(t, "trusted key id, other key", dir, trustA, "invalid signature")
		// A trust entry cannot rebind a key id to another key either.
		bad := TrustEntry(pubOf(keyB), "spoof")
		bad.KeyID = keyA.KeyID
		if _, err := NewTrustSet(bad); err == nil {
			t.Fatal("a trust entry with another key's id was accepted")
		}
	})

	t.Run("A4 retired key", func(t *testing.T) {
		cut := uint64(7)
		retired := TrustEntry(pubOf(keyA), "retired")
		retired.ComputerID, retired.FirstInvalidSequence = "computer-synth", &cut
		trustRetired, err := NewTrustSet(retired, TrustEntry(pubOf(keyB), "new"))
		if err != nil {
			t.Fatal(err)
		}
		mustNotVerify(t, "old key after its sequence cutoff", honest.Dir, trustRetired, "retired from sequence 7")
		rotated := synth{Events: 12, Tag: "rotated", KeyFor: func(seq int64) computerevent.SigningKey {
			if seq < int64(cut) {
				return keyA
			}
			return keyB
		}}.write(t, filepath.Join(t.TempDir(), "rotated"))
		mustVerify(t, rotated.Dir, trustRetired)
		// The pin receipts carry no sequence: they are held to their event's.
		late := synth{Events: 12, Tag: "late", Key: keyB, PinKeyFor: func(seq int64) computerevent.SigningKey {
			if seq < int64(cut)+2 {
				return keyA
			}
			return keyB
		}}.write(t, filepath.Join(t.TempDir(), "late"))
		mustNotVerify(t, "pin receipts by the old key past the cutoff", late.Dir, trustRetired, "pin receipt")
		// A time cutoff.
		at := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC).Add(5 * time.Millisecond)
		timed := TrustEntry(pubOf(keyA), "timed")
		timed.FirstInvalidTime = &at
		mustNotVerify(t, "old key after its time cutoff", honest.Dir, mustTrust(t, timed), "retired from")
		// A sequence cutoff without a computer is refused at load.
		unscoped := TrustEntry(pubOf(keyA), "unscoped")
		unscoped.FirstInvalidSequence = &cut
		if _, err := NewTrustSet(unscoped); err == nil {
			t.Fatal("a sequence cutoff without a computer id was accepted")
		}
	})

	t.Run("A5 snapshot the tape does not name", func(t *testing.T) {
		named := synth{Events: 12, Start: 5, SnapshotAt: 5, NameSnapshot: true, Key: keyA, Tag: "named"}.write(t, filepath.Join(t.TempDir(), "named"))
		if v := mustVerify(t, named.Dir, trustA); v.SnapshotNamedAt != 6 {
			t.Fatalf("snapshot named at %d, want 6", v.SnapshotNamedAt)
		}
		unnamed := synth{Events: 12, Start: 5, SnapshotAt: 5, NameSnapshot: false, Key: keyA, Tag: "unnamed"}.write(t, filepath.Join(t.TempDir(), "unnamed"))
		mustNotVerify(t, "snapshot root no event names", unnamed.Dir, trustA, "not named")
		// Another root swapped into a named bundle's manifest.
		dir := copyOf(named.Dir)
		m := named.Manifest
		snap := *m.SnapshotRoot
		snap.Root = digestOf('c')
		m.SnapshotRoot = &snap
		writeManifest(t, dir, m)
		mustNotVerify(t, "swapped snapshot root", dir, trustA, "not named")
		// A snapshot claimed at a head after its naming event.
		dir = copyOf(named.Dir)
		m = named.Manifest
		snap = *m.SnapshotRoot
		snap.HeadSequence = 8
		m.SnapshotRoot = &snap
		writeManifest(t, dir, m)
		mustNotVerify(t, "snapshot head after its naming event", dir, trustA, "not named")
	})

	t.Run("A6 truncated chain", func(t *testing.T) {
		dir := copyOf(honest.Dir)
		rewriteReceipts(t, dir, func(row *ReceiptRow) bool { return row.Sequence <= 9 })
		mustNotVerify(t, "receipts end before the head", dir, trustA, "the tape ends at 9")
		dir = copyOf(honest.Dir)
		rewriteReceipts(t, dir, func(row *ReceiptRow) bool { return row.Sequence != 4 })
		mustNotVerify(t, "a receipt missing in the middle", dir, trustA, "out of sequence")
	})

	t.Run("A7 spliced foreign chain", func(t *testing.T) {
		foreign := synth{ComputerID: "computer-foreign", Events: 12, Key: keyA, Tag: "foreign"}.write(t, filepath.Join(t.TempDir(), "foreign"))
		fork := synth{Events: 12, Key: keyA, Tag: "fork"}.write(t, filepath.Join(t.TempDir(), "fork"))
		for name, src := range map[string]synthResult{"another computer": foreign, "a fork of the same computer": fork} {
			dir := copyOf(honest.Dir)
			var tail []ReceiptRow
			rewriteReceipts(t, src.Dir, func(row *ReceiptRow) bool {
				if row.Sequence > 6 {
					tail = append(tail, *row)
				}
				return true
			})
			rewriteReceipts(t, dir, func(row *ReceiptRow) bool { return row.Sequence <= 6 })
			f, _ := os.OpenFile(filepath.Join(dir, ReceiptsName), os.O_APPEND|os.O_WRONLY, 0)
			enc := json.NewEncoder(f)
			for _, r := range tail {
				_ = enc.Encode(r)
			}
			_ = f.Close()
			// The spliced tail's objects too, so only the chain can refuse.
			mergePack(t, dir, src.Dir)
			m := honest.Manifest
			m.Tape.Head, m.Tape.HeadDigest = src.Manifest.Tape.Head, src.Manifest.Tape.HeadDigest
			m.Tape.Head.ComputerID = m.ComputerID
			writeManifest(t, dir, m)
			mustNotVerify(t, name, dir, trustA, "")
		}
	})

	t.Run("A8 genesis mismatch", func(t *testing.T) {
		dir := copyOf(honest.Dir)
		m := honest.Manifest
		m.Genesis = digestOf('d')
		writeManifest(t, dir, m)
		mustNotVerify(t, "manifest genesis is not event 1", dir, trustA, "genesis")
	})

	t.Run("A9 tampered payload, digests recomputed and re-signed", func(t *testing.T) {
		tamper := func(seq int64) []byte {
			if seq == 4 {
				return []byte("the attacker's payload")
			}
			return []byte("synth honest payload " + string(rune('0'+seq)))
		}
		// Re-signed by the attacker's own key from the tampered event on.
		resigned := synth{Events: 12, Tag: "honest", PayloadFor: tamper, KeyFor: func(seq int64) computerevent.SigningKey {
			if seq >= 4 {
				return keyB
			}
			return keyA
		}}.write(t, filepath.Join(t.TempDir(), "resigned"))
		mustNotVerify(t, "re-signed by an untrusted key", resigned.Dir, trustA, "not trusted")
		// Re-signed under the trusted key id.
		spoof := keyB
		spoof.KeyID = keyA.KeyID
		spoofed := synth{Events: 12, Tag: "honest", PayloadFor: tamper, KeyFor: func(seq int64) computerevent.SigningKey {
			if seq >= 4 {
				return spoof
			}
			return keyA
		}}.write(t, filepath.Join(t.TempDir(), "spoofed"))
		mustNotVerify(t, "re-signed under the trusted key id", spoofed.Dir, trustA, "invalid signature")
	})

	t.Run("A10 receipt field tampered, payload digest recomputed", func(t *testing.T) {
		dir := copyOf(honest.Dir)
		rewriteReceipts(t, dir, func(row *ReceiptRow) bool {
			if row.Sequence == 12 {
				editReceipt(t, row, func(r *computerevent.Receipt) {
					r.KindFields["effective_state_commitment"] = digestOf('e')
					r.CanonicalPayloadSHA256 = recomputePayloadDigest(t, *r)
				})
			}
			return true
		})
		m := honest.Manifest
		m.Tape.Head.EffectiveStateCommitment = digestOf('e')
		writeManifest(t, dir, m)
		mustNotVerify(t, "head projection rewritten", dir, trustA, "invalid signature")
	})

	t.Run("A11 empty trust set", func(t *testing.T) {
		mustNotVerify(t, "no trusted keys", honest.Dir, &TrustSet{}, "no trusted keys")
		mustNotVerify(t, "nil trust set", honest.Dir, nil, "no trusted keys")
	})

	t.Run("A12 stored JSON spelling", func(t *testing.T) {
		dir := copyOf(honest.Dir)
		rewriteReceipts(t, dir, func(row *ReceiptRow) bool {
			var fields map[string]any
			_ = json.Unmarshal([]byte(row.EventHeadReceiptJSON), &fields)
			spaced, _ := json.MarshalIndent(fields, "", " ")
			row.EventHeadReceiptJSON = string(spaced)
			return true
		})
		mustVerify(t, dir, trustA)
	})
}

func pubOf(k computerevent.SigningKey) ed25519.PublicKey {
	return k.PrivateKey.Public().(ed25519.PublicKey)
}

func mustTrust(t *testing.T, entries ...TrustedKey) *TrustSet {
	t.Helper()
	set, err := NewTrustSet(entries...)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

// recomputePayloadDigest is the canonical payload digest of r as the
// receipt package computes it (canonical bytes without the attestations).
func recomputePayloadDigest(t *testing.T, r computerevent.Receipt) string {
	t.Helper()
	fields := map[string]any{}
	for k, v := range r.KindFields {
		fields[k] = v
	}
	fields["receipt_version"], fields["receipt_kind"], fields["receipt_id"] = r.ReceiptVersion, r.ReceiptKind, r.ReceiptID
	fields["issuer"], fields["issued_at"], fields["required_signers"] = r.Issuer, r.IssuedAt, r.RequiredSigners
	b, err := computerevent.CanonicalJSON(fields)
	if err != nil {
		t.Fatal(err)
	}
	return computerevent.DigestBytes(b)
}

func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o640)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// mergePack appends src's pack objects to dir's pack.
func mergePack(t *testing.T, dir, src string) {
	t.Helper()
	from, err := openPackIndex(src)
	if err != nil {
		t.Fatal(err)
	}
	defer from.close()
	to, err := openPack(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, sub := range packKinds {
		ds, _ := from.digests(sub)
		for _, d := range ds {
			raw, err := from.get(sub, d)
			if err != nil {
				t.Fatal(err)
			}
			if err := to.put(sub, d, raw); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := to.close(); err != nil {
		t.Fatal(err)
	}
}

// TestBundleVerifyCounts: the head row's credential revocation epoch is
// bound to the key_revoked events of a range from genesis.
func TestBundleVerifyCounts(t *testing.T) {
	key := newSigningKey(t)
	r := synth{Events: 300, Key: key, RevokeEvery: 50, Tag: "counts"}.write(t, filepath.Join(t.TempDir(), "b"))
	v := mustVerify(t, r.Dir, trustOf(t, key))
	if v.Manifest.Tape.Head.CredentialRevocationEpoch != 6 {
		t.Fatalf("epoch = %d", v.Manifest.Tape.Head.CredentialRevocationEpoch)
	}
	// The epoch is bound: a head row claiming fewer revocations is refused.
	dir := filepath.Join(t.TempDir(), "epoch")
	copyDir(t, r.Dir, dir)
	m := r.Manifest
	m.Tape.Head.CredentialRevocationEpoch = 2
	writeManifest(t, dir, m)
	mustNotVerify(t, "head row epoch", dir, trustOf(t, key), "revocation epoch")
}
