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
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/bundle"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/recoveryplan"
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

// packEntries reads a format 2 bundle's objects.idx lines.
func packEntries(t *testing.T, dir string) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, bundle.PackIndexName))
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		var e map[string]any
		if err := json.Unmarshal(line, &e); err != nil {
			t.Fatalf("index line %q: %v", line, err)
		}
		out = append(out, e)
	}
	return out
}

// dropIndexLine rewrites objects.idx without the first line of kind.
func dropIndexLine(t *testing.T, dir, kind string) {
	t.Helper()
	p := filepath.Join(dir, bundle.PackIndexName)
	raw, _ := os.ReadFile(p)
	var kept [][]byte
	dropped := false
	for _, line := range bytes.SplitAfter(raw, []byte("\n")) {
		if !dropped && bytes.Contains(line, []byte(`"kind":"`+kind+`"`)) {
			dropped = true
			continue
		}
		kept = append(kept, line)
	}
	if !dropped {
		t.Fatalf("no %s line in the index", kind)
	}
	_ = os.WriteFile(p, bytes.Join(kept, nil), 0o640)
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
	// E6: the tape objects are packed, not one file each.
	if m.FormatVersion != 2 {
		t.Fatalf("format version = %d, want 2", m.FormatVersion)
	}
	for _, sub := range []string{bundle.EventsDir, bundle.PayloadsDir, bundle.PinsDir} {
		if _, err := os.Stat(filepath.Join(out, sub)); !os.IsNotExist(err) {
			t.Fatalf("the bundle holds a %s directory", sub)
		}
	}
	if got, want := int64(len(packEntries(t, out))), m.Counts.Events+m.Counts.Payloads+m.Counts.PinReceipts; got != want {
		t.Fatalf("index lines = %d, want %d", got, want)
	}

	// E7: an export stopped part way (bytes past the last index line, a torn
	// last line) resumes: the next export keeps the durable part and verifies.
	interrupted := filepath.Join(t.TempDir(), "interrupted")
	copyTree(t, out, interrupted)
	_ = os.Remove(filepath.Join(interrupted, bundle.ManifestName))
	if f, err := os.OpenFile(filepath.Join(interrupted, bundle.PackName), os.O_APPEND|os.O_WRONLY, 0); err == nil {
		_, _ = f.Write([]byte("bytes no index line names"))
		_ = f.Close()
	}
	if f, err := os.OpenFile(filepath.Join(interrupted, bundle.PackIndexName), os.O_APPEND|os.O_WRONLY, 0); err == nil {
		_, _ = f.Write([]byte(`{"kind":"events","digest":"torn`))
		_ = f.Close()
	}
	if mi := mustExport(t, src, interrupted); mi.Counts != m.Counts {
		t.Fatalf("resumed export counts = %+v, want %+v", mi.Counts, m.Counts)
	}
	if _, err := bundle.Verify(interrupted); err != nil {
		t.Fatalf("verify after a resumed export: %v", err)
	}
	if got := len(packEntries(t, interrupted)); got != len(packEntries(t, out)) {
		t.Fatalf("resumed export index lines = %d, want %d", got, len(packEntries(t, out)))
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
			var off int64 = -1
			for _, e := range packEntries(t, dir) {
				if e["kind"] == bundle.PayloadsDir {
					off = int64(e["offset"].(float64))
					break
				}
			}
			f, err := os.OpenFile(filepath.Join(dir, bundle.PackName), os.O_RDWR, 0)
			if err != nil || off < 0 {
				t.Fatalf("open pack: %v (payload offset %d)", err, off)
			}
			b := make([]byte, 1)
			_, _ = f.ReadAt(b, off)
			b[0] ^= 0xff
			_, _ = f.WriteAt(b, off)
			_ = f.Close()
		},
		"chunk bytes": func(dir string) {
			p := firstFile(t, filepath.Join(dir, bundle.ChunksDir))
			raw, _ := os.ReadFile(p)
			raw[0] ^= 0xff
			_ = os.WriteFile(p, raw, 0o640)
		},
		"missing event":       func(dir string) { dropIndexLine(t, dir, bundle.EventsDir) },
		"missing pin receipt": func(dir string) { dropIndexLine(t, dir, bundle.PinsDir) },
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

func drillReceipts(t *testing.T, c *drillComputer, after int64) []bundle.ReceiptRow {
	t.Helper()
	rows, err := c.store.db.Query(`SELECT computer_id, idempotency_key, request_commitment, sequence, event_kind, event_digest, event_pin_receipt_digest, event_head_receipt_id, event_head_receipt_json, created_at FROM computer_event_append_receipts WHERE computer_id=? AND sequence>? ORDER BY sequence`, c.id, after)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []bundle.ReceiptRow
	for rows.Next() {
		var r bundle.ReceiptRow
		if err := rows.Scan(&r.ComputerID, &r.IdempotencyKey, &r.RequestCommitment, &r.Sequence, &r.EventKind, &r.EventDigest, &r.EventPinReceiptDigest, &r.EventHeadReceiptID, &r.EventHeadReceiptJSON, &r.CreatedAt); err != nil {
			t.Fatal(err)
		}
		r.CreatedAt = r.CreatedAt.UTC()
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// drillChunkSet is the computer's file CAS chunk directory, each chunk
// checked against its name.
func drillChunkSet(t *testing.T, c *drillComputer) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	dir := filepath.Join(c.root, "sha256", "file-cas-chunks", c.id)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return out
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil || computerevent.DigestBytes(raw) != e.Name() {
			t.Fatalf("chunk %s does not hash to its name (%v)", e.Name(), err)
		}
		out[e.Name()] = true
	}
	return out
}

func drillPlan(t *testing.T, c *drillComputer) (recoveryplan.RecoveryPlan, string) {
	t.Helper()
	seq, root, err := c.store.RecoveryBase(context.Background(), c.id)
	if err != nil {
		t.Fatalf("recovery base: %v", err)
	}
	plan, err := recoveryplan.PlanRecovery(true, 0, true, uint64(seq), c.head().Sequence)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	return plan, root
}

func liveRoots(t *testing.T, c *drillComputer) []SnapshotRootRecord {
	t.Helper()
	roots, err := c.store.LiveSnapshotRoots(context.Background(), c.id)
	if err != nil {
		t.Fatal(err)
	}
	for i := range roots {
		roots[i].CreatedAt = roots[i].CreatedAt.UTC()
	}
	return roots
}

// assertSameComputer is I1 and I2: the target answers like the source.
func assertSameComputer(t *testing.T, src, dst *drillComputer, after int64) {
	t.Helper()
	if s, d := src.head(), dst.head(); !reflect.DeepEqual(s, d) {
		t.Fatalf("heads differ:\nsource %+v\ntarget %+v", s, d)
	}
	srcRows, dstRows := drillReceipts(t, src, after), drillReceipts(t, dst, after)
	if len(srcRows) == 0 || !reflect.DeepEqual(srcRows, dstRows) {
		t.Fatalf("append receipts after %d differ: source %d rows, target %d rows", after, len(srcRows), len(dstRows))
	}
	if s, d := liveRoots(t, src), liveRoots(t, dst); !reflect.DeepEqual(s, d) {
		t.Fatalf("snapshot roots differ:\nsource %+v\ntarget %+v", s, d)
	}
	if s, d := drillChunkSet(t, src), drillChunkSet(t, dst); !reflect.DeepEqual(s, d) || len(s) == 0 {
		t.Fatalf("chunk sets differ: source %d target %d", len(s), len(d))
	}
	reader := dst.service.tapeReader()
	for _, row := range dstRows {
		raw, _, err := reader.Read(dst.id, tapestore.NSEvent, row.EventDigest)
		if err != nil {
			t.Fatalf("target cannot read event %d: %v", row.Sequence, err)
		}
		var receipt computerevent.Receipt
		if err := receipt.UnmarshalJSON([]byte(row.EventHeadReceiptJSON)); err != nil {
			t.Fatal(err)
		}
		for _, pin := range receipt.KindFields["pin_receipt_digests"].([]any) {
			if _, _, err := reader.Read(dst.id, tapestore.NSReceipt, pin.(string)); err != nil {
				t.Fatalf("target cannot read pin receipt of event %d: %v", row.Sequence, err)
			}
		}
		event, err := computerevent.DecodeHistoricEvent(raw)
		if err != nil {
			t.Fatal(err)
		}
		for _, ref := range event.OutputArtifactRefs {
			digest, _ := eventArtifactDigestFromRef(ref)
			want, err := src.artifacts.FetchPayload(src.id, digest)
			if err != nil {
				t.Fatal(err)
			}
			got, err := dst.artifacts.FetchPayload(dst.id, digest)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("target payload %s of event %d: %v", digest, row.Sequence, err)
			}
		}
	}
	srcPlan, srcRoot := drillPlan(t, src)
	dstPlan, dstRoot := drillPlan(t, dst)
	if !reflect.DeepEqual(srcPlan, dstPlan) || srcRoot != dstRoot || dstPlan.Action != recoveryplan.RecoveryInstall {
		t.Fatalf("recovery plans differ:\nsource %+v %s\ntarget %+v %s", srcPlan, srcRoot, dstPlan, dstRoot)
	}
}

func mustRefuse(t *testing.T, what, dir string, c *drillComputer) {
	t.Helper()
	before := c.head()
	_, err := bundle.Import(context.Background(), dir, c.store.db, c.root)
	if !errors.Is(err, bundle.ErrRefused) {
		t.Fatalf("%s: import = %v, want a refusal", what, err)
	}
	t.Logf("%s: %v", what, err)
	if after := c.head(); !reflect.DeepEqual(before, after) {
		t.Fatalf("%s: a refused import moved the head", what)
	}
	if len(liveRoots(t, c)) != 0 || len(drillChunkSet(t, c)) != 0 {
		t.Fatalf("%s: a refused import wrote the snapshot root or its chunks", what)
	}
}

// TestBundleRestoreDrill is the restore drill: a computer's tape and snapshot
// in one scratch platform store and PostgreSQL, exported, verified and
// imported into a second, which then answers like the first.
func TestBundleRestoreDrill(t *testing.T) {
	ctx := context.Background()
	const id = "computer-bundle-drill"
	src := newDrillComputer(t, id)
	src.appendEvent(computerevent.EventGenesisImported, "", nil)
	src.appendMixed(3, "pre")
	snap := src.snapshot(3)
	src.appendEvent(computerevent.EventSnapshotRecorded, "text/plain", []byte("sealed snapshot record"))
	src.appendMixed(5, "post")
	out := filepath.Join(t.TempDir(), "bundle")
	m := mustExport(t, src, out)
	if _, err := bundle.Verify(out); err != nil {
		t.Fatalf("verify: %v", err)
	}

	dst := newDrillComputer(t, id)
	res, err := bundle.Import(ctx, out, dst.store.db, dst.root)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if res.HeadBefore != -1 || res.HeadAfter != m.Tape.HeadSeq || res.ReceiptsInserted != m.Counts.Receipts || !res.SnapshotRootInserted || res.FilesWritten == 0 {
		t.Fatalf("import result = %+v", res)
	}
	assertSameComputer(t, src, dst, snap.HeadSequence)

	// I3: idempotent.
	again, err := bundle.Import(ctx, out, dst.store.db, dst.root)
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if again.ReceiptsInserted != 0 || again.FilesWritten != 0 || again.SnapshotRootInserted || again.HeadAfter != m.Tape.HeadSeq {
		t.Fatalf("second import changed the target: %+v", again)
	}
	assertSameComputer(t, src, dst, snap.HeadSequence)

	// I5: the source advances; an incremental export extends the target.
	src.appendMixed(2, "later")
	m2 := mustExport(t, src, out)
	res2, err := bundle.Import(ctx, out, dst.store.db, dst.root)
	if err != nil {
		t.Fatalf("incremental import: %v", err)
	}
	if res2.HeadBefore != m.Tape.HeadSeq || res2.HeadAfter != m2.Tape.HeadSeq || res2.ReceiptsInserted != 2 {
		t.Fatalf("incremental import result = %+v", res2)
	}
	assertSameComputer(t, src, dst, snap.HeadSequence)

	// I4: a fork at the start, a fork inside the range, and a gap.
	fork := newDrillComputer(t, id)
	fork.appendEvent(computerevent.EventGenesisImported, "", nil)
	fork.appendMixed(3, "fork")
	if fork.head().Sequence != uint64(snap.HeadSequence) {
		t.Fatalf("fork head %d, want the snapshot head %d", fork.head().Sequence, snap.HeadSequence)
	}
	mustRefuse(t, "fork at the start", out, fork)
	fork.appendMixed(2, "fork more")
	mustRefuse(t, "fork inside the range", out, fork)
	gap := newDrillComputer(t, id)
	gap.appendEvent(computerevent.EventGenesisImported, "", nil)
	mustRefuse(t, "gap", out, gap)

	// I6: the target continues the tape through its own CAS.
	dst.appendEvent(computerevent.EventToolInvoked, "", nil)
	if got := dst.head().Sequence; got != uint64(m2.Tape.HeadSeq)+1 {
		t.Fatalf("target head after its own append = %d", got)
	}
	// A target past the bundle's head that holds the bundle's head imports
	// as a no-op.
	past, err := bundle.Import(ctx, out, dst.store.db, dst.root)
	if err != nil || past.ReceiptsInserted != 0 || past.HeadAfter != m2.Tape.HeadSeq+1 {
		t.Fatalf("import into a target past the bundle = %+v, %v", past, err)
	}
}
