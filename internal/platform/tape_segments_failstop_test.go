package platform

// Fail-stop at the pin path (S10), written before the fix. The segment writer
// poisons itself on its first failed fsync or write and returns an error from
// then on. The pin that hit the failure, and every later pin until the
// process restarts, must still be stored durably the legacy way: the caller
// falls back to the legacy file path, byte for byte what the flag-off path
// writes. Collision and corruption errors stay errors (P5).
//
//  PF1 A failed segment fsync: the pin succeeds, its four files are the exact
//      legacy files, and the chain validates and fetches.
//  PF2 A poisoned writer: later pins (same computer and another one) succeed
//      through the legacy path and touch nothing under tape-seg.
//  PF3 A restarted service (a new writer) recovers the poisoned writer's
//      segment, writes segments again, and everything stays readable.
//  PF4 Concurrent pins of one computer while the first one's fsync fails: all
//      succeed, every payload is on the tape, and the failed one is legacy.

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

func segTreeBytes(t *testing.T, root string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	base := filepath.Join(root, tapestore.SegDirName)
	_ = filepath.WalkDir(base, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(base, p)
		if d.IsDir() {
			out["dir:"+rel] = nil
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

func requireSameSegTree(t *testing.T, what string, before, after map[string][]byte) {
	t.Helper()
	if len(before) != len(after) {
		t.Fatalf("%s changed tape-seg: %v -> %v", what, treeKeys(before), treeKeys(after))
	}
	for k, v := range before {
		if w, ok := after[k]; !ok || string(v) != string(w) {
			t.Fatalf("%s changed tape-seg file %s", what, k)
		}
	}
}

func failSegmentFsyncs(f *writeFixture) {
	f.service.tapeAppender().SetSyncHookForTest(func(*os.File) error { return syscall.EIO })
}

// PF1
func TestTapeSegmentsFailedAppendFallsBackToLegacy(t *testing.T) {
	f := newWriteFixture(t, "write", true)
	const id = "computer-failstop"
	failSegmentFsyncs(f)
	chain := f.pinChain(t, id, computerevent.EventProjectionBatchRecorded, computerevent.ProjectionBatchMediaType, projectionPayload("fsync fails"))
	requireLegacyExactly(t, f.root, chain.legacyFiles())
	if err := f.artifacts.ValidateEventPins(context.Background(), chain.request); err != nil {
		t.Fatalf("validate: %v", err)
	}
	for _, it := range []segItem{chain.items()[0], chain.items()[1]} {
		got, err := f.artifacts.FetchPayload(id, it.digest)
		if err != nil || string(got) != string(chain.bytesOf(it)) {
			t.Fatalf("FetchPayload %s: %v", it.ns, err)
		}
	}
}

// PF2
func TestTapeSegmentsPoisonedWriterKeepsPinsOnTheLegacyPath(t *testing.T) {
	f := newWriteFixture(t, "write", true)
	failSegmentFsyncs(f)
	first := f.pinChain(t, "computer-poisoned", computerevent.EventProjectionBatchRecorded, computerevent.ProjectionBatchMediaType, projectionPayload("first"))
	f.service.tapeAppender().SetSyncHookForTest(nil) // the disk heals; the writer stays poisoned
	before := segTreeBytes(t, f.root)
	second := f.pinChain(t, "computer-poisoned", computerevent.EventProjectionBatchRecorded, computerevent.ProjectionBatchMediaType, projectionPayload("second"))
	other := f.pinChain(t, "computer-other", computerevent.EventProjectionBatchRecorded, computerevent.ProjectionBatchMediaType, projectionPayload("other computer"))
	requireSameSegTree(t, "pins through a poisoned writer", before, segTreeBytes(t, f.root))
	want := map[string][]byte{}
	for _, c := range []pinnedChain{first, second, other} {
		for k, v := range c.legacyFiles() {
			want[k] = v
		}
	}
	requireLegacyExactly(t, f.root, want)
	for _, c := range []pinnedChain{second, other} {
		if err := f.artifacts.ValidateEventPins(context.Background(), c.request); err != nil {
			t.Fatalf("validate: %v", err)
		}
	}
}

// PF3
func TestTapeSegmentsRestartedWriterRecoversAndWritesSegmentsAgain(t *testing.T) {
	f := newWriteFixture(t, "write", true)
	const id = "computer-restart"
	healthy := f.pinChain(t, id, computerevent.EventProjectionBatchRecorded, computerevent.ProjectionBatchMediaType, projectionPayload("before the failure"))
	failSegmentFsyncs(f)
	failed := f.pinChain(t, id, computerevent.EventProjectionBatchRecorded, computerevent.ProjectionBatchMediaType, projectionPayload("fsync fails"))

	f.service.tapeReaderMu.Lock()
	f.service.tapeAppenderVal = nil // the process restarts: a new writer over the same root
	f.service.tapeReaderMu.Unlock()
	after := f.pinChain(t, id, computerevent.EventProjectionBatchRecorded, computerevent.ProjectionBatchMediaType, projectionPayload("after the restart"))

	if got := segmentRecords(t, f.root, id); got[segItem{tapestore.NSPayload, computerevent.DigestBytes(after.payload)}] == 0 {
		t.Fatalf("the restarted writer did not write to a segment: %v", got)
	}
	for _, c := range []pinnedChain{healthy, failed, after} {
		if err := f.artifacts.ValidateEventPins(context.Background(), c.request); err != nil {
			t.Fatalf("validate: %v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(f.root, tapestore.SegDirName, id, tapestore.IndexFileName(1))); err != nil {
		t.Fatalf("the restarted writer did not seal the poisoned writer's segment: %v", err)
	}
}

// PF4
func TestTapeSegmentsConcurrentPinsWhileTheFirstFsyncFails(t *testing.T) {
	f := newWriteFixture(t, "write", true)
	const id = "computer-failed-pin"
	payload := projectionPayload("shared by two concurrent pins")
	digest := computerevent.DigestBytes(payload)

	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	f.service.tapeAppender().SetSyncHookForTest(func(*os.File) error {
		once.Do(func() { close(entered) })
		<-release
		return syscall.EIO
	})
	var aErr, bErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, aErr = f.artifacts.pinPayload(context.Background(), id, payload, computerevent.ProjectionBatchMediaType, "public", platformTestDigest('1'))
	}()
	<-entered
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, bErr = f.artifacts.pinPayload(context.Background(), id, payload, computerevent.ProjectionBatchMediaType, "public", platformTestDigest('2'))
	}()
	time.Sleep(250 * time.Millisecond)
	close(release)
	wg.Wait()
	if aErr != nil || bErr != nil {
		t.Fatalf("pins = %v, %v; a failed segment append must fall back to legacy, not fail the pin", aErr, bErr)
	}
	got, err := f.artifacts.FetchPayload(id, digest)
	if err != nil || string(got) != string(payload) {
		t.Fatalf("an acknowledged pin's payload is not on the tape: %v", err)
	}
}
