package bundle

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// maxReceiptLine bounds one receipts.jsonl line (a signed head receipt is
// about 2 KiB).
const maxReceiptLine = 1 << 20

// ErrUnauthentic is wrapped by every verification failure: the bundle is
// damaged, incomplete, or not what a trusted host signed.
var ErrUnauthentic = errors.New("bundle: not authentic")

func unauthentic(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrUnauthentic, fmt.Sprintf(format, args...))
}

// ReadManifest reads and checks the shape of dir's manifest.json. Nothing in
// the manifest is signed: a field is trusted only once the signed tape
// confirms it, and an unconfirmed field (Genesis on a range that does not
// start at genesis) may only cause a refusal.
func ReadManifest(dir string) (Manifest, error) {
	raw, err := os.ReadFile(filepath.Join(dir, ManifestName))
	if err != nil {
		return Manifest{}, fmt.Errorf("bundle: read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Manifest{}, fmt.Errorf("bundle: decode manifest: %w", err)
	}
	if m.FormatVersion != 1 && m.FormatVersion != 2 {
		return Manifest{}, fmt.Errorf("bundle: format version %d is not 1 or 2", m.FormatVersion)
	}
	if !safeComputerID(m.ComputerID) || m.Tape.StartSeq < 0 || m.Tape.HeadSeq < m.Tape.StartSeq || !validDigest(m.Tape.StartHead) || !validDigest(m.Tape.HeadDigest) {
		return Manifest{}, fmt.Errorf("bundle: manifest is malformed")
	}
	if m.Tape.StartSeq == 0 && m.Tape.StartHead != computerevent.ZeroHead {
		return Manifest{}, fmt.Errorf("bundle: a range from genesis must start at the zero head")
	}
	if m.Genesis != "" && !validDigest(m.Genesis) {
		return Manifest{}, fmt.Errorf("bundle: manifest genesis is malformed")
	}
	if m.Tape.Head.ComputerID != m.ComputerID || int64(m.Tape.Head.Sequence) != m.Tape.HeadSeq || m.Tape.Head.CanonicalEventHead != m.Tape.HeadDigest {
		return Manifest{}, fmt.Errorf("bundle: manifest head row does not match the tape range")
	}
	if s := m.SnapshotRoot; s != nil {
		if s.ComputerID != m.ComputerID || !validDigest(s.Root) || s.HeadSequence < 0 || s.HeadSequence > m.Tape.HeadSeq || len(s.Chunks) == 0 {
			return Manifest{}, fmt.Errorf("bundle: manifest snapshot root is malformed")
		}
		if m.Tape.StartSeq > s.HeadSequence {
			return Manifest{}, fmt.Errorf("bundle: the range starts at %d, after its snapshot at %d: the events between are missing", m.Tape.StartSeq, s.HeadSequence)
		}
	} else if m.Tape.StartSeq != 0 && !m.Delta {
		return Manifest{}, fmt.Errorf("bundle: a bundle without a snapshot root must start at genesis or be a delta")
	}
	if m.Delta && m.SnapshotRoot != nil {
		return Manifest{}, fmt.Errorf("bundle: a delta carries no snapshot root")
	}
	return m, nil
}

// Verified is what VerifyBundle proved about a bundle.
type Verified struct {
	Manifest Manifest
	// ReceiptsSHA256 is the digest of the receipts.jsonl bytes verified.
	// Import re-reads the file and refuses when its bytes changed.
	ReceiptsSHA256 string
	// Signatures is the number of receipt signatures checked.
	Signatures int64
	// SnapshotNamedAt is the sequence of the snapshot_recorded event that
	// names the snapshot root (0 without a snapshot).
	SnapshotNamedAt int64
}

// Verify is VerifyBundle returning only the manifest.
func Verify(dir string, trust *TrustSet) (Manifest, error) {
	v, err := VerifyBundle(dir, trust)
	return v.Manifest, err
}

// VerifyBundle re-derives the bundle in dir and checks it is what a host in
// trust signed. Every failure wraps ErrUnauthentic (or is an I/O error).
//
// Integrity: every chunk, payload and pin receipt hashes to its name; every
// event is canonical, hashes to the digest its append receipt names, and
// chains by sequence and previous head from the manifest's start head to its
// head digest; every artifact ref an event names has its payload in the
// bundle, and every pin receipt names the artifact it pins; the counts match
// the manifest.
//
// Authenticity: every event head receipt and every pin receipt is signed by
// a key in trust, valid at the receipt's sequence and time, and the head
// receipt binds its event (computer, sequence, previous head, digest, kind,
// request commitment, pin receipts). The receipt is checked over its
// canonical bytes, whatever JSON spelling the host stored. The snapshot root
// is named by a snapshot_recorded event of the range after the snapshot's
// head; a range from genesis starts with genesis_imported; the head row is
// the last head receipt's projection.
//
// It streams: memory is the pack index (48 bytes an object) and one seen bit
// an object, plus a bounded pipeline (scan.go): every row is checked,
// signatures included, on GOMAXPROCS workers, and only the order-dependent
// checks run in sequence.
func VerifyBundle(dir string, trust *TrustSet) (Verified, error) {
	m, err := ReadManifest(dir)
	if err != nil {
		return Verified{}, fmt.Errorf("%w: %w", ErrUnauthentic, err)
	}
	v := Verified{Manifest: m}
	if trust.Len() == 0 {
		return v, unauthentic("no trusted keys: a bundle cannot be authenticated against an empty trust set")
	}
	objs, err := openObjects(dir, m.FormatVersion, false)
	if err != nil {
		return v, unauthentic("open tape objects: %v", err)
	}
	defer objs.close()
	var got Totals
	if m.SnapshotRoot != nil {
		seen := map[string]bool{}
		for _, digest := range m.SnapshotRoot.Chunks {
			if !validDigest(digest) {
				return v, unauthentic("invalid chunk digest %q", digest)
			}
			if seen[digest] {
				continue
			}
			seen[digest] = true
			if err := checkFile(dir, ChunksDir, digest); err != nil {
				return v, err
			}
			got.Chunks++
		}
	}
	scan, err := scanTape(dir, m, objs, trust)
	if err != nil {
		return v, err
	}
	got.Events, got.Payloads, got.PinReceipts, got.Receipts = scan.got.Events, scan.got.Payloads, scan.got.PinReceipts, scan.got.Receipts
	v.Signatures, v.SnapshotNamedAt, v.ReceiptsSHA256 = scan.signatures, scan.snapNamedAt, scan.receiptsHash
	if scan.end != m.Tape.HeadSeq || scan.endDigest != m.Tape.HeadDigest {
		return v, unauthentic("the tape ends at %d (%s), the manifest head is %d (%s)", scan.end, scan.endDigest, m.Tape.HeadSeq, m.Tape.HeadDigest)
	}
	if last := scan.last; last != nil {
		h := m.Tape.Head
		for field, want := range map[string]string{
			"desired_event_head": h.DesiredEventHead, "effective_event_head": h.EffectiveEventHead,
			"pending_transition_ref": h.PendingTransitionRef, "desired_state_commitment": h.DesiredStateCommitment,
			"effective_state_commitment": h.EffectiveStateCommitment,
		} {
			if fmt.Sprint(last.head.KindFields[field]) != want {
				return v, unauthentic("head row %s does not match the last head receipt", field)
			}
		}
		if h.ReducerVersion != last.reducer {
			return v, unauthentic("head row reducer version %d is not the last event's %d", h.ReducerVersion, last.reducer)
		}
		// The epoch counts key_revoked events from genesis.
		if (m.Tape.StartSeq == 0 && h.CredentialRevocationEpoch != scan.revocations) || h.CredentialRevocationEpoch < scan.revocations {
			return v, unauthentic("head row credential revocation epoch %d does not count the range's %d key revocations", h.CredentialRevocationEpoch, scan.revocations)
		}
	}
	if m.SnapshotRoot != nil && v.SnapshotNamedAt == 0 {
		return v, unauthentic("snapshot root %s is not named by a snapshot_recorded event of the tape after its head %d", m.SnapshotRoot.Root, m.SnapshotRoot.HeadSequence)
	}
	if got != m.Counts {
		return v, unauthentic("contents %+v do not match the manifest counts %+v", got, m.Counts)
	}
	return v, nil
}

func checkFile(dir, sub, digest string) error {
	ok, _, err := presentVerified(filepath.Join(dir, sub, digest), digest)
	if err != nil {
		return fmt.Errorf("bundle: %s/%s: %w", sub, digest, err)
	}
	if !ok {
		return unauthentic("%s/%s is missing or does not hash to its name", sub, digest)
	}
	return nil
}

func checkPin(objs objects, digest, computerID, artifact string) (computerevent.Receipt, error) {
	var receipt computerevent.Receipt
	raw, err := objs.get(PinsDir, digest)
	if err != nil {
		return receipt, fmt.Errorf("pin receipt %s: %w", digest, err)
	}
	if err := receipt.UnmarshalJSON(raw); err != nil {
		return receipt, fmt.Errorf("pin receipt %s: %w", digest, err)
	}
	if receipt.ReceiptKind != "PinReceipt" || receipt.Issuer != ReceiptIssuer || receipt.KindFields["computer_id"] != computerID || receipt.KindFields["artifact_digest"] != artifact {
		return receipt, fmt.Errorf("pin receipt %s does not pin %s", digest, artifact)
	}
	return receipt, nil
}

// headReceiptFields is the exact kind field set of an EventHeadReceipt
// (internal/platform eventHeadReceiptFields).
var headReceiptFields = []string{
	"computer_id", "previous_head", "event_digest", "sequence", "event_kind",
	"request_commitment", "pin_receipt_digests", "desired_event_head",
	"effective_event_head", "pending_transition_ref", "desired_state_commitment",
	"effective_state_commitment",
}

// seen is a set of tape objects: one bit a pack slot, or a map for a
// layout without slots (format 1).
type seen struct {
	objs objects
	bits []uint64
	m    map[string]bool
}

func newSeen(objs objects) *seen {
	if n := objs.slots(); n > 0 {
		return &seen{objs: objs, bits: make([]uint64, (n+63)/64)}
	}
	return &seen{objs: objs, m: map[string]bool{}}
}

func (s *seen) has(sub, digest string) bool {
	if s.bits != nil {
		i, ok := s.objs.slot(sub, digest)
		return ok && i >= 0 && s.bits[i/64]&(1<<(i%64)) != 0
	}
	return s.m[digest]
}

func (s *seen) add(sub, digest string) {
	if s.bits != nil {
		if i, ok := s.objs.slot(sub, digest); ok && i >= 0 {
			s.bits[i/64] |= 1 << (i % 64)
		}
		return
	}
	s.m[digest] = true
}
