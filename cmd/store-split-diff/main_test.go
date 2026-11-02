package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/store/splitfixture"
)

func runDiff(t *testing.T, oldPath, newPath string) (int, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run([]string{"--old", oldPath, "--new", newPath, "--as-of", "2026-10-30T18:00:00Z", "--days", "5"}, &out, &errb)
	return code, out.String() + errb.String()
}

func TestStoreSplitDiffEqualPairExitsZero(t *testing.T) {
	oldPath, newPath, err := splitfixture.Build(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	code, out := runDiff(t, oldPath, newPath)
	if code != 0 {
		t.Fatalf("exit %d on an equal pair:\n%s", code, out)
	}
	if strings.Contains(out, `"rows_compared": 0`) || !strings.Contains(out, `"rows_compared"`) {
		t.Errorf("retire equivalence compared no rows:\n%s", out)
	}
	for _, name := range []string{"layouts", "rows.og_header_body_pairing", "witness.tables", "derived.retire_equivalence", "keys.og_key_index", "heads.computer_event_projection_heads"} {
		if !strings.Contains(out, `"`+name+`"`) {
			t.Errorf("report lacks check %s", name)
		}
	}
}

func TestStoreSplitDiffNamesChangedBody(t *testing.T) {
	oldPath, newPath, err := splitfixture.Build(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	db, err := splitfixture.OpenRW(newPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE og_bodies SET body = ? WHERE obj_id = (SELECT obj_id FROM og_objects WHERE canonical_id = 'doc-1')`, []byte("tampered")); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	code, out := runDiff(t, oldPath, newPath)
	if code != 1 {
		t.Fatalf("exit %d, want 1:\n%s", code, out)
	}
	if !strings.Contains(out, "doc-1") || !strings.Contains(out, "row differs") {
		t.Fatalf("report does not name doc-1:\n%s", out)
	}
}

func TestStoreSplitDiffCatchesMissingBodyAndStaleDerived(t *testing.T) {
	oldPath, newPath, err := splitfixture.Build(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	db, err := splitfixture.OpenRW(newPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE choir_events SET event_phase = 'wrong' WHERE canonical_id = 'e1'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM choir_event_bodies WHERE ev_id = (SELECT ev_id FROM choir_events WHERE canonical_id = 'e3')`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	code, out := runDiff(t, oldPath, newPath)
	if code != 1 {
		t.Fatalf("exit %d, want 1:\n%s", code, out)
	}
	for _, want := range []string{"e1", "missing_bodies"} {
		if !strings.Contains(out, want) {
			t.Errorf("report lacks %q:\n%s", want, out)
		}
	}
}
