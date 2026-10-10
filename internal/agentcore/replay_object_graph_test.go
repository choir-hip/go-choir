package agentcore

import (
	"testing"

	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// The replay-completeness report hashed og_objects as one value, so an
// ineligible replay could not say which object was written outside the chain
// (problems/selfdev-apply-checkpoint-starved-by-resumed-work-2026-10-10.md).
func TestCompareReplayObjectGraphNamesDifferingObjects(t *testing.T) {
	row := func(id, kind, hash, meta string) choirstore.ObjectGraphFingerprint {
		return choirstore.ObjectGraphFingerprint{CanonicalID: id, ObjectKind: kind, ContentHash: hash, MetadataDigest: meta}
	}
	live := []choirstore.ObjectGraphFingerprint{
		row("obj:run:a", "run", "h1", "m1"),
		row("obj:run:b", "run", "h2", "m2"),
		row("obj:work:c", "work_item", "h3", "m3"),
	}
	replay := []choirstore.ObjectGraphFingerprint{
		row("obj:run:a", "run", "h1", "m1"),
		row("obj:run:b", "run", "h2x", "m2"),
		row("obj:event:d", "event", "h4", "m4"),
	}
	got := compareReplayObjectGraph(live, replay)
	if got == nil {
		t.Fatal("differing object graphs produced no comparison")
	}
	if got.LiveOnlyCount != 1 || got.ReplayOnlyCount != 1 || got.ChangedCount != 1 {
		t.Fatalf("counts = live_only %d replay_only %d changed %d, want 1/1/1", got.LiveOnlyCount, got.ReplayOnlyCount, got.ChangedCount)
	}
	byID := map[string]ReplayObjectGraphDifference{}
	for _, s := range got.Samples {
		byID[s.CanonicalID] = s
	}
	if s := byID["obj:work:c"]; s.Kind != "live_only" || s.ObjectKind != "work_item" {
		t.Fatalf("live-only sample = %+v", s)
	}
	if s := byID["obj:event:d"]; s.Kind != "replay_only" {
		t.Fatalf("replay-only sample = %+v", s)
	}
	if s := byID["obj:run:b"]; s.Kind != "changed" || len(s.Fields) != 1 || s.Fields[0] != "content_hash" {
		t.Fatalf("changed sample = %+v", s)
	}
	if got.ByObjectKind["run"] != 1 || got.ByObjectKind["work_item"] != 1 || got.ByObjectKind["event"] != 1 {
		t.Fatalf("by kind = %v", got.ByObjectKind)
	}
	if compareReplayObjectGraph(live, live) != nil {
		t.Fatal("equal object graphs must produce no comparison (keeps eligible reports unchanged)")
	}
}
