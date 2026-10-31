package platform

// Computer bundle (internal/bundle, cmd/choir-bundle): failure modes, written
// before the test code. A bundle is a snapshot root, its chunks and the tape
// from the snapshot head to the head; export (S3) reads only PostgreSQL rows
// and sealed host files.
//
// Export and verify (S3):
//  E1  Export reads the wrong range: events at or before the snapshot head,
//      or a range that stops short of the head row read at the start.
//  E2  Export misses a store layout: an event or payload that lives in a
//      tape segment (PLATFORM_TAPE_SEGMENTS=write) instead of a legacy file
//      is not found, or the bundle copies segment files and so depends on
//      the host layout.
//  E3  Export misses an artifact: a payload an event names, a pin receipt an
//      append receipt names, or a chunk the snapshot root names.
//  E4  A re-export to the same directory rewrites content-addressed files
//      (not incremental), or leaves a stale manifest that names an old head.
//  E5  Verify accepts a damaged bundle: a payload or chunk whose bytes do
//      not hash to its name, a missing event, a receipt line out of
//      sequence, or a manifest head that is not the tape's last event.
//
// Import and the restore drill (S4):
//  I1  The imported target does not answer like the source: its head row,
//      its append receipts of the range, its snapshot root or its chunk set
//      differ, or its tape reader cannot serve an imported event, payload
//      or pin receipt.
//  I2  The recovery plan differs: RecoveryBase or PlanRecovery answers a
//      different base, start or tail on the target.
//  I3  Import is not idempotent: a second import of the same bundle errors
//      or changes a row.
//  I4  Import merges a fork: a target whose head at the bundle's start, or
//      past it, has another digest accepts the bundle; or a target behind
//      the start (a gap) accepts it. A refusal must leave the target as it
//      was.
//  I5  An incremental bundle (the source advanced) does not extend a target
//      that holds the earlier range.
//  I6  The target cannot continue the tape: a new append on the target
//      after the import is refused by its CAS.

import (
	"bytes"
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/bundle"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

type drillComputer struct {
	t         *testing.T
	id        string
	store     *Store
	service   *Service
	artifacts *EventArtifactService
	cas       *ComputerEventCAS
	root      string
}

func newDrillComputer(t *testing.T, id string) *drillComputer {
	t.Helper()
	store, root := openTestPlatformStore(t)
	service := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	// Projection batches go to segments, other kinds to legacy files: the
	// bundle must read both (E2).
	service.SetTapeSegmentsMode("write")
	artifacts, err := NewEventArtifactService(service, platformTestKeyResolver{key: service.signingKey.Public})
	if err != nil {
		t.Fatal(err)
	}
	cas, err := NewComputerEventCAS(store, "corpusd", service.computerEventSigningKey(), artifacts)
	if err != nil {
		t.Fatal(err)
	}
	return &drillComputer{t: t, id: id, store: store, service: service, artifacts: artifacts, cas: cas, root: service.artifactsRoot}
}

// appendEvent pins the payload (if any) and the event through the production
// pin entry points and appends it through the head CAS.
func (c *drillComputer) appendEvent(kind computerevent.EventKind, mediaType string, payload []byte) string {
	t := c.t
	t.Helper()
	ctx := context.Background()
	head, err := c.cas.Head(ctx, c.id)
	if err != nil {
		t.Fatal(err)
	}
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: c.id,
		Sequence: 1, PreviousHead: computerevent.ZeroHead, EventKind: kind,
		IdempotencyKey: "drill-" + eventID, ActorProfile: "trusted-core", AuthorityRef: "authority:test",
		PayloadCommitment: platformTestDigest('a'), PrivacyClass: "public", ReducerVersion: computerevent.ReducerVersionV1,
		ExpectedDesiredEventHead: computerevent.ZeroHead, ExpectedEffectiveEventHead: computerevent.ZeroHead,
		ExpectedDesiredStateCommitment: computerevent.ZeroHead, ExpectedEffectiveStateCommitment: computerevent.ZeroHead,
	}
	input := computerevent.TransitionInput{}
	if head == nil {
		event.ResultingEffectiveCommitment = platformTestDigest('b')
		input.TargetStateCommitment = platformTestDigest('b')
	} else {
		event.Sequence = head.Sequence + 1
		event.PreviousHead = head.CanonicalEventHead
		event.ExpectedDesiredEventHead, event.ExpectedEffectiveEventHead = head.DesiredEventHead, head.EffectiveEventHead
		event.ExpectedPendingTransitionRef = head.PendingTransitionRef
		event.ExpectedDesiredStateCommitment, event.ExpectedEffectiveStateCommitment = head.DesiredStateCommitment, head.EffectiveStateCommitment
	}
	event.OccurredAt = time.Date(2026, 10, 30, 12, 0, int(event.Sequence), 0, time.UTC).Format(time.RFC3339Nano)
	if payload != nil {
		digest := computerevent.DigestBytes(payload)
		event.OutputArtifactRefs = []string{"artifact:sha256:" + digest}
		event.PayloadCommitment = digest
	}
	event.RequestCommitment = computerevent.ZeroHead
	pinIntent, err := computerevent.ComputePinIntentCommitment(event, input)
	if err != nil {
		t.Fatal(err)
	}
	var payloadPins []string
	if payload != nil {
		pin, err := c.artifacts.pinPayload(ctx, c.id, payload, mediaType, "public", pinIntent)
		if err != nil {
			t.Fatalf("pin payload: %v", err)
		}
		raw, _ := pin.Receipt.CanonicalBytes()
		payloadPins = append(payloadPins, computerevent.DigestBytes(raw))
	}
	event.RequestCommitment, err = computerevent.ComputeRequestCommitment(event, input, pinIntent, payloadPins)
	if err != nil {
		t.Fatal(err)
	}
	eventJSON, err := event.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	eventPin, err := c.artifacts.PinEvent(ctx, c.id, eventJSON, event.RequestCommitment)
	if err != nil {
		t.Fatalf("pin event: %v", err)
	}
	eventReceipt, _ := eventPin.Receipt.CanonicalBytes()
	next, err := computerevent.Reduce(head, event, input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.cas.CompareAndSwap(ctx, computerevent.CASRequest{
		Event: event, EventDigest: eventPin.ArtifactDigest, EventArtifactDigest: eventPin.ArtifactDigest,
		EventPinReceiptDigest: computerevent.DigestBytes(eventReceipt), PayloadPinReceiptDigests: payloadPins,
		PinIntentCommitment: pinIntent, Input: input, Next: next,
	}); err != nil {
		t.Fatalf("append %s at %d: %v", kind, event.Sequence, err)
	}
	return eventPin.ArtifactDigest
}

func (c *drillComputer) appendMixed(n int, tag string) {
	for i := 0; i < n; i++ {
		switch i % 3 {
		case 0:
			c.appendEvent(computerevent.EventMessageRecorded, "text/plain", []byte(tag+" message "+string(rune('a'+i))))
		case 1:
			c.appendEvent(computerevent.EventProjectionBatchRecorded, computerevent.ProjectionBatchMediaType, []byte(`{"batch":"`+tag+string(rune('a'+i))+`"}`))
		default:
			c.appendEvent(computerevent.EventToolInvoked, "", nil)
		}
	}
}

// snapshot pins n sealed chunks and records a snapshot root at the head.
func (c *drillComputer) snapshot(n int) bundle.SnapshotRoot {
	t := c.t
	t.Helper()
	ctx := context.Background()
	head, err := c.cas.Head(ctx, c.id)
	if err != nil || head == nil {
		t.Fatalf("head: %v", err)
	}
	var chunks []string
	for i := 0; i < n; i++ {
		data := make([]byte, 4096+i)
		if _, err := rand.Read(data); err != nil {
			t.Fatal(err)
		}
		digest := computerevent.DigestBytes(data)
		if err := c.service.PinFileChunk(ctx, c.id, digest, data); err != nil {
			t.Fatal(err)
		}
		chunks = append(chunks, digest)
	}
	rootBytes := make([]byte, 32)
	_, _ = rand.Read(rootBytes)
	root := computerevent.DigestBytes(rootBytes)
	if err := c.store.RecordSnapshotRoot(ctx, c.id, root, int64(head.Sequence), chunks); err != nil {
		t.Fatal(err)
	}
	return bundle.SnapshotRoot{ComputerID: c.id, Root: root, HeadSequence: int64(head.Sequence), Chunks: chunks}
}

func (c *drillComputer) head() computerevent.Head {
	c.t.Helper()
	head, err := c.cas.Head(context.Background(), c.id)
	if err != nil || head == nil {
		c.t.Fatalf("head = %v, %v", head, err)
	}
	return *head
}

func mustExport(t *testing.T, c *drillComputer, out string) bundle.Manifest {
	t.Helper()
	m, err := bundle.Export(context.Background(), c.store.db, c.root, bundle.ExportOptions{
		ComputerID: c.id, Out: out, FromSeq: -1, Producer: bundle.Producer{Host: "drill", Tool: "test"},
	})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	return m
}

func copyTree(t *testing.T, src, dst string) {
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

func firstFile(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("read %s: %v", dir, err)
	}
	return filepath.Join(dir, entries[0].Name())
}

func TestBundleExportVerify(t *testing.T) {
	src := newDrillComputer(t, "computer-bundle-src")
	src.appendEvent(computerevent.EventGenesisImported, "", nil)
	src.appendMixed(3, "pre")
	snap := src.snapshot(3)
	src.appendEvent(computerevent.EventSnapshotRecorded, "text/plain", []byte("sealed snapshot record"))
	src.appendMixed(5, "post")
	head := src.head()

	out := filepath.Join(t.TempDir(), "bundle")
	m := mustExport(t, src, out)
	// E1: the range is (snapshot head, head].
	if m.Tape.StartSeq != snap.HeadSequence || m.Tape.HeadSeq != int64(head.Sequence) || m.Tape.HeadDigest != head.CanonicalEventHead || !reflect.DeepEqual(m.Tape.Head, head) {
		t.Fatalf("tape range = %+v, want (%d, %d] head %s", m.Tape, snap.HeadSequence, head.Sequence, head.CanonicalEventHead)
	}
	if m.Counts.Receipts != m.Tape.HeadSeq-m.Tape.StartSeq || m.Counts.Events != m.Counts.Receipts {
		t.Fatalf("counts = %+v", m.Counts)
	}
	// E3: every payload (five of the six post-snapshot events carry one),
	// every pin receipt (one per event plus one per payload), every chunk.
	if m.Counts.Payloads != 5 || m.Counts.PinReceipts != m.Counts.Events+m.Counts.Payloads || m.Counts.Chunks != int64(len(snap.Chunks)) || m.SnapshotRoot == nil || m.SnapshotRoot.Root != snap.Root {
		t.Fatalf("artifact counts = %+v root %+v", m.Counts, m.SnapshotRoot)
	}
	// E2: some of them were read from a segment.
	if src.service.tapeReader().Stats().SegmentHits == 0 {
		t.Fatal("no tape object came from a segment; the drill does not cover the segment layout")
	}
	if _, err := os.Stat(filepath.Join(out, tapestore.SegDirName)); !os.IsNotExist(err) {
		t.Fatal("the bundle holds host segment files")
	}
	if _, err := bundle.Verify(out); err != nil {
		t.Fatalf("verify: %v", err)
	}

	// E4: a second export rewrites no content-addressed file, and after the
	// source advances, names the new head.
	chunk := firstFile(t, filepath.Join(out, bundle.ChunksDir))
	before, _ := os.Stat(chunk)
	time.Sleep(20 * time.Millisecond)
	src.appendMixed(2, "more")
	m2 := mustExport(t, src, out)
	after, _ := os.Stat(chunk)
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("re-export rewrote a chunk")
	}
	if m2.Tape.HeadSeq != m.Tape.HeadSeq+2 {
		t.Fatalf("re-export head = %d, want %d", m2.Tape.HeadSeq, m.Tape.HeadSeq+2)
	}
	if _, err := bundle.Verify(out); err != nil {
		t.Fatalf("verify after re-export: %v", err)
	}

	// E5: damaged copies are refused.
	damage := map[string]func(dir string){
		"payload bytes": func(dir string) {
			p := firstFile(t, filepath.Join(dir, bundle.PayloadsDir))
			_ = os.WriteFile(p, []byte("not the payload"), 0o640)
		},
		"chunk bytes": func(dir string) {
			p := firstFile(t, filepath.Join(dir, bundle.ChunksDir))
			raw, _ := os.ReadFile(p)
			raw[0] ^= 0xff
			_ = os.WriteFile(p, raw, 0o640)
		},
		"missing event": func(dir string) { _ = os.Remove(firstFile(t, filepath.Join(dir, bundle.EventsDir))) },
		"receipt order": func(dir string) {
			p := filepath.Join(dir, bundle.ReceiptsName)
			raw, _ := os.ReadFile(p)
			lines := bytes.SplitAfter(raw, []byte("\n"))
			lines[0], lines[1] = lines[1], lines[0]
			_ = os.WriteFile(p, bytes.Join(lines, nil), 0o640)
		},
		"truncated tape": func(dir string) {
			p := filepath.Join(dir, bundle.ReceiptsName)
			raw, _ := os.ReadFile(p)
			lines := bytes.SplitAfter(raw, []byte("\n"))
			_ = os.WriteFile(p, bytes.Join(lines[:len(lines)-2], nil), 0o640)
		},
	}
	names := make([]string, 0, len(damage))
	for name := range damage {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		dir := filepath.Join(t.TempDir(), "damaged")
		copyTree(t, out, dir)
		damage[name](dir)
		if _, err := bundle.Verify(dir); err == nil {
			t.Errorf("verify accepted a bundle with damaged %s", name)
		}
	}
	if _, err := bundle.Verify(t.TempDir()); err == nil {
		t.Error("verify accepted a directory without a manifest")
	}
}
