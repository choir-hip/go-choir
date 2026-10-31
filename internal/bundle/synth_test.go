package bundle

import (
	"bufio"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// synth writes a bundle of a synthetic but production-shaped tape: real
// canonical events reduced by computerevent.Reduce, pin receipts and event
// head receipts signed exactly as internal/platform signs them
// (event_artifacts.go pin, computer_events.go eventHeadReceiptFields). It
// needs no database, so the authenticity tests and the cost measurement run
// anywhere.
type synth struct {
	ComputerID string
	Events     int
	// Start is the range start (0: from genesis).
	Start int64
	// SnapshotAt records a snapshot root at this head and names it by a
	// snapshot_recorded event right after (0: none). NameSnapshot false
	// leaves it unnamed.
	SnapshotAt   int64
	NameSnapshot bool
	// KeyFor signs the receipts of a sequence (default: Key).
	Key    computerevent.SigningKey
	KeyFor func(seq int64) computerevent.SigningKey
	// PinKeyFor signs the pin receipts of a sequence (default: KeyFor).
	PinKeyFor func(seq int64) computerevent.SigningKey
	// PayloadFor varies the payload of a sequence (default: derived from
	// the sequence and Tag).
	Tag        string
	PayloadFor func(seq int64) []byte
	// RevokeEvery appends a key_revoked event every n events (0: never).
	RevokeEvery int64
}

type synthResult struct {
	Dir      string
	Manifest Manifest
	Digests  []string // event digest by sequence (index 0 unused)
	Root     string
}

func newSigningKey(t testing.TB) computerevent.SigningKey {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: SignerDomain, KeyID: KeyIDFor(pub)}, PrivateKey: priv}
}

func trustOf(t testing.TB, keys ...computerevent.SigningKey) *TrustSet {
	t.Helper()
	set, err := NewTrustSet()
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range keys {
		if err := set.Add(TrustEntry(k.PrivateKey.Public().(ed25519.PublicKey), "test")); err != nil {
			t.Fatal(err)
		}
	}
	return set
}

func digestOf(b byte) string {
	out := make([]byte, 64)
	for i := range out {
		out[i] = "0123456789abcdef"[b%16]
	}
	return string(out)
}

func (s synth) write(t testing.TB, dir string) synthResult {
	t.Helper()
	if s.ComputerID == "" {
		s.ComputerID = "computer-synth"
	}
	keyFor := s.KeyFor
	if keyFor == nil {
		keyFor = func(int64) computerevent.SigningKey { return s.Key }
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	objs, err := openPack(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	rf, err := os.Create(filepath.Join(dir, ReceiptsName))
	if err != nil {
		t.Fatal(err)
	}
	bw := bufio.NewWriterSize(rf, 1<<20)
	enc := json.NewEncoder(bw)
	res := synthResult{Dir: dir, Digests: make([]string, s.Events+1)}
	m := Manifest{FormatVersion: FormatVersion, ComputerID: s.ComputerID, Producer: Producer{Host: "synth", Tool: "test"}}
	m.Tape.StartSeq = s.Start
	m.Tape.StartHead = computerevent.ZeroHead
	var head *computerevent.Head
	base := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	for seq := int64(1); seq <= int64(s.Events); seq++ {
		key := keyFor(seq)
		pinKey := key
		if s.PinKeyFor != nil {
			pinKey = s.PinKeyFor(seq)
		}
		inRange := seq > s.Start
		eventID, err := computerevent.NewEventID()
		if err != nil {
			t.Fatal(err)
		}
		kind := computerevent.EventMessageRecorded
		idem := fmt.Sprintf("synth-%d-%s", seq, eventID)
		switch {
		case seq == 1:
			kind = computerevent.EventGenesisImported
		case s.SnapshotAt > 0 && seq == s.SnapshotAt+1 && s.NameSnapshot:
			kind = computerevent.EventSnapshotRecorded
			idem = "snapshot:" + res.Root
		case s.RevokeEvery > 0 && seq%s.RevokeEvery == 0:
			kind = computerevent.EventKeyRevoked
		case seq%3 == 0:
			kind = computerevent.EventToolInvoked
		}
		event := computerevent.Event{
			SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: s.ComputerID,
			Sequence: uint64(seq), PreviousHead: computerevent.ZeroHead, EventKind: kind,
			OccurredAt:     base.Add(time.Duration(seq) * time.Millisecond).Format(time.RFC3339Nano),
			IdempotencyKey: idem, ActorProfile: "trusted-core", AuthorityRef: "authority:synth",
			PayloadCommitment: digestOf('a'), PrivacyClass: "public", ReducerVersion: computerevent.ReducerVersionV1,
			ExpectedDesiredEventHead: computerevent.ZeroHead, ExpectedEffectiveEventHead: computerevent.ZeroHead,
			ExpectedDesiredStateCommitment: computerevent.ZeroHead, ExpectedEffectiveStateCommitment: computerevent.ZeroHead,
		}
		input := computerevent.TransitionInput{}
		if head == nil {
			event.ResultingEffectiveCommitment = digestOf('b')
			input.TargetStateCommitment = digestOf('b')
		} else {
			event.PreviousHead = head.CanonicalEventHead
			event.ExpectedDesiredEventHead, event.ExpectedEffectiveEventHead = head.DesiredEventHead, head.EffectiveEventHead
			event.ExpectedPendingTransitionRef = head.PendingTransitionRef
			event.ExpectedDesiredStateCommitment, event.ExpectedEffectiveStateCommitment = head.DesiredStateCommitment, head.EffectiveStateCommitment
		}
		var payload []byte
		if kind == computerevent.EventMessageRecorded || kind == computerevent.EventSnapshotRecorded {
			if s.PayloadFor != nil {
				payload = s.PayloadFor(seq)
			} else {
				payload = []byte(fmt.Sprintf("synth %s payload %d", s.Tag, seq))
			}
			d := computerevent.DigestBytes(payload)
			event.OutputArtifactRefs = []string{"artifact:sha256:" + d}
			event.PayloadCommitment = d
		}
		event.RequestCommitment = computerevent.ZeroHead
		pinIntent, err := computerevent.ComputePinIntentCommitment(event, input)
		if err != nil {
			t.Fatal(err)
		}
		pin := func(raw []byte, namespace, field, commitment string) (string, []byte) {
			r, err := computerevent.NewSignedReceipt("PinReceipt", ReceiptIssuer, map[string]any{
				"computer_id": s.ComputerID, "artifact_digest": computerevent.DigestBytes(raw), "media_type": "application/octet-stream",
				"length": len(raw), "privacy_class": "public", "pin_namespace": namespace, field: commitment,
			}, []computerevent.SigningKey{pinKey}, base.Add(time.Duration(seq)*time.Millisecond))
			if err != nil {
				t.Fatal(err)
			}
			b, _ := r.CanonicalBytes()
			return computerevent.DigestBytes(b), b
		}
		var payloadPins []string
		var payloadPinBytes [][]byte
		if payload != nil {
			d, b := pin(payload, "computer-event-payload", "pin_intent_commitment", pinIntent)
			payloadPins, payloadPinBytes = append(payloadPins, d), append(payloadPinBytes, b)
		}
		if event.RequestCommitment, err = computerevent.ComputeRequestCommitment(event, input, pinIntent, payloadPins); err != nil {
			t.Fatal(err)
		}
		eventJSON, err := event.CanonicalBytes()
		if err != nil {
			t.Fatal(err)
		}
		eventDigest := computerevent.DigestBytes(eventJSON)
		eventPin, eventPinBytes := pin(eventJSON, "computer-event", "request_commitment", event.RequestCommitment)
		next, err := computerevent.Reduce(head, event, input)
		if err != nil {
			t.Fatalf("reduce %d: %v", seq, err)
		}
		res.Digests[seq] = eventDigest
		if s.SnapshotAt > 0 && seq == s.SnapshotAt {
			chunk := []byte(fmt.Sprintf("chunk of %s at %d", s.ComputerID, seq))
			cd := computerevent.DigestBytes(chunk)
			if err := writeAtomic(filepath.Join(dir, ChunksDir, cd), chunk, 0o640); err != nil {
				t.Fatal(err)
			}
			res.Root = computerevent.DigestBytes([]byte("root " + cd))
			m.SnapshotRoot = &SnapshotRoot{ComputerID: s.ComputerID, Root: res.Root, HeadSequence: seq, Chunks: []string{cd}, CreatedAt: base}
			m.Counts.Chunks = 1
		}
		if seq == s.Start {
			m.Tape.StartHead = eventDigest
		}
		if seq == 1 {
			m.Genesis = eventDigest
		}
		if inRange {
			put := func(sub, digest string, raw []byte) {
				if _, ok := objs.size(sub, digest); ok {
					return
				}
				if err := objs.put(sub, digest, raw); err != nil {
					t.Fatal(err)
				}
				switch sub {
				case EventsDir:
					m.Counts.Events++
				case PayloadsDir:
					m.Counts.Payloads++
				case PinsDir:
					m.Counts.PinReceipts++
				}
			}
			put(EventsDir, eventDigest, eventJSON)
			if payload != nil {
				put(PayloadsDir, event.PayloadCommitment, payload)
			}
			put(PinsDir, eventPin, eventPinBytes)
			for i, d := range payloadPins {
				put(PinsDir, d, payloadPinBytes[i])
			}
			receipt, err := computerevent.NewSignedReceipt("EventHeadReceipt", ReceiptIssuer, map[string]any{
				"computer_id": s.ComputerID, "previous_head": event.PreviousHead, "event_digest": eventDigest,
				"sequence": event.Sequence, "event_kind": event.EventKind, "request_commitment": event.RequestCommitment,
				"pin_receipt_digests": append([]string{eventPin}, payloadPins...),
				"desired_event_head":  next.DesiredEventHead, "effective_event_head": next.EffectiveEventHead,
				"pending_transition_ref": next.PendingTransitionRef, "desired_state_commitment": next.DesiredStateCommitment,
				"effective_state_commitment": next.EffectiveStateCommitment,
			}, []computerevent.SigningKey{key}, base.Add(time.Duration(seq)*time.Millisecond))
			if err != nil {
				t.Fatal(err)
			}
			rj, _ := receipt.CanonicalBytes()
			if err := enc.Encode(ReceiptRow{
				ComputerID: s.ComputerID, IdempotencyKey: event.IdempotencyKey, RequestCommitment: event.RequestCommitment,
				Sequence: seq, EventKind: string(event.EventKind), EventDigest: eventDigest, EventPinReceiptDigest: eventPin,
				EventHeadReceiptID: receipt.ReceiptID, EventHeadReceiptJSON: string(rj), CreatedAt: base.Add(time.Duration(seq) * time.Millisecond),
			}); err != nil {
				t.Fatal(err)
			}
			m.Counts.Receipts++
		}
		head = &next
	}
	if err := bw.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := rf.Close(); err != nil {
		t.Fatal(err)
	}
	if err := objs.close(); err != nil {
		t.Fatal(err)
	}
	m.Tape.HeadSeq = int64(head.Sequence)
	m.Tape.HeadDigest = head.CanonicalEventHead
	m.Tape.Head = *head
	m.CreatedAt = base
	res.Manifest = m
	writeManifest(t, dir, m)
	return res
}

func writeManifest(t testing.TB, dir string, m Manifest) {
	t.Helper()
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := writeAtomic(filepath.Join(dir, ManifestName), append(raw, '\n'), 0o640); err != nil {
		t.Fatal(err)
	}
}
