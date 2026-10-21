package platform

// Round 5 at the platform (choir-tape-segments-a2-repack-2026-10-21.md,
// round_5_design and its panel): a segment error is answered from a
// digest-verified legacy copy, so
//
//  RF1 FetchPayload keeps probing the next namespace after a segment error
//      and returns the copy it finds there; with none it returns the
//      segment error, not "not found";
//  RF2 a pin whose reuse check hits a corrupt segment is stored the legacy
//      way (an append error), not refused, and stays readable.

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// breakIndex flips the trailer crc of the computer's first index, so every
// segment lookup for that computer fails.
func breakIndex(t *testing.T, artifactsRoot, computerID string) {
	t.Helper()
	p := filepath.Join(artifactsRoot, tapestore.SegDirName, computerID, tapestore.IndexFileName(1))
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	n := len(raw)
	binary.BigEndian.PutUint32(raw[n-4:], binary.BigEndian.Uint32(raw[n-4:])^0xffffffff)
	if err := os.WriteFile(p, raw, 0o640); err != nil {
		t.Fatal(err)
	}
}

// RF1
func TestSegmentErrorFetchPayloadProbesTheNextNamespace(t *testing.T) {
	store, root := openTestPlatformStore(t)
	service := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	artifacts, err := NewEventArtifactService(service, platformTestKeyResolver{key: service.signingKey.Public})
	if err != nil {
		t.Fatal(err)
	}
	const id = "computer-rf1"
	put := func(ns string, data []byte) string {
		d := computerevent.DigestBytes(data)
		if err := service.writeBlob(filepath.Join("sha256", ns, d), data); err != nil {
			t.Fatal(err)
		}
		return d
	}
	segOnly := []byte("only in the broken segment")
	dSeg := put(tapeNSPayload, segOnly)
	segmentize(t, service.artifactsRoot, id, 1, true, false, segItem{tapestore.NSPayload, dSeg})
	ev := []byte(`{"an":"event only in legacy"}`)
	dEv := put(tapeNSEvent, ev)
	breakIndex(t, service.artifactsRoot, id)

	if got, err := artifacts.FetchPayload(id, dEv); err != nil || string(got) != string(ev) {
		t.Fatalf("event behind a payload-namespace segment error = %q, %v, want the legacy event", got, err)
	}
	if _, err := artifacts.FetchPayload(id, dSeg); !errors.Is(err, tapestore.ErrCorrupt) {
		t.Fatalf("segment-only payload = %v, want the segment error, not not-found", err)
	}
}

// RF2
func TestSegmentErrorAtPinTimeStoresThePinTheLegacyWay(t *testing.T) {
	f := newWriteFixture(t, "write", true)
	const id = "computer-rf2"
	old := []byte("an older payload in a sealed segment")
	d := computerevent.DigestBytes(old)
	if err := f.service.writeBlob(filepath.Join("sha256", tapeNSPayload, d), old); err != nil {
		t.Fatal(err)
	}
	segmentize(t, f.root, id, 1, true, false, segItem{tapestore.NSPayload, d})
	breakIndex(t, f.root, id)

	chain := f.pinChain(t, id, computerevent.EventProjectionBatchRecorded, computerevent.ProjectionBatchMediaType, projectionPayload("pinned past a corrupt index"))
	if err := f.artifacts.ValidateEventPins(context.Background(), chain.request); err != nil {
		t.Fatalf("validate: %v", err)
	}
	for _, it := range []segItem{chain.items()[0], chain.items()[1]} {
		got, err := f.artifacts.FetchPayload(id, it.digest)
		if err != nil || string(got) != string(chain.bytesOf(it)) {
			t.Fatalf("FetchPayload %s: %v", it.ns, err)
		}
	}
	if dg := f.service.TapeDiagnostics(); dg.SegmentFallbacks == 0 || dg.Appender.Poisoned {
		t.Fatalf("diagnostics: %+v, want counted legacy fallbacks and an unpoisoned writer", dg)
	}
}
