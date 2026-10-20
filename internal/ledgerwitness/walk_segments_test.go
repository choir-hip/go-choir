package ledgerwitness

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// A1 (forward-fix doc section 9.4, FM21): the walk reads events and payloads
// from segments first and legacy second, and fails closed on a corrupt
// segment instead of answering from another layout.

// moveToSegment copies the named legacy files into one sealed segment of the
// computer and, unless keepLegacy, removes the legacy copies.
func moveToSegment(t *testing.T, root, computerID string, segno uint64, keepLegacy bool, files map[string]tapestore.NS) {
	t.Helper()
	w, err := tapestore.CreateSegment(root, computerID, segno)
	if err != nil {
		t.Fatal(err)
	}
	for digest, ns := range files {
		path := filepath.Join(root, filepath.FromSlash(ns.LegacyRel(digest)))
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Append(ns, raw); err != nil {
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

func TestWalkReadsSegmentResidentEventsAndPayloads(t *testing.T) {
	c := newChain(t)
	c.append(t, testComputer, nil, false)
	castEvent := c.append(t, testComputer, []objectgraph.Object{record("cast-1")}, true)
	plainEvent := c.append(t, testComputer, nil, false)
	arrivalEvent := c.append(t, testComputer, []objectgraph.Object{record("arrival-1")}, true)
	cfg := Config{ArtifactsRoot: c.root, ComputerID: testComputer, Head: c.head, MaxEvents: 100}
	want := []string{"cast-1", "arrival-1", "never-written"}
	baseline, err := Walk(context.Background(), cfg, want)
	if err != nil {
		t.Fatal(err)
	}

	// The two projection events and their payloads move to a segment (and
	// an event with no records also to a second one, kept in both layouts); the rest
	// stay legacy.
	files := map[string]tapestore.NS{castEvent: tapestore.NSEvent, arrivalEvent: tapestore.NSEvent}
	entries, err := os.ReadDir(filepath.Join(c.root, "sha256", "computer-event-payload"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		files[e.Name()] = tapestore.NSPayload
	}
	moveToSegment(t, c.root, testComputer, 1, false, files)
	moveToSegment(t, c.root, testComputer, 2, true, map[string]tapestore.NS{plainEvent: tapestore.NSEvent})

	got, err := Walk(context.Background(), cfg, want)
	if err != nil {
		t.Fatalf("segment-resident walk: %v", err)
	}
	if !reflect.DeepEqual(got, baseline) {
		t.Fatalf("segment walk = %+v, legacy walk = %+v", got, baseline)
	}

	// Rot in the segment is an error, not a walk through legacy.
	seg := filepath.Join(c.root, tapestore.SegDirName, testComputer, tapestore.SegmentFileName(1))
	raw, err := os.ReadFile(seg)
	if err != nil {
		t.Fatal(err)
	}
	raw[32+46+2] ^= 0x7f
	if err := os.WriteFile(seg, raw, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := Walk(context.Background(), cfg, want); !errors.Is(err, tapestore.ErrCorrupt) {
		t.Fatalf("corrupt segment walk = %v, want ErrCorrupt", err)
	}
}

func TestWalkMissingPayloadStillStopsWithSegmentsPresent(t *testing.T) {
	c := newChain(t)
	ev := c.append(t, testComputer, []objectgraph.Object{record("cast-1")}, false) // payload written nowhere
	moveToSegment(t, c.root, testComputer, 1, false, map[string]tapestore.NS{ev: tapestore.NSEvent})
	if _, err := Walk(context.Background(), Config{ArtifactsRoot: c.root, ComputerID: testComputer, Head: c.head, MaxEvents: 10}, []string{"cast-1"}); err == nil {
		t.Fatal("a projection event without its payload must stop the walk")
	}
}
