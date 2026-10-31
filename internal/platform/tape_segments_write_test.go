package platform

// Stage A2 failure modes for the segment write path, written before the code.
// Section 4.2, 4.4 and 9.6 of
// docs/reports/tape-volume-forward-fix-and-surgery-2026-10-19.md. The flag is
// PLATFORM_TAPE_SEGMENTS=off|write, default off. Mutation class red: pin
// receipts, event and payload artifacts, replay.
//
//  P1  Flag off, unset or unknown (the unknown value logs once): a projection
//      batch event, its payload and both pin receipts are written as exactly
//      the four legacy files, with exactly the bytes they have today, and no
//      tape-seg directory exists. This is the regression guard for production
//      with the flag at its default.
//  P2  Flag write: a projection_batch_recorded event, its
//      ProjectionBatchMediaType payload and both receipts are records of one
//      segment of the computer, and none of the four exists in the legacy
//      layout.
//  P3  Flag write, any other kind: an event of another kind, a payload of
//      another media type, and their receipts stay legacy; a projection event
//      with an ordinary payload splits exactly as the kinds say. A computer
//      id that cannot name a segment directory stays legacy too.
//  P4  Flag write: the reader resolves every written digest from the segment
//      with the written bytes, FetchPayload and the head CAS pin check
//      (ValidateEventPins) accept the segment-resident chain.
//  P5  Collisions across both stores: the same bytes already in legacy is a
//      no-op (not copied into a segment, mtime refreshed as today); different
//      bytes in legacy is an error and appends nothing; the same bytes
//      already in a segment is a no-op; a segment record that does not hold
//      its digest's bytes is an error and appends nothing.
//  P6  Flag flipped back to off: digests written while it was write still
//      resolve (reader, FetchPayload, ValidateEventPins), new pins go legacy.
//  P7  Concurrent pins of one computer through the service are serialized
//      (-race): every payload resolves, no record is torn.
//  P8  Counters: the diagnostics snapshot and the health endpoint report the
//      mode, reader hits, appender fsyncs and files, and files created by
//      writeBlob; a segment pin creates no legacy file.
//  P9  Dead weight (residual, not a test): a pin whose CAS never lands stays
//      in its segment until the segment is dropped. Documented in
//      event_artifacts.go; no bound is enforced here.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

type writeFixture struct {
	store     *Store
	service   *Service
	artifacts *EventArtifactService
	root      string // the artifacts root
}

func newWriteFixture(t *testing.T, mode string, setMode bool) *writeFixture {
	t.Helper()
	store, root := openTestPlatformStore(t)
	service := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	if setMode {
		service.SetTapeSegmentsMode(mode)
	}
	artifacts, err := NewEventArtifactService(service, platformTestKeyResolver{key: service.signingKey.Public})
	if err != nil {
		t.Fatal(err)
	}
	return &writeFixture{store: store, service: service, artifacts: artifacts, root: service.artifactsRoot}
}

type pinnedChain struct {
	request       computerevent.CASRequest
	eventJSON     []byte
	payload       []byte
	eventReceipt  []byte
	payloadReceip []byte
}

// pinChain pins one payload (of mediaType) and then one event of eventKind,
// through the production pin entry points.
func (f *writeFixture) pinChain(t *testing.T, computerID string, kind computerevent.EventKind, mediaType string, payload []byte) pinnedChain {
	t.Helper()
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: computerID, Sequence: 1,
		PreviousHead: computerevent.ZeroHead, EventKind: kind,
		OccurredAt: time.Date(2026, 7, 18, 22, 0, 0, 0, time.UTC).Format(time.RFC3339Nano), IdempotencyKey: "k-" + eventID,
		ActorProfile: "trusted-core", AuthorityRef: "authority:test",
		InputArtifactRefs: []string{"artifact:sha256:" + computerevent.DigestBytes(payload)},
		PayloadCommitment: platformTestDigest('a'), PrivacyClass: "public", ReducerVersion: computerevent.ReducerVersionV1,
		ExpectedDesiredEventHead: computerevent.ZeroHead, ExpectedEffectiveEventHead: computerevent.ZeroHead,
		ExpectedDesiredStateCommitment: computerevent.ZeroHead, ExpectedEffectiveStateCommitment: computerevent.ZeroHead,
		ResultingEffectiveCommitment: platformTestDigest('b'),
	}
	input := computerevent.TransitionInput{TargetStateCommitment: platformTestDigest('b')}
	pinIntent, err := computerevent.ComputePinIntentCommitment(event, input)
	if err != nil {
		t.Fatal(err)
	}
	payloadPin, err := f.artifacts.pinPayload(context.Background(), computerID, payload, mediaType, "public", pinIntent)
	if err != nil {
		t.Fatalf("pin payload: %v", err)
	}
	payloadReceipt, _ := payloadPin.Receipt.CanonicalBytes()
	event.RequestCommitment, err = computerevent.ComputeRequestCommitment(event, input, pinIntent, []string{computerevent.DigestBytes(payloadReceipt)})
	if err != nil {
		t.Fatal(err)
	}
	eventJSON, _ := event.CanonicalBytes()
	eventPin, err := f.artifacts.PinEvent(context.Background(), computerID, eventJSON, event.RequestCommitment)
	if err != nil {
		t.Fatalf("pin event: %v", err)
	}
	eventReceipt, _ := eventPin.Receipt.CanonicalBytes()
	return pinnedChain{
		request: computerevent.CASRequest{
			Event: event, EventDigest: eventPin.ArtifactDigest, EventArtifactDigest: eventPin.ArtifactDigest,
			EventPinReceiptDigest: computerevent.DigestBytes(eventReceipt), PayloadPinReceiptDigests: []string{computerevent.DigestBytes(payloadReceipt)},
			PinIntentCommitment: pinIntent, Input: input,
		},
		eventJSON: eventJSON, payload: payload, eventReceipt: eventReceipt, payloadReceip: payloadReceipt,
	}
}

// legacyTree lists every file under the artifacts root outside tape-seg, as
// slash-separated relative path to bytes.
func legacyTree(t *testing.T, root string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, tapestore.SegDirName+"/") {
			return nil
		}
		raw, rerr := os.ReadFile(p)
		if rerr != nil {
			t.Fatal(rerr)
		}
		out[rel] = raw
		return nil
	})
	return out
}

func treeKeys(m map[string][]byte) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (c pinnedChain) legacyFiles() map[string][]byte {
	return map[string][]byte{
		"sha256/computer-event/" + c.request.EventDigest:                         c.eventJSON,
		"sha256/computer-event-payload/" + computerevent.DigestBytes(c.payload):  c.payload,
		"sha256/pin-receipts/" + c.request.EventPinReceiptDigest + ".json":       c.eventReceipt,
		"sha256/pin-receipts/" + c.request.PayloadPinReceiptDigests[0] + ".json": c.payloadReceip,
	}
}

func (c pinnedChain) items() []segItem {
	return []segItem{
		{tapestore.NSEvent, c.request.EventDigest}, {tapestore.NSPayload, computerevent.DigestBytes(c.payload)},
		{tapestore.NSReceipt, c.request.EventPinReceiptDigest}, {tapestore.NSReceipt, c.request.PayloadPinReceiptDigests[0]},
	}
}

func (c pinnedChain) bytesOf(it segItem) []byte {
	switch {
	case it.ns == tapestore.NSEvent:
		return c.eventJSON
	case it.ns == tapestore.NSPayload:
		return c.payload
	case it.digest == c.request.EventPinReceiptDigest:
		return c.eventReceipt
	}
	return c.payloadReceip
}

func requireLegacyExactly(t *testing.T, root string, want map[string][]byte) {
	t.Helper()
	got := legacyTree(t, root)
	if len(got) != len(want) {
		t.Fatalf("legacy files = %v, want %v", treeKeys(got), treeKeys(want))
	}
	for k, v := range want {
		if !bytes.Equal(got[k], v) {
			t.Fatalf("legacy file %s differs from the bytes written today (%d vs %d bytes)", k, len(got[k]), len(v))
		}
	}
}

func requireNoSegments(t *testing.T, root string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, tapestore.SegDirName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("tape-seg exists (stat err %v)", err)
	}
}

func segmentRecords(t *testing.T, root, computerID string) map[segItem]int {
	t.Helper()
	out := map[segItem]int{}
	if err := tapestore.NewReader(root).Scan(computerID, func(r tapestore.Record) error {
		out[segItem{r.NS, r.Digest}]++
		return nil
	}); err != nil {
		t.Fatalf("scan: %v", err)
	}
	return out
}

func projectionPayload(label string) []byte {
	return []byte(`{"schema":"projection-batch","label":"` + label + `"}`)
}

// P1
func TestTapeSegmentsOffWritesTheLegacyFilesExactlyAsToday(t *testing.T) {
	var logs bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(prev)
	ran := 0
	for name, c := range map[string]struct {
		mode string
		set  bool
	}{"unset": {"", false}, "empty": {"", true}, "off": {"off", true}, "unknown value": {"writ", true}, "garbage": {"on;write", true}} {
		t.Run(name, func(t *testing.T) {
			f := newWriteFixture(t, c.mode, c.set)
			ran++
			chain := f.pinChain(t, "computer-flag-off", computerevent.EventProjectionBatchRecorded, computerevent.ProjectionBatchMediaType, projectionPayload("off"))
			requireLegacyExactly(t, f.root, chain.legacyFiles())
			requireNoSegments(t, f.root)
			if err := f.artifacts.ValidateEventPins(context.Background(), chain.request); err != nil {
				t.Fatalf("validate: %v", err)
			}
			if got := f.service.TapeDiagnostics().SegmentsMode; got != "off" {
				t.Fatalf("mode = %q", got)
			}
		})
	}
	if ran == 0 {
		t.Skip("no case ran (no PostgreSQL), so the log count proves nothing")
	}
	if n := strings.Count(logs.String(), "PLATFORM_TAPE_SEGMENTS"); n != 2 {
		t.Fatalf("the unknown values logged %d lines naming the flag, want 2:\n%s", n, logs.String())
	}
}

// P2, P4
func TestTapeSegmentsWriteRoutesProjectionBatchesToASegment(t *testing.T) {
	f := newWriteFixture(t, "write", true)
	const id = "computer-seg-write"
	chain := f.pinChain(t, id, computerevent.EventProjectionBatchRecorded, computerevent.ProjectionBatchMediaType, projectionPayload("segment"))
	if got := legacyTree(t, f.root); len(got) != 0 {
		t.Fatalf("legacy files written for a segmented pin: %v", treeKeys(got))
	}
	records := segmentRecords(t, f.root, id)
	if len(records) != 4 {
		t.Fatalf("segment records = %v, want the event, the payload and two receipts", records)
	}
	reader := f.service.tapeReader()
	for _, it := range chain.items() {
		got, src, err := reader.Read(id, it.ns, it.digest)
		if err != nil || src != tapestore.SourceSegment || !bytes.Equal(got, chain.bytesOf(it)) {
			t.Fatalf("%s %s: src %v err %v same-bytes %v", it.ns, it.digest, src, err, bytes.Equal(got, chain.bytesOf(it)))
		}
	}
	for _, it := range []segItem{chain.items()[0], chain.items()[1]} {
		got, err := f.artifacts.FetchPayload(id, it.digest)
		if err != nil || !bytes.Equal(got, chain.bytesOf(it)) {
			t.Fatalf("FetchPayload %s: %v", it.ns, err)
		}
	}
	if err := f.artifacts.ValidateEventPins(context.Background(), chain.request); err != nil {
		t.Fatalf("head CAS pin check refused a segment-resident chain: %v", err)
	}
	// The segment is the open one, not a sealed one: the writer has not hit a limit.
	if _, err := os.Stat(filepath.Join(f.root, tapestore.SegDirName, id, tapestore.SegmentFileName(1))); err != nil {
		t.Fatal(err)
	}
}

// P3
func TestTapeSegmentsWriteLeavesOtherKindsInLegacy(t *testing.T) {
	f := newWriteFixture(t, "write", true)
	other := f.pinChain(t, "computer-other-kinds", computerevent.EventGenesisImported, "application/octet-stream", []byte("an ordinary payload"))
	requireLegacyExactly(t, f.root, other.legacyFiles())
	requireNoSegments(t, f.root)

	// A projection event with a payload of another media type: the event and
	// its receipt are segmented, the payload and its receipt are not.
	const id = "computer-split"
	split := f.pinChain(t, id, computerevent.EventProjectionBatchRecorded, "application/octet-stream", []byte("not a batch"))
	records := segmentRecords(t, f.root, id)
	if len(records) != 2 || records[segItem{tapestore.NSEvent, split.request.EventDigest}] != 1 || records[segItem{tapestore.NSReceipt, split.request.EventPinReceiptDigest}] != 1 {
		t.Fatalf("segment records = %v, want only the event and its receipt", records)
	}
	legacy := legacyTree(t, f.root)
	for k := range split.legacyFiles() {
		_, inLegacy := legacy[k]
		isEventSide := strings.Contains(k, "computer-event/") || strings.Contains(k, split.request.EventPinReceiptDigest)
		if inLegacy == isEventSide {
			t.Fatalf("%s: inLegacy=%v but the event side is %v", k, inLegacy, isEventSide)
		}
	}
	if err := f.artifacts.ValidateEventPins(context.Background(), split.request); err != nil {
		t.Fatalf("a split chain failed the pin check: %v", err)
	}

	// A payload of the batch media type under a non-projection event: the payload and its receipt are segmented.
	const id2 = "computer-batch-payload-only"
	odd := f.pinChain(t, id2, computerevent.EventGenesisImported, computerevent.ProjectionBatchMediaType, projectionPayload("odd"))
	records = segmentRecords(t, f.root, id2)
	if len(records) != 2 || records[segItem{tapestore.NSPayload, computerevent.DigestBytes(odd.payload)}] != 1 {
		t.Fatalf("segment records = %v, want the payload and its receipt", records)
	}

	// An id that cannot name a segment directory is legacy, never an error.
	before := len(legacyTree(t, f.root))
	f.pinChain(t, "computer/with/slashes", computerevent.EventProjectionBatchRecorded, computerevent.ProjectionBatchMediaType, projectionPayload("bad id"))
	if after := len(legacyTree(t, f.root)); after != before+4 {
		t.Fatalf("an id that cannot name a segment wrote %d legacy files, want 4", after-before)
	}
	if _, err := os.Stat(filepath.Join(f.root, tapestore.SegDirName, "computer")); err == nil {
		t.Fatal("a slash id escaped into a segment directory")
	}
}

// P5
func TestTapeSegmentsCollisionsAcrossBothStores(t *testing.T) {
	const id = "computer-collide"
	pinBatch := func(f *writeFixture, payload []byte) error {
		_, err := f.artifacts.pinPayload(context.Background(), id, payload, computerevent.ProjectionBatchMediaType, "public", platformTestDigest('c'))
		return err
	}
	t.Run("same bytes in legacy is a no-op that refreshes mtime", func(t *testing.T) {
		f := newWriteFixture(t, "write", true)
		payload := projectionPayload("legacy resident")
		digest := computerevent.DigestBytes(payload)
		ref := filepath.Join("sha256", tapeNSPayload, digest)
		if err := f.service.writeBlob(ref, payload); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(f.root, ref)
		old := time.Now().Add(-48 * time.Hour)
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
		if err := pinBatch(f, payload); err != nil {
			t.Fatal(err)
		}
		records := segmentRecords(t, f.root, id)
		if records[segItem{tapestore.NSPayload, digest}] != 0 {
			t.Fatal("a payload already in legacy was copied into a segment")
		}
		if info, _ := os.Stat(path); time.Since(info.ModTime()) > time.Hour {
			t.Fatal("the reused legacy file's mtime was not refreshed (the tape sweep grace rule)")
		}
	})
	t.Run("different bytes in legacy is an error and appends nothing", func(t *testing.T) {
		f := newWriteFixture(t, "write", true)
		payload := projectionPayload("legacy impostor")
		path := filepath.Join(f.root, "sha256", tapeNSPayload, computerevent.DigestBytes(payload))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("different bytes under this digest"), 0o640); err != nil {
			t.Fatal(err)
		}
		if err := pinBatch(f, payload); err == nil || !strings.Contains(err.Error(), "immutable digest collision") {
			t.Fatalf("pin = %v, want an immutable digest collision", err)
		}
		// The reuse checks run inside the writer's lock, which may have created
		// the computer's empty first segment; no record may have been appended.
		if n := len(segmentRecords(t, f.root, id)); n != 0 {
			t.Fatalf("a refused pin appended %d records", n)
		}
	})
	t.Run("same bytes in a segment appends nothing and leaves a legacy copy (r1)", func(t *testing.T) {
		f := newWriteFixture(t, "write", true)
		payload := projectionPayload("segment resident")
		if err := pinBatch(f, payload); err != nil {
			t.Fatal(err)
		}
		if len(legacyTree(t, f.root)) != 0 {
			t.Fatal("legacy files appeared on the first pin")
		}
		if err := pinBatch(f, payload); err != nil {
			t.Fatal(err)
		}
		if n := segmentRecords(t, f.root, id)[segItem{tapestore.NSPayload, computerevent.DigestBytes(payload)}]; n != 1 {
			t.Fatalf("payload records = %d, want exactly 1", n)
		}
		if got := legacyTree(t, f.root)["sha256/computer-event-payload/"+computerevent.DigestBytes(payload)]; !bytes.Equal(got, payload) {
			t.Fatal("a pin answered from a segment record left no legacy copy of it")
		}
	})
	t.Run("a segment record that does not hold its digest sends the pin the legacy way and appends nothing", func(t *testing.T) {
		// Round 5: the unreadable segment is a store failure, not a verdict
		// on the pin. The pin is stored as a legacy file, the segment is not
		// touched, and the digest-verified read serves the legacy copy.
		f := newWriteFixture(t, "write", true)
		payload := projectionPayload("rotten resident")
		if err := pinBatch(f, payload); err != nil {
			t.Fatal(err)
		}
		corruptSegments(t, f.root, id) // first record = the payload; a valid record (the receipt) follows
		segFile := filepath.Join(f.root, tapestore.SegDirName, id, tapestore.SegmentFileName(1))
		before, _ := os.ReadFile(segFile)
		if err := pinBatch(f, payload); err != nil {
			t.Fatalf("pin = %v, want the legacy fallback", err)
		}
		after, _ := os.ReadFile(segFile)
		if !bytes.Equal(before, after) {
			t.Fatal("the pin changed the corrupt segment")
		}
		if _, ok := legacyTree(t, f.root)["sha256/computer-event-payload/"+computerevent.DigestBytes(payload)]; !ok {
			t.Fatal("the pin did not store the payload as a legacy file")
		}
		if got, err := f.artifacts.FetchPayload(id, computerevent.DigestBytes(payload)); err != nil || !bytes.Equal(got, payload) {
			t.Fatalf("FetchPayload after the fallback = %v", err)
		}
	})
}

// P6
func TestTapeSegmentsFlagFlippedBackToOff(t *testing.T) {
	f := newWriteFixture(t, "write", true)
	const id = "computer-flip"
	inSeg := f.pinChain(t, id, computerevent.EventProjectionBatchRecorded, computerevent.ProjectionBatchMediaType, projectionPayload("before the flip"))
	f.service.SetTapeSegmentsMode("off")
	afterOff := f.pinChain(t, id, computerevent.EventProjectionBatchRecorded, computerevent.ProjectionBatchMediaType, projectionPayload("after the flip"))
	requireLegacyExactly(t, f.root, afterOff.legacyFiles())
	for _, it := range inSeg.items() {
		got, src, err := f.service.tapeReader().Read(id, it.ns, it.digest)
		if err != nil || src != tapestore.SourceSegment || !bytes.Equal(got, inSeg.bytesOf(it)) {
			t.Fatalf("%s from before the flip: src %v err %v", it.ns, src, err)
		}
	}
	if got, err := f.artifacts.FetchPayload(id, computerevent.DigestBytes(inSeg.payload)); err != nil || !bytes.Equal(got, inSeg.payload) {
		t.Fatalf("FetchPayload: %v", err)
	}
	for _, c := range []pinnedChain{inSeg, afterOff} {
		if err := f.artifacts.ValidateEventPins(context.Background(), c.request); err != nil {
			t.Fatalf("pin check after the flip: %v", err)
		}
	}
	// And back on: the same segment continues.
	f.service.SetTapeSegmentsMode("write")
	f.pinChain(t, id, computerevent.EventProjectionBatchRecorded, computerevent.ProjectionBatchMediaType, projectionPayload("flipped back on"))
	if n := len(segmentRecords(t, f.root, id)); n != 8 {
		t.Fatalf("segment records = %d, want 8", n)
	}
}

// P7
func TestTapeSegmentsConcurrentPinsOfOneComputer(t *testing.T) {
	f := newWriteFixture(t, "write", true)
	const id = "computer-race"
	const workers, per = 6, 8
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < per; i++ {
				payload := projectionPayload(fmt.Sprintf("w%d-i%d", w, i))
				if _, err := f.artifacts.pinPayload(context.Background(), id, payload, computerevent.ProjectionBatchMediaType, "public", platformTestDigest(byte('a'+w))); err != nil {
					t.Errorf("pin: %v", err)
					return
				}
			}
		}(w)
	}
	wg.Wait()
	records := segmentRecords(t, f.root, id)
	payloads := 0
	for it := range records {
		if it.ns == tapestore.NSPayload {
			payloads++
		}
	}
	if payloads != workers*per {
		t.Fatalf("payload records = %d, want %d (all %d records: %d)", payloads, workers*per, len(records), len(records))
	}
	for w := 0; w < workers; w++ {
		for i := 0; i < per; i++ {
			payload := projectionPayload(fmt.Sprintf("w%d-i%d", w, i))
			if got, err := f.artifacts.FetchPayload(id, computerevent.DigestBytes(payload)); err != nil || !bytes.Equal(got, payload) {
				t.Fatalf("w%d i%d: %v", w, i, err)
			}
		}
	}
}

// P8
func TestTapeDiagnosticsCountersAndHealth(t *testing.T) {
	f := newWriteFixture(t, "write", true)
	const id = "computer-counters"
	seg := f.pinChain(t, id, computerevent.EventProjectionBatchRecorded, computerevent.ProjectionBatchMediaType, projectionPayload("counted"))
	d := f.service.TapeDiagnostics()
	if d.SegmentsMode != "write" || d.SegmentPins != 2 || d.LegacyPins != 0 || d.LegacyFilesCreated != 0 || d.BlobFilesCreated != 0 {
		t.Fatalf("after segment pins: %+v", d)
	}
	if d.Appender.Syncs != 2 || d.Appender.SegmentsCreated != 1 || d.Appender.Records != 4 || d.SegmentFilesCreated != 1 {
		t.Fatalf("appender counters: %+v", d)
	}
	f.service.tapeReader().Read(id, tapestore.NSEvent, seg.request.EventDigest)
	f.service.SetTapeSegmentsMode("off")
	f.pinChain(t, id, computerevent.EventGenesisImported, "application/octet-stream", []byte("legacy now"))
	d = f.service.TapeDiagnostics()
	if d.SegmentsMode != "off" || d.LegacyPins != 2 || d.LegacyFilesCreated != 4 || d.BlobFilesCreated != 4 || d.Reader.SegmentHits < 1 {
		t.Fatalf("after legacy pins: %+v", d)
	}
	if d.SegmentPins != 2 || d.SegmentFilesCreated != 1 {
		t.Fatalf("segment counters moved on legacy pins: %+v", d)
	}

	rec := httptest.NewRecorder()
	NewHandler(f.service).HandleHealth(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("health = %d", rec.Code)
	}
	var body struct {
		Tape *struct {
			SegmentsMode        string `json:"segments_mode"`
			SegmentPins         uint64 `json:"segment_pins"`
			LegacyPins          uint64 `json:"legacy_pins"`
			BlobFilesCreated    uint64 `json:"blob_files_created"`
			LegacyFilesCreated  uint64 `json:"legacy_files_created"`
			SegmentFilesCreated uint64 `json:"segment_files_created"`
			Reader              struct {
				SegmentHits uint64 `json:"segment_hits"`
			} `json:"reader"`
			Appender struct {
				Syncs uint64 `json:"syncs"`
			} `json:"appender"`
		} `json:"tape"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Tape == nil {
		t.Fatalf("health body has no tape block: %v\n%s", err, rec.Body.String())
	}
	if body.Tape.SegmentsMode != "off" || body.Tape.SegmentPins != 2 || body.Tape.LegacyPins != 2 || body.Tape.BlobFilesCreated != 4 ||
		body.Tape.LegacyFilesCreated != 4 || body.Tape.SegmentFilesCreated != 1 || body.Tape.Reader.SegmentHits < 1 || body.Tape.Appender.Syncs != 2 {
		t.Fatalf("health tape block = %+v", body.Tape)
	}
}
