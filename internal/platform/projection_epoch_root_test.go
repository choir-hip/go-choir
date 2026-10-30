package platform

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// An epoch root (owner, 2026-10-27: "maybe we lose some event log data and
// restart it from here"): a captured base advertised as the watermark becomes
// the computer's root. Ways it could fail:
//  1. the root is not recorded durably;
//  2. a later, ordinary advertisement drops the root;
//  3. the advertisement path cannot tell a root from a replayed base.
func TestEpochRootIsRecordedAndKept(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	ctx := context.Background()
	id := "computer-epoch"
	insertTestHead(t, s, id, 9000)
	preRoot, root := strings.Repeat("c", 64), strings.Repeat("d", 64)
	if err := s.RecordReplayWatermark(ctx, id, 100, preRoot); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordReplayWatermarkWithRoot(ctx, id, 6000, root, true); err != nil {
		t.Fatal(err)
	}
	read := func() (string, uint64) {
		var ref string
		var seq uint64
		if err := s.db.QueryRowContext(ctx, `SELECT base_ref,sequence FROM computer_projection_epoch_roots WHERE computer_id=?`, id).Scan(&ref, &seq); err != nil {
			t.Fatal(err)
		}
		return ref, seq
	}
	// 1
	if ref, seq := read(); ref != root || seq != 6000 {
		t.Fatalf("epoch root = %s %d, want %s 6000", ref, seq, root)
	}
	// 2
	if err := s.RecordReplayWatermark(ctx, id, 8000, strings.Repeat("f", 64)); err != nil {
		t.Fatal(err)
	}
	if ref, seq := read(); ref != root || seq != 6000 {
		t.Fatalf("epoch root after a derived base = %s %d, want it kept", ref, seq)
	}
}

// 3: the advertisement reads the root's mark from the host-written sidecar,
// not from the caller.
func TestAdvertisementBindingNamesAnEpochRoot(t *testing.T) {
	for raw, want := range map[string]bool{
		`{"computer_id":"c","sequence":1,"origin":"live_captured"}`: true,
		`{"computer_id":"c","sequence":1}`:                          false,
		`{"computer_id":"c","sequence":1,"origin":"other"}`:         false,
	} {
		var d projectionBaseDescriptorBinding
		if err := json.Unmarshal([]byte(raw), &d); err != nil {
			t.Fatal(err)
		}
		if got := d.epochRoot(); got != want {
			t.Fatalf("%s: epoch root = %v, want %v", raw, got, want)
		}
	}
}
