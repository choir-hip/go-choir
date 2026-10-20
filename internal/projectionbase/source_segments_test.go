package projectionbase

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// A1 (forward-fix doc section 9.4, FM20): DiskEventSource reads events and
// payloads from segments first and legacy second, fails closed on corruption,
// and refuses the directory-enumeration mode once a computer has segments
// (it would silently miss segment-resident events).

func moveEventsToSegment(t *testing.T, root, computerID string, segno uint64, keepLegacy bool, digests ...string) {
	t.Helper()
	w, err := tapestore.CreateSegment(root, computerID, segno)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range digests {
		path := filepath.Join(root, "sha256", "computer-event", d)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Append(tapestore.NSEvent, raw); err != nil {
			t.Fatal(err)
		}
		if !keepLegacy {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := w.Seal(); err != nil {
		t.Fatal(err)
	}
}

func TestDiskEventSourceReadsSegmentResidentEvents(t *testing.T) {
	root := t.TempDir()
	const id = "computer-disk-seg"
	digests := buildInstallChain(t, root, id)
	head := digests[len(digests)-1]
	baseline, err := NewDiskEventSource(root, id, head).EventsPage(context.Background(), id, 0, 0)
	if err != nil || len(baseline) != 3 {
		t.Fatalf("legacy baseline: %d, %v", len(baseline), err)
	}
	// Event 1 in a segment only, event 2 in both layouts, event 3 legacy.
	moveEventsToSegment(t, root, id, 1, false, digests[0])
	moveEventsToSegment(t, root, id, 2, true, digests[1])
	got, err := NewDiskEventSource(root, id, head).EventsPage(context.Background(), id, 0, 0)
	if err != nil {
		t.Fatalf("segment-resident replay: %v", err)
	}
	if !reflect.DeepEqual(got, baseline) {
		t.Fatal("replay from mixed layouts differs from the legacy replay")
	}

	// The directory enumeration cannot see segment events, so it refuses.
	if _, err := NewDiskEventSource(root, id).EventsPage(context.Background(), id, 0, 0); err == nil || !strings.Contains(err.Error(), "segment") {
		t.Fatalf("ReadDir mode with segments present = %v, want a refusal naming segments", err)
	}
	// A computer without segments still enumerates.
	other := t.TempDir()
	buildInstallChain(t, other, "computer-no-seg")
	if recs, err := NewDiskEventSource(other, "computer-no-seg").EventsPage(context.Background(), "computer-no-seg", 0, 0); err != nil || len(recs) != 3 {
		t.Fatalf("ReadDir mode without segments: %d, %v", len(recs), err)
	}

	// Rot in a segment fails the trace; legacy is not consulted.
	seg := filepath.Join(root, tapestore.SegDirName, id, tapestore.SegmentFileName(2))
	raw, err := os.ReadFile(seg)
	if err != nil {
		t.Fatal(err)
	}
	raw[32+46+1] ^= 0x33
	if err := os.WriteFile(seg, raw, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := NewDiskEventSource(root, id, head).EventsPage(context.Background(), id, 0, 0); !errors.Is(err, tapestore.ErrCorrupt) {
		t.Fatalf("corrupt segment trace = %v, want ErrCorrupt", err)
	}
}

func TestDiskEventSourceFetchPayloadFromSegments(t *testing.T) {
	root := t.TempDir()
	const id = "computer-disk-payload"
	payload := []byte(`{"projection":"batch"}`)
	digest := computerevent.DigestBytes(payload)
	w, err := tapestore.CreateSegment(root, id, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Append(tapestore.NSPayload, payload); err != nil {
		t.Fatal(err)
	}
	if err := w.Seal(); err != nil {
		t.Fatal(err)
	}
	legacyOnly := []byte(`{"projection":"legacy"}`)
	if err := os.MkdirAll(filepath.Join(root, "sha256", "computer-event-payload"), 0o755); err != nil {
		t.Fatal(err)
	}
	legacyDigest := computerevent.DigestBytes(legacyOnly)
	if err := os.WriteFile(filepath.Join(root, "sha256", "computer-event-payload", legacyDigest), legacyOnly, 0o600); err != nil {
		t.Fatal(err)
	}
	src := NewDiskEventSource(root, id)
	for d, want := range map[string][]byte{digest: payload, legacyDigest: legacyOnly} {
		got, err := src.FetchPayload(context.Background(), id, d)
		if err != nil || string(got) != string(want) {
			t.Fatalf("FetchPayload %s: %q, %v", d, got, err)
		}
	}
	if _, err := src.FetchPayload(context.Background(), id, strings.Repeat("9", 64)); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("absent payload: %v", err)
	}
	seg := filepath.Join(root, tapestore.SegDirName, id, tapestore.SegmentFileName(1))
	raw, _ := os.ReadFile(seg)
	raw[32+46] ^= 0x01
	if err := os.WriteFile(seg, raw, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := src.FetchPayload(context.Background(), id, digest); !errors.Is(err, tapestore.ErrCorrupt) {
		t.Fatalf("corrupt payload segment = %v, want ErrCorrupt", err)
	}
}
