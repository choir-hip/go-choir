package bundle

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// maxReceiptLine bounds one receipts.jsonl line (a signed head receipt is
// about 2 KiB).
const maxReceiptLine = 1 << 20

// ReadManifest reads and checks the shape of dir's manifest.json.
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
	if m.Tape.Head.ComputerID != m.ComputerID || int64(m.Tape.Head.Sequence) != m.Tape.HeadSeq || m.Tape.Head.CanonicalEventHead != m.Tape.HeadDigest {
		return Manifest{}, fmt.Errorf("bundle: manifest head row does not match the tape range")
	}
	if s := m.SnapshotRoot; s != nil {
		if s.ComputerID != m.ComputerID || !validDigest(s.Root) || s.HeadSequence < 0 || s.HeadSequence > m.Tape.HeadSeq || len(s.Chunks) == 0 {
			return Manifest{}, fmt.Errorf("bundle: manifest snapshot root is malformed")
		}
	} else if m.Tape.StartSeq != 0 {
		return Manifest{}, fmt.Errorf("bundle: a bundle without a snapshot root must start at genesis")
	}
	return m, nil
}

// Verify re-derives the bundle in dir: every chunk, payload and pin receipt
// hashes to its name; every event is canonical, hashes to the digest its
// append receipt names, and chains by sequence and previous head from the
// manifest's start head to its head digest; every artifact ref an event names
// has its payload in the bundle, and every pin receipt names the artifact it
// pins; the last append receipt's projection matches the head row; the
// counts match the manifest. It returns the manifest.
func Verify(dir string) (Manifest, error) {
	m, err := ReadManifest(dir)
	if err != nil {
		return Manifest{}, err
	}
	objs, err := openObjects(dir, m.FormatVersion, false)
	if err != nil {
		return m, fmt.Errorf("bundle: open tape objects: %w", err)
	}
	defer objs.close()
	var got Totals
	if m.SnapshotRoot != nil {
		seen := map[string]bool{}
		for _, digest := range m.SnapshotRoot.Chunks {
			if !validDigest(digest) {
				return m, fmt.Errorf("bundle: invalid chunk digest %q", digest)
			}
			if seen[digest] {
				continue
			}
			seen[digest] = true
			if err := checkFile(dir, ChunksDir, digest); err != nil {
				return m, err
			}
			got.Chunks++
		}
	}
	f, err := os.Open(filepath.Join(dir, ReceiptsName))
	if err != nil {
		return m, fmt.Errorf("bundle: open receipts: %w", err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64<<10), maxReceiptLine)
	prev, seq := m.Tape.StartHead, m.Tape.StartSeq
	seenPayload, seenPin := map[string]bool{}, map[string]bool{}
	var last *computerevent.Receipt
	for scanner.Scan() {
		var row ReceiptRow
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			return m, fmt.Errorf("bundle: receipt line after %d: %w", seq, err)
		}
		if row.ComputerID != m.ComputerID || row.Sequence != seq+1 {
			return m, fmt.Errorf("bundle: receipt after %d is out of sequence or scope (sequence %d)", seq, row.Sequence)
		}
		if !validDigest(row.EventDigest) {
			return m, fmt.Errorf("bundle: receipt %d has an invalid event digest", row.Sequence)
		}
		raw, err := objs.get(EventsDir, row.EventDigest)
		if err != nil {
			return m, fmt.Errorf("bundle: event %d: %w", row.Sequence, err)
		}
		event, err := decodeEvent(raw, row.EventDigest)
		if err != nil {
			return m, err
		}
		if event.ComputerID != m.ComputerID || int64(event.Sequence) != row.Sequence || string(event.EventKind) != row.EventKind || event.IdempotencyKey != row.IdempotencyKey || event.RequestCommitment != row.RequestCommitment {
			return m, fmt.Errorf("bundle: event %d does not match its append receipt", row.Sequence)
		}
		if event.PreviousHead != prev {
			return m, fmt.Errorf("bundle: event %d does not chain to %s", row.Sequence, prev)
		}
		got.Events++
		receipt, pins, err := headReceipt(row)
		if err != nil {
			return m, err
		}
		if receipt.ReceiptKind != "EventHeadReceipt" || receipt.ReceiptID != row.EventHeadReceiptID ||
			receipt.KindFields["computer_id"] != m.ComputerID || receipt.KindFields["event_digest"] != row.EventDigest ||
			receipt.KindFields["previous_head"] != prev || fmt.Sprint(receipt.KindFields["sequence"]) != fmt.Sprint(row.Sequence) ||
			receipt.KindFields["request_commitment"] != row.RequestCommitment || receipt.KindFields["event_kind"] != row.EventKind {
			return m, fmt.Errorf("bundle: head receipt %d does not bind its event", row.Sequence)
		}
		payloads, err := eventPayloadDigests(event)
		if err != nil {
			return m, err
		}
		if len(pins) != 1+len(payloads) {
			return m, fmt.Errorf("bundle: event %d names %d payloads and %d payload pins", row.Sequence, len(payloads), len(pins)-1)
		}
		for _, digest := range payloads {
			if seenPayload[digest] {
				continue
			}
			seenPayload[digest] = true
			if _, err := objs.get(PayloadsDir, digest); err != nil {
				return m, err
			}
			got.Payloads++
		}
		for i, digest := range pins {
			want := row.EventDigest
			if i > 0 {
				want = payloads[i-1]
			}
			if err := checkPin(objs, digest, m.ComputerID, want); err != nil {
				return m, fmt.Errorf("bundle: event %d: %w", row.Sequence, err)
			}
			if !seenPin[digest] {
				seenPin[digest] = true
				got.PinReceipts++
			}
		}
		got.Receipts++
		prev, seq = row.EventDigest, row.Sequence
		r := receipt
		last = &r
	}
	if err := scanner.Err(); err != nil {
		return m, fmt.Errorf("bundle: read receipts: %w", err)
	}
	if seq != m.Tape.HeadSeq || prev != m.Tape.HeadDigest {
		return m, fmt.Errorf("bundle: the tape ends at %d (%s), the manifest head is %d (%s)", seq, prev, m.Tape.HeadSeq, m.Tape.HeadDigest)
	}
	if last != nil {
		h := m.Tape.Head
		for field, want := range map[string]string{
			"desired_event_head": h.DesiredEventHead, "effective_event_head": h.EffectiveEventHead,
			"pending_transition_ref": h.PendingTransitionRef, "desired_state_commitment": h.DesiredStateCommitment,
			"effective_state_commitment": h.EffectiveStateCommitment,
		} {
			if fmt.Sprint(last.KindFields[field]) != want {
				return m, fmt.Errorf("bundle: head row %s does not match the last head receipt", field)
			}
		}
	}
	if got != m.Counts {
		return m, fmt.Errorf("bundle: contents %+v do not match the manifest counts %+v", got, m.Counts)
	}
	return m, nil
}

func checkFile(dir, sub, digest string) error {
	ok, _, err := presentVerified(filepath.Join(dir, sub, digest), digest)
	if err != nil {
		return fmt.Errorf("bundle: %s/%s: %w", sub, digest, err)
	}
	if !ok {
		return fmt.Errorf("bundle: %s/%s is missing or does not hash to its name", sub, digest)
	}
	return nil
}

func checkPin(objs objects, digest, computerID, artifact string) error {
	raw, err := objs.get(PinsDir, digest)
	if err != nil {
		return fmt.Errorf("pin receipt %s: %w", digest, err)
	}
	var receipt computerevent.Receipt
	if err := receipt.UnmarshalJSON(raw); err != nil {
		return fmt.Errorf("pin receipt %s: %w", digest, err)
	}
	if receipt.ReceiptKind != "PinReceipt" || receipt.KindFields["computer_id"] != computerID || receipt.KindFields["artifact_digest"] != artifact {
		return fmt.Errorf("pin receipt %s does not pin %s", digest, artifact)
	}
	return nil
}
